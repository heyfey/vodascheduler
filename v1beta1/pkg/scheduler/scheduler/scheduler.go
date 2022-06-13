package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/algorithm"
	"github.com/heyfey/vodascheduler/pkg/common/mongo"
	"github.com/heyfey/vodascheduler/pkg/common/rabbitmq"
	"github.com/heyfey/vodascheduler/pkg/common/trainingjob"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"github.com/heyfey/vodascheduler/pkg/placement"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	mpijobclientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	client "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/typed/kubeflow/v1"
	kubeflowinformers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubeClient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

const (
	reschedChannelSize          = 100
	rateLimitTimeMetricsSeconds = 5
	reschedRateLimitSeconds     = 30
	databaseNameJobInfo         = "job_info"
	databaseNameJobMetadata     = "job_metadata"
	collectionNameJobMetadata   = "v1beta1"
	databaseNameRunningJobs     = "runnings"
	gpuNameLabel                = "vodascheduler/accelerator"
)

type Scheduler struct {
	SchedulerID    string
	TotalGpus      int
	mpiClient      *client.KubeflowV1Client
	mpiJobInformer cache.SharedIndexInformer
	kClient        *kubeClient.Clientset
	nodeInformer   cache.SharedIndexInformer

	// Waiting & running jobs
	ReadyJobsMap map[string]*trainingjob.TrainingJob
	// Completed & failed jobs
	DoneJobsMap map[string]*trainingjob.TrainingJob
	// Number of allocated GPUs of each training job
	JobNumGPU types.JobScheduleResult
	// SchedulerLock is used to protect ReadyJobsMap, DoneJobsMap and JobNumGPU
	SchedulerLock sync.RWMutex

	// ScheduleAlgorithm is an interface implemented by things that know how to schedule training jobs
	Algorithm algorithm.SchedulerAlgorithm

	// SchedulerMetrics contains run-time metrics of the scheduler
	Metrics SchedulerMetrics

	// channels used for main logic of the scheduler, should only be cousumed by scheduler.Run()
	reschedCh       chan time.Time
	stopSchedulerCh chan time.Time

	lastResched         time.Time
	reschedBlockedUntil time.Time

	session *mgo.Session
	msgs    <-chan rabbitmq.Msg
	Router  *mux.Router

	PlacementManager *placement.PlacementManager
	ticker           time.Ticker
}

func discoverGPUs(kClient *kubeClient.Clientset, gpuType string) (int, error) {
	labelSelector := gpuNameLabel + "=" + gpuType
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	nodes, err := kClient.CoreV1().Nodes().List(context.TODO(), listOptions)
	if err != nil {
		return 0, err
	}

	totalGPUs := 0
	for _, node := range nodes.Items {
		totalGPUs += countGPUs(node)
	}
	return totalGPUs, err
}

func countGPUs(node corev1.Node) int {
	gpus := node.Status.Capacity["nvidia.com/gpu"]
	return int(gpus.Value())
}

// NewScheduler creates a new scheduler
func NewScheduler(id string, kConfig *rest.Config, resume bool) (*Scheduler, error) {
	mpiClient, err := client.NewForConfig(kConfig)
	if err != nil {
		return nil, err
	}

	kClient, err := kubeClient.NewForConfig(kConfig)
	if err != nil {
		return nil, err
	}

	totalGpus, err := discoverGPUs(kClient, id)
	if err != nil {
		return nil, err
	}

	pm, err := placement.NewPlacementManager(id, kConfig, resume)
	if err != nil {
		return nil, err
	}

	mpiJobClientSet, err := mpijobclientset.NewForConfig(kConfig)
	if err != nil {
		return nil, err
	}

	labelString := gpuNameLabel + "=" + id
	// setup mpijob informer
	mpijobLabelOptions := kubeflowinformers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = labelString
	})
	kubeflowInformerFactory := kubeflowinformers.NewSharedInformerFactoryWithOptions(
		mpiJobClientSet,
		0,
		kubeflowinformers.WithNamespace(config.Namespace),
		mpijobLabelOptions)
	mpiJobInformer := kubeflowInformerFactory.Kubeflow().V1().MPIJobs().Informer()

	// setup node informer
	nodeLabelOptions := kubeinformers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = labelString
	})
	factory := kubeinformers.NewSharedInformerFactoryWithOptions(kClient, 0, nodeLabelOptions)
	nodeInformer := factory.Core().V1().Nodes().Informer()

	session := mongo.ConnectMongo()

	conn := rabbitmq.ConnectRabbitMQ()
	msgs, err := rabbitmq.ReceiveFromQueue(conn, id)
	if err != nil {
		return nil, err
	}

	s := &Scheduler{
		SchedulerID:    id,
		TotalGpus:      totalGpus,
		mpiClient:      mpiClient,
		mpiJobInformer: mpiJobInformer,
		kClient:        kClient,
		nodeInformer:   nodeInformer,

		ReadyJobsMap:  map[string]*trainingjob.TrainingJob{},
		DoneJobsMap:   map[string]*trainingjob.TrainingJob{},
		JobNumGPU:     map[string]int{},
		SchedulerLock: sync.RWMutex{},

		Algorithm: algorithm.NewElasticFIFO(id),

		reschedCh:           make(chan time.Time, reschedChannelSize),
		stopSchedulerCh:     make(chan time.Time),
		reschedBlockedUntil: time.Now(),
		lastResched:         time.Now(),

		session: session,
		msgs:    msgs,
		Router:  mux.NewRouter(),

		PlacementManager: pm,
		ticker:           *time.NewTicker(rateLimitTimeMetricsSeconds * time.Second),
	}

	s.initRoutes()
	s.initSchedulerMetrics()

	// setup informer callbacks
	s.mpiJobInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: s.updateMPIJob,
		},
	)

	s.nodeInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.addNode,
			UpdateFunc: s.updateNode,
			DeleteFunc: s.deleteNode,
		},
	)

	if resume {
		// The call to constructStatusOnRestart doesn't acquire lock, hence
		// should make sure it is called only when initiating the scheduler
		s.constructStatusOnRestart()
		s.TriggerResched()
	}

	go s.Run()

	return s, nil
}

func (s *Scheduler) initRoutes() {
	s.Router.HandleFunc(config.EntryPoint, s.getAllTrainingJobHandler()).Methods("GET")
	s.Router.Handle("/metrics", promhttp.Handler())
}

func (s *Scheduler) TriggerResched() {
	s.reschedCh <- time.Now()
}

func (s *Scheduler) Run() {
	klog.InfoS("Starting scheduler")
	defer klog.InfoS("Stopping scheduler")

	klog.InfoS("Discovered total number of GPUs", "totalGpus", s.TotalGpus)

	// defer close channels ..?

	stopTickerCh := make(chan bool)
	go s.updateTimeMetrics(stopTickerCh)

	go s.readMsgs()

	stopInformerCh := make(chan struct{})
	go s.mpiJobInformer.Run(stopInformerCh)
	go s.nodeInformer.Run(stopInformerCh)
	if !cache.WaitForCacheSync(
		stopInformerCh,
		s.mpiJobInformer.HasSynced,
		s.nodeInformer.HasSynced) {
		err := errors.New("Failed to WaitForCacheSync")
		klog.ErrorS(err, "Scheduler failed to WaitForCacheSync")
		klog.Flush()
		os.Exit(1)
	}

	for {
		select {
		case r := <-s.reschedCh:
			if r.After(s.lastResched) {
				klog.V(4).InfoS("Received rescheduling event, may be blocked because of rate limit",
					"receivedAtTimestamp", r, "lastReschedulingAtTimestamp", s.lastResched,
					"blockedUntilTimestamp", s.reschedBlockedUntil)

				for time.Now().Before(s.reschedBlockedUntil) {
					time.Sleep(2)
				}
				s.resched()
				s.lastResched = time.Now()
				s.reschedBlockedUntil = s.lastResched.Add(time.Second * reschedRateLimitSeconds)
			} else {
				// The rescheduling events with timestamp before s.lastResched are
				// considered sastified, simply ignore them.
				klog.V(5).InfoS("Ignored rescheduling event", "receivedAtTimestamp", r)
			}

		case _ = <-s.stopSchedulerCh:
			stopTickerCh <- true
			stopInformerCh <- struct{}{}
			return
		}
	}
}

func (s *Scheduler) resched() {
	klog.V(3).InfoS("Started rescheduling")
	defer klog.V(3).InfoS("Finished rescheduling")

	timer := prometheus.NewTimer(s.Metrics.reschedDuration)
	defer timer.ObserveDuration()

	s.SchedulerLock.Lock()
	oldJobNumGPU := s.JobNumGPU
	s.updateAllJobsInfoFromDB()

	timerAlgo := prometheus.NewTimer(s.Metrics.reschedAlgoDuration)
	s.JobNumGPU = s.Algorithm.Schedule(s.makeReadyJobslist(), s.TotalGpus)
	timerAlgo.ObserveDuration()

	// s.SchedulerLock.Unlock() // may want to unlock here to implement cancelling mechanism

	adjusted := s.applySchedulerResults(oldJobNumGPU)
	s.SchedulerLock.Unlock()

	if adjusted {
		s.SchedulerLock.RLock()
		s.recordRunningJobsInDB()
		s.PlacementManager.Place(s.JobNumGPU)
		s.SchedulerLock.RUnlock()
	} else {
		klog.V(3).InfoS("Skipped ajust placement because nothing changed")
	}

	s.Metrics.reschedCounter.Inc()
}

func (s *Scheduler) makeReadyJobslist() algorithm.ReadyJobs {
	list := make(algorithm.ReadyJobs, len(s.ReadyJobsMap))
	for _, job := range s.ReadyJobsMap {
		list = append(list, *job)
	}
	return list
}

// updateAllJobsInfoFromDB finds information of all training jobs in mongodb
// and update the training jobs' info with retrieved information
func (s *Scheduler) updateAllJobsInfoFromDB() {
	sess := s.session.Clone()
	defer sess.Close()

	klog.V(4).InfoS("Updating all jobs info")

	for _, job := range s.ReadyJobsMap {
		info := mongo.TrainingJobInfo{}
		err := sess.DB(databaseNameJobInfo).C(job.Category).Find(bson.M{"name": job.Name}).One(&info)
		if err != nil {
			klog.ErrorS(err, "Failed to update job info", "job", job.Name)
			continue
		}
		job.Info.EstimatedRemainningTimeSec = info.EstimatedRemainningTimeSec
		job.Info.Efficiency = info.Efficiency
		job.Info.Speedup = info.Speedup

		klog.V(5).InfoS("Updated job info", "job", job)
	}
}

// recordRunningJobsToDB records witch jobs are currently running in mongodb,
// these records would be fetched by the metrics collector
// If we saves all jobs' scheduler status in mongodb, we may don't need this
func (s *Scheduler) recordRunningJobsInDB() error {
	sess := s.session.Clone()
	defer sess.Close()

	// clear the whole collection
	_, err := sess.DB(databaseNameRunningJobs).C(s.SchedulerID).RemoveAll(nil)
	if err != nil {
		klog.ErrorS(err, "Failed to remove all records in mongo collection",
			"database", databaseNameJobInfo, "collection", s.SchedulerID)
		return err
	}
	// insert running jobs
	for _, job := range s.ReadyJobsMap {
		if job.Status == types.JobRunning {
			entry := mongo.JobRunning{Name: job.Name}
			err = sess.DB(databaseNameRunningJobs).C(s.SchedulerID).Insert(entry)
			if err != nil {
				klog.ErrorS(err, "Failed to insert record to mongo",
					"database", databaseNameJobInfo, "collection", s.SchedulerID,
					"entry", entry)
				return err
			}
		}
	}
	return nil
}

// applySchedulerResults performs required changes to achieve new JobScheduleResult
// and returns if there is any change has been made.
func (s *Scheduler) applySchedulerResults(oldResult map[string]int) bool {
	halts, scaleIns, scaleOuts, starts := s.compareResults(oldResult)
	changes := len(halts) + len(scaleIns) + len(scaleOuts) + len(starts)
	s.haltTrainingJobMany(halts...)
	s.scaleTrainingJobMany(scaleIns...)
	// (optinal) wait for ajustments complete
	s.startTrainingJobMany(starts...)
	s.scaleTrainingJobMany(scaleOuts...)
	// (optinal) wait for ajustments complete

	return changes != 0
}

// compareResults compares old and new JobScheduleResult to find required changes
func (s *Scheduler) compareResults(oldResult map[string]int) ([]string, []string, []string, []string) {
	halts := make([]string, 0)
	scaleIns := make([]string, 0)
	scaleOuts := make([]string, 0)
	starts := make([]string, 0)

	for job, n := range oldResult {
		if n > s.JobNumGPU[job] {
			if s.JobNumGPU[job] == 0 {
				status, err := s.getJobStatus(job)
				if err != nil {
					klog.ErrorS(err, "Job not exist", "job", job)
					continue
				}
				// don't delete a completed or failed job
				if status != types.JobCompleted && status != types.JobFailed {
					halts = append(halts, job)
				}
			} else {
				scaleIns = append(scaleIns, job)
			}
		} else if n < s.JobNumGPU[job] {
			if n == 0 {
				starts = append(starts, job)
			} else {
				scaleOuts = append(scaleOuts, job)
			}
		} else {
			// n == s.JobNumGPU[job], no change
		}
	}
	return halts, scaleIns, scaleOuts, starts
}

// startTrainingJobMany creates MPIJobs of training jobs
func (s *Scheduler) startTrainingJobMany(jobs ...string) {
	// Use term "start training job" in logging to distinguish between "create training job"
	klog.V(4).InfoS("Starting training jobs", "jobs", jobs)

	for _, job := range jobs {
		s.startTrainingJob(job)
	}
}

// startTrainingJob creates MPIJob of a training job, with the number of
// worker replicas equal to the number of GPU assigned.
// Should acquire lock before calling it.
func (s *Scheduler) startTrainingJob(job string) error {
	mpiJob := s.ReadyJobsMap[job].Spec
	s.setMPIJobWorkerReplicas(mpiJob)

	_, err := s.mpiClient.MPIJobs(config.Namespace).Create(context.TODO(), mpiJob, metav1.CreateOptions{})
	if err != nil {
		klog.ErrorS(err, "Failed to start training job, this should not happen", "job", klog.KObj(mpiJob))
		// TODO(heyfey): error handling
		// https://github.com/kubeflow/mpi-operator/blob/master/pkg/controllers/v1/mpi_job_controller.go#L875
	} else {
		s.ReadyJobsMap[job].Status = types.JobRunning
		klog.V(5).InfoS("Started training job", "job", klog.KObj(mpiJob))

		// reset time metrics
		s.ReadyJobsMap[job].Metrics.LastGpuDuration = 0
		s.ReadyJobsMap[job].Metrics.LastRunningDuration = 0

		// if the job is first started
		if s.ReadyJobsMap[job].Metrics.RunningDuration == 0 {
			s.ReadyJobsMap[job].Metrics.FirstStartTime = time.Now()
		}
	}
	return err
}

// setMPIJobWorkerReplicas sets the number of worker replicas of a MPIJob to the number of GPU assigned
func (s *Scheduler) setMPIJobWorkerReplicas(mpiJob *kubeflowv1.MPIJob) {
	workerSpec := mpiJob.Spec.MPIReplicaSpecs["Worker"]
	*workerSpec.Replicas = int32(s.JobNumGPU[mpiJob.GetName()])
}

// scaleTrainingJobMany scales MPIJob of training jobs
func (s *Scheduler) scaleTrainingJobMany(jobs ...string) {
	klog.V(4).InfoS("Scaling training jobs", "jobs", jobs)

	for _, job := range jobs {
		s.scaleTrainingJob(job)
	}
}

// scaleTrainingJob gets and updates worker replicas of a MPIJob, wrapped with retry.RetryOnConflict.
// Should acquire lock before calling it.
// Though jobs may completed or failed during the scaling, it's no harm to update a completed/failed job,
// and the results would be fixed by the following rescheduling
func (s *Scheduler) scaleTrainingJob(job string) error {
	// TODO: may want to check job status == types.JobRunning
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		mpiJob, err := s.mpiClient.MPIJobs(config.Namespace).Get(context.TODO(), job, metav1.GetOptions{})
		if err != nil {
			return err
		}

		s.setMPIJobWorkerReplicas(mpiJob)

		_, err = s.mpiClient.MPIJobs(config.Namespace).Update(context.TODO(), mpiJob, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		klog.ErrorS(err, "Failed to scale training job, this should not happen",
			"job", klog.KRef(config.Namespace, job)) // TODO(heyfey): error handling
	} else {
		klog.V(5).InfoS("Scaled training job", "job", klog.KRef(config.Namespace, job))
	}
	return err
}

// haltTrainingJobMany deletes MPIJobs of training jobs
func (s *Scheduler) haltTrainingJobMany(jobs ...string) {
	klog.V(4).InfoS("Stopping training jobs", "jobs", jobs)

	for _, job := range jobs {
		s.haltTrainingJob(job)
	}
}

// haltTrainingJob deletes MPIJob of a training job.
// Should acquire lock before calling it.
func (s *Scheduler) haltTrainingJob(job string) error {
	err := s.mpiClient.MPIJobs(config.Namespace).Delete(context.TODO(), job, metav1.DeleteOptions{})
	if err != nil {
		klog.ErrorS(err, "Failed to stop training job, this should not happen",
			"job", klog.KRef(config.Namespace, job))
		// TODO: error handling if not delete
	} else {
		klog.V(5).InfoS("Stopped training job", "job", klog.KRef(config.Namespace, job))

		s.ReadyJobsMap[job].Status = types.JobWaiting
		s.ReadyJobsMap[job].Metrics.LastWaitingDuration = 0
	}
	return err
}

// updateMPIJob handles completion and failure of MPIJobs
func (s *Scheduler) updateMPIJob(oldObj interface{}, newObj interface{}) {
	mpiJob, ok := newObj.(*kubeflowv1.MPIJob)
	if !ok {
		klog.ErrorS(errors.New("unexpected MPIJob type"),
			"Failed to update MPIJob", "mpijob", klog.KObj(mpiJob))
		return
	}
	klog.V(5).InfoS("MPIJob updated", "mpijob", klog.KObj(mpiJob))

	if isFinished(mpiJob.Status) {
		s.SchedulerLock.Lock()
		defer s.SchedulerLock.Unlock()

		s.handleMPIJobFinished(mpiJob)
	}
}

// handleMPIJobFinished handles job finished.
// Should acquire lock before calling it.
func (s *Scheduler) handleMPIJobFinished(mpiJob *kubeflowv1.MPIJob) {
	job := mpiJob.GetName()
	status, err := s.getJobStatus(job)
	if err != nil {
		klog.ErrorS(err, "Finished job not exist", "job", job)
		return
	}
	if isSucceeded(mpiJob.Status) {
		if status != types.JobCompleted { // is the first succeeded event
			s.handleJobCompleted(job)
		}
	} else if isFailed(mpiJob.Status) {
		if status != types.JobFailed { // is the first failed event
			s.handleJobFailed(job)
		}
	}
}

// handleJobCompleted makes essential updates and sends rescheduling signal.
// It should only be called by handleMPIJobFinished, and should acquire lock
// before calling it.
func (s *Scheduler) handleJobCompleted(jobName string) {
	job, ok := s.ReadyJobsMap[jobName]
	if !ok {
		panic(errors.New("Failed to find completed job"))
	}
	klog.InfoS("Training job completed", "job", klog.KRef(config.Namespace, job.Name),
		"waitedTotalSeconds", job.Metrics.WaitingDuration.Seconds(),
		"ranTotalSeconds", job.Metrics.RunningDuration.Seconds(),
		"gpuTotalSeconds", job.Metrics.GpuDuration.Seconds(),
		"elaspedTotalSeconds", job.Metrics.TotalDuration.Seconds())

	job.Status = types.JobCompleted

	s.handleJobDoneInternal(job)

	s.Metrics.jobsCompletedCounter.Inc()
	s.TriggerResched()
}

// handleJobFailed makes essential updates and sends rescheduling signal.
// Note that if a job uses OnFailure restart policy, the watcher won't receive
// a event with JobFailed phase even when the job fails, thus this function
// won't be called in this situation.
// (Do not use ExitCode since there are hanging issue when job fails)
// It should only be called by handleMPIJobFinished, and should acquire lock
// before calling it.
func (s *Scheduler) handleJobFailed(jobName string) {
	klog.InfoS("Training job failed", "job", jobName)

	job, ok := s.ReadyJobsMap[jobName]
	if !ok {
		panic(errors.New("Failed to find failed job"))
	}
	job.Status = types.JobFailed

	s.handleJobDoneInternal(job)

	s.Metrics.jobsFailedCounter.Inc()
	s.TriggerResched()
}

func (s *Scheduler) handleJobDoneInternal(job *trainingjob.TrainingJob) {
	sess := s.session.Clone()
	defer sess.Close()
	err := sess.DB(databaseNameJobMetadata).C(collectionNameJobMetadata).
		Update(bson.M{"job_name": job.Name, "gpu_type": s.SchedulerID}, job)
	if err != nil {
		klog.ErrorS(err, "Failed to update training job in mongodb",
			"job", job.Name, "status", job.Status)
		// TODO(heyfey): error handling
	}

	s.DoneJobsMap[job.Name] = job
	delete(s.ReadyJobsMap, job.Name)
	delete(s.JobNumGPU, job.Name)
}

func (s *Scheduler) addNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.ErrorS(errors.New("unexpected node type"), "Failed to add Node", "node", klog.KObj(node))
		return
	}

	s.SchedulerLock.Lock()
	defer s.SchedulerLock.Unlock()

	// Unlike deleteNode and updateNode, addNode has to be declarative
	var err error
	s.TotalGpus, err = discoverGPUs(s.kClient, s.SchedulerID)
	if err != nil {
		klog.ErrorS(err, "Failed to add Node", "node", klog.KObj(node)) // TODO(heyfey): error handling
		return
	}

	s.TriggerResched()
	klog.InfoS("Node added", "node", klog.KObj(node), "totalGpus", s.TotalGpus)
}

func (s *Scheduler) updateNode(oldObj interface{}, newObj interface{}) {
	oldNode, ok := oldObj.(*corev1.Node)
	if !ok {
		klog.ErrorS(errors.New("unexpected node type"), "Failed to update node", "node", klog.KObj(oldNode))
		return
	}
	newNode, ok := newObj.(*corev1.Node)
	if !ok {
		klog.ErrorS(errors.New("unexpected node type"), "Failed to update node", "node", klog.KObj(newNode))
		return
	}
	// Informer may deliver an Update event with UID changed if a delete is
	// immediately followed by a create.
	if oldNode.UID != newNode.UID {
		s.SchedulerLock.Lock()
		defer s.SchedulerLock.Unlock()

		s.TotalGpus += (countGPUs(*newNode) - countGPUs(*oldNode))
		s.TriggerResched()
		klog.InfoS("Node updated", "node", klog.KObj(newNode), "totalGpus", s.TotalGpus)
	}
}

func (s *Scheduler) deleteNode(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.ErrorS(errors.New("unexpected node type"), "Failed to delete Node", "node", klog.KObj(node))
		return
	}

	s.SchedulerLock.Lock()
	defer s.SchedulerLock.Unlock()

	s.TotalGpus -= countGPUs(*node)
	s.TriggerResched()
	klog.InfoS("Node deleted", "node", klog.KObj(node), "totalGpus", s.TotalGpus)
}

func (s *Scheduler) Stop() {
	s.stopSchedulerCh <- time.Now()
}

// updateTimeMetrics updates time metrics of all training jobs every
// rateLimitTimeMetricsSeconds seconds.
// Depends on the scheduling algorithm, it may also checks for priority changes
// and/or triggers rescheduling.
func (s *Scheduler) updateTimeMetrics(stopTickerCh chan bool) {
	for {
		select {
		case <-stopTickerCh:
			klog.InfoS("Stopped ticker")
			return
		case <-s.ticker.C:
			// Some algorithms change priority of training jobs according to time metrics,
			// and may trigger rescheduling if that happens.
			priorityChanged := false

			s.SchedulerLock.Lock()
			for _, job := range s.ReadyJobsMap {
				status := job.Status
				elasped := time.Since(job.Metrics.LastUpdateTime)
				numGpu := s.JobNumGPU[job.Name]
				if status == types.JobRunning {
					job.Metrics.RunningDuration += elasped
					job.Metrics.GpuDuration += elasped * time.Duration(numGpu)
					job.Metrics.TotalDuration += elasped
					job.Metrics.LastRunningDuration += elasped
					job.Metrics.LastGpuDuration += elasped * time.Duration(numGpu)
				} else if status == types.JobWaiting {
					job.Metrics.WaitingDuration += elasped
					job.Metrics.TotalDuration += elasped
					job.Metrics.LastWaitingDuration += elasped
				}
				job.Metrics.LastUpdateTime = time.Now()

				// Tiresias' rules of priority changes
				if (s.Algorithm.GetName() == "Tiresias" || s.Algorithm.GetName() == "ElasticTiresias") &&
					(status == types.JobRunning || status == types.JobWaiting) {
					// demote the job if last GPU time crosses threshold
					if job.Metrics.LastGpuDuration.Seconds() > algorithm.TiresiasThresholdsSec[job.Priority] {
						job.Priority = algorithm.TiresiasDemotePriority(job.Priority)
						priorityChanged = true
						klog.V(4).InfoS("Demoted priority to training job", "job", job.Name, "priority", job.Priority)

						// promote the job if last waiting time longer than STARVELIMIT
					} else if job.Metrics.LastWaitingDuration >= job.Metrics.LastRunningDuration*time.Duration(algorithm.TiresiasPromoteKnob) &&
						job.Priority > 0 {
						job.Priority = algorithm.TiresiasPromotePriority(job.Priority)
						priorityChanged = true
						klog.V(4).InfoS("Promoted priority to training job", "job", job.Name, "priority", job.Priority)
					}
				}
			}
			s.SchedulerLock.Unlock()

			// trigger rescheduling if priority changed
			if priorityChanged {
				klog.V(3).InfoS("Triggered rescheduling because of priority changed")
				s.TriggerResched()
			}
		}
	}
}

// getJobStatus takes job name and return current status of the job, or an error
// if the job doesn't exist. Should acquire lock/rlock before calling it.
func (s *Scheduler) getJobStatus(job string) (types.JobStatusType, error) {
	j, ok := s.ReadyJobsMap[job]
	if ok {
		return j.Status, nil
	}
	j, ok = s.DoneJobsMap[job]
	if ok {
		return j.Status, nil
	}
	return "", errors.New("Not exist")
}

func (s *Scheduler) readMsgs() {
	for msg := range s.msgs {
		jobName := msg.JobName
		verb := msg.Verb
		if verb == rabbitmq.VerbCreate {
			s.CreateTrainingJob(jobName)
		} else if verb == rabbitmq.VerbDelete {
			s.DeleteTrainingJob(jobName)
		} else if verb == rabbitmq.VerbConfigure {
			// TODO(heyfey)
		} else {
			klog.Info("Unknown message verb from mq", "message", msg)
		}
	}
}

func (s *Scheduler) CreateTrainingJob(jobName string) {
	s.SchedulerLock.Lock()
	defer s.SchedulerLock.Unlock()

	// Check if job already exist
	_, err := s.getJobStatus(jobName)
	if err == nil {
		klog.ErrorS(errors.New("Already exist"),
			"Attempted to create training job that already exist, try using another job name", "job", jobName)
		return
	}

	// find job metadata
	sess := s.session.Clone()
	defer sess.Close()

	t := &trainingjob.TrainingJob{}
	err = sess.DB(databaseNameJobMetadata).C(collectionNameJobMetadata).
		Find(bson.M{"job_name": jobName, "gpu_type": s.SchedulerID}).One(t)
	if err != nil {
		klog.ErrorS(err, "Failed to find training job metadata", "job", jobName)
		return
		// TODO(heyfey): retry if mongodb temporary down
	}

	s.preprocessTrainingJob(t)
	t.Status = types.JobWaiting

	// update job status in db
	err = sess.DB(databaseNameJobMetadata).C(collectionNameJobMetadata).
		Update(bson.M{"job_name": jobName, "gpu_type": s.SchedulerID}, t)
	if err != nil {
		klog.ErrorS(err, "Failed to update training job in mongodb",
			"job", t.Name, "status", t.Status)
		// TODO(heyfey): error handling
	}

	s.ReadyJobsMap[t.Name] = t
	s.JobNumGPU[t.Name] = 0

	s.TriggerResched()
	s.Metrics.JobsCreatedCounter.Inc()
}

func (s *Scheduler) preprocessTrainingJob(t *trainingjob.TrainingJob) {
	// Currently unable to store resource limits fields in mongodb, so mannually
	// set it after find as a workaround
	// TODO(heyfey)
	setGpuResourceLimits(t)

	// insert gpuNameLabel
	insertLabel(t)
}

// setGpuResourceLimits sets
// MPIJob.Spec.MPIReplicaSpecs["Worker"].Template.Spec.Containers[0].Resources.Limits["nvidia.com/gpu"] to 1
func setGpuResourceLimits(t *trainingjob.TrainingJob) {
	q := *resource.NewQuantity(1, resource.DecimalSI)
	t.Spec.Spec.MPIReplicaSpecs["Worker"].Template.Spec.Containers[0].Resources.Limits["nvidia.com/gpu"] = q
}

// insertLabel insert gpuNameLabel to training job's mpijob and its launcher
// and worker pod.
func insertLabel(t *trainingjob.TrainingJob) {
	t.Spec.Labels[gpuNameLabel] = t.GpuType
	t.Spec.Spec.MPIReplicaSpecs["Launcher"].Template.Labels[gpuNameLabel] = t.GpuType
	t.Spec.Spec.MPIReplicaSpecs["Worker"].Template.Labels[gpuNameLabel] = t.GpuType
}

func (s *Scheduler) DeleteTrainingJob(jobName string) {
	s.SchedulerLock.Lock()
	defer s.SchedulerLock.Unlock()

	// Check if job already exist
	status, err := s.getJobStatus(jobName)
	if err != nil {
		klog.ErrorS(err, "Attempted to delete a non-exist training job", "job", jobName)
		return
	}

	running := (status == types.JobRunning)

	if running || status == types.JobWaiting {
		delete(s.ReadyJobsMap, jobName)
		delete(s.JobNumGPU, jobName)
	} else {
		delete(s.DoneJobsMap, jobName)
	}

	// trigger resched if delete a running job
	if running {
		s.TriggerResched()
	}
	s.Metrics.JobsDeletedCounter.Inc()
}

func (s *Scheduler) getAllTrainingJobHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Endpoint Hit: getAllTrainingJob")
		fmt.Fprintf(w, s.GetAllTrainingJob())
	}
}

// GetAllTrainingJob lists all training jobs and their scheduler, status, and waiting/running/total time
func (s *Scheduler) GetAllTrainingJob() string {
	result := fmt.Sprintf("%-60s %-10s %-10s %-25s %-10s %-10s %-10s\n", "NAME", "STATUS", "WORKERS", "SCHEDULER", "WAITING", "RUNNING", "TOTAL")

	buffer := make([]string, 0)

	s.SchedulerLock.RLock()
	for _, job := range s.ReadyJobsMap {
		str := fmt.Sprintf("%-60s %-10s %-10d %-25s %-10s %-10s %-10s\n",
			job.Name, string(job.Status), s.JobNumGPU[job.Name], s.SchedulerID,
			job.Metrics.WaitingDuration.Round(time.Second),
			job.Metrics.RunningDuration.Round(time.Second),
			job.Metrics.TotalDuration.Round(time.Second))
		buffer = append(buffer, str)
	}
	for _, job := range s.DoneJobsMap {
		str := fmt.Sprintf("%-60s %-10s %-10d %-25s %-10s %-10s %-10s\n",
			job.Name, string(job.Status), s.JobNumGPU[job.Name], s.SchedulerID,
			job.Metrics.WaitingDuration.Round(time.Second),
			job.Metrics.RunningDuration.Round(time.Second),
			job.Metrics.TotalDuration.Round(time.Second))
		buffer = append(buffer, str)
	}
	s.SchedulerLock.RUnlock()

	sort.Strings(buffer)
	for _, str := range buffer {
		result = fmt.Sprintf("%s%s", result, str)
	}

	return result
}

// constructStatusOnRestart is used to re-construct scheduler status when
// re-starting on failure for fault tolerance.
//    1. Find all jobs metadata that are managed by this scheduler in mongodb to
//       reconstruct Ready/DoneJobsMap
//    2. Find existing jobs that are menaged by this scheduler in k8s cluster to
//       re-construct JobNumGPU, also handle job finished if needed
// Potential concern: If any existing mpijob status changes after
// constructStatusOnRestart() and before mpijob informer start running, the
// scheduler may not catch the event. TODO(heyfey): verify the concern
func (s *Scheduler) constructStatusOnRestart() {
	klog.V(4).InfoS("Re-constructing scheduler status")
	defer klog.V(4).InfoS("Re-constructed scheduler status")

	// 1. find running/waiting/completed/failed jobs in mongodb to reconstruct Ready/DoneJobsMap
	sess := s.session.Clone()
	defer sess.Close()

	jobs := []trainingjob.TrainingJob{}
	err := sess.DB(databaseNameJobMetadata).C(collectionNameJobMetadata).Find(bson.M{"gpu_type": s.SchedulerID}).All(&jobs)
	if err != nil {
		klog.ErrorS(err, "Failed to find training job metadata")
		klog.Flush()
		os.Exit(1)
	}
	for _, j := range jobs {
		job := j // make a copy of j so job has its own unique address
		klog.V(5).InfoS("Found job metadata in mongodb", "job", job.Name,
			"status", job.Status, "kind", job.Kind, "gpu", job.GpuType)

		if job.Status == types.JobSubmitted {
			continue
		} else if job.Status == types.JobRunning || job.Status == types.JobWaiting {
			s.preprocessTrainingJob(&job)
			s.ReadyJobsMap[job.Name] = &job
		} else if job.Status == types.JobCompleted || job.Status == types.JobFailed || job.Status == types.JobCanceled {
			s.DoneJobsMap[job.Name] = &job
		} else {
			klog.Flush()
			panic("unrecognized job status")
		}
	}

	// 2. find existing mpijobs in k8s cluster to re-construct JobNumGPU, also handle job finished if needed
	labelSelector := gpuNameLabel + "=" + s.SchedulerID
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	mpiJobs, err := s.mpiClient.MPIJobs(config.Namespace).List(context.TODO(), listOptions)
	if err != nil {
		klog.ErrorS(err, "Failed to list mpijob")
		klog.Flush()
		os.Exit(1)
	}
	for _, mpiJob := range mpiJobs.Items {
		job := mpiJob.GetName()
		_, ok := s.ReadyJobsMap[job]
		if isFinished(mpiJob.Status) {
			// job status is running or waiting in mongodb, but the job is actually completed or failed
			if ok {
				s.handleMPIJobFinished(&mpiJob)
			}
		} else {
			// the job is actually running, job status should be running or waiting in mongodb
			if !ok {
				klog.ErrorS(errors.New("not found"), "Failed to find running job in ReadyJobsMap", "job", job)
				continue
			}
			s.ReadyJobsMap[job].Status = types.JobRunning
			s.JobNumGPU[job] = int(*mpiJob.Spec.MPIReplicaSpecs["Worker"].Replicas)
		}
	}

	klog.V(4).InfoS("Re-constructed scheduler status", "readyJobs", &s.ReadyJobsMap,
		"doneJobs", &s.DoneJobsMap, "jobNumGPU", s.JobNumGPU)
}
