package scheduler

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/heyfey/vodascheduler/pkg/algorithm"
	"github.com/heyfey/vodascheduler/pkg/common/mongo"
	"github.com/heyfey/vodascheduler/pkg/common/trainingjob"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"github.com/heyfey/vodascheduler/pkg/placement"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	mpijobclientset "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned"
	client "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/typed/kubeflow/v1"
	kubeflowinformers "github.com/kubeflow/mpi-operator/pkg/client/informers/externalversions"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeClient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

const (
	databaseNameRunningJobs     = "runnings"
	reschedChannelSize          = 100
	restartChannelSize          = 100
	rateLimitTimeMetricsSeconds = 5
	reschedRateLimitSeconds     = 30
)

type Scheduler struct {
	SchedulerID    string
	GPUAvailable   int
	mpiClient      *client.KubeflowV1Client
	mpiJobInformer cache.SharedIndexInformer

	Queue *TrainingJobQueue
	// MPIJob representations of each training job
	JobMPIJobs map[string]*kubeflowv1.MPIJob
	// Number of allocated GPUs of each training job
	JobNumGPU types.JobScheduleResult
	// Status of each training job. "Running", "Waiting", "Failed" or "Completed"
	JobStatuses map[string]types.JobStatusType
	JobMetrics  map[string]*trainingjob.JobMetrics
	// SchedulerLock is used to protect Queue, JobMPIJobs, JobNumGPU and JobStatuses
	SchedulerLock sync.RWMutex

	// ScheduleAlgorithm is an interface implemented by things that know how to schedule training jobs
	Algorithm algorithm.SchedulerAlgorithm

	// SchedulerMetrics contains run-time metrics of the scheduler
	Metrics SchedulerMetrics

	// channels used for main logic of the scheduler, should only be cousumed by scheduler.Run()
	ReschedCh       chan time.Time
	StopSchedulerCh chan time.Time

	lastResched         time.Time
	reschedBlockedUntil time.Time

	session  *mgo.Session
	database string
	// metrics       *SchedulerMetrics

	PlacementManager *placement.PlacementManager
	ticker           time.Ticker
}

func discoverGPUs(config *rest.Config) (int, error) {
	clientset, err := kubeClient.NewForConfig(config)
	if err != nil {
		return 0, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	totalGPUs := 0
	for _, node := range nodes.Items {
		gpus := node.Status.Capacity["nvidia.com/gpu"]
		totalGPUs += int(gpus.Value())
	}
	return totalGPUs, err
}

// NewScheduler creates a new scheduler
func NewScheduler(id string, config *rest.Config, session *mgo.Session, database string) (*Scheduler, error) {
	q, err := newTrainingJobQueue()
	if err != nil {
		return nil, err
	}

	c, err := client.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	gpus, err := discoverGPUs(config)
	if err != nil {
		return nil, err
	}

	pm, err := placement.NewPlacementManager(id, config)
	if err != nil {
		return nil, err
	}

	mpiJobClientSet, err := mpijobclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kubeflowInformerFactory := kubeflowinformers.NewSharedInformerFactory(mpiJobClientSet, 0)
	mpiJobInformer := kubeflowInformerFactory.Kubeflow().V1().MPIJobs().Informer()

	s := &Scheduler{
		SchedulerID:    id,
		GPUAvailable:   gpus,
		mpiClient:      c,
		mpiJobInformer: mpiJobInformer,

		Queue:         q,
		JobMPIJobs:    map[string]*kubeflowv1.MPIJob{},
		JobNumGPU:     map[string]int{},
		JobStatuses:   map[string]types.JobStatusType{},
		JobMetrics:    map[string]*trainingjob.JobMetrics{},
		SchedulerLock: sync.RWMutex{},

		Algorithm: algorithm.NewElasticFIFO(gpus, id),

		ReschedCh:           make(chan time.Time, reschedChannelSize),
		StopSchedulerCh:     make(chan time.Time),
		reschedBlockedUntil: time.Now(),
		lastResched:         time.Now(),

		session:  session,
		database: database,

		PlacementManager: pm,
		ticker:           *time.NewTicker(rateLimitTimeMetricsSeconds * time.Second),
	}
	s.Metrics = s.initSchedulerMetrics()

	// setup informer callbacks
	// TODO(heyfey): set namespace and replace with FilteringResourceEventHandler
	s.mpiJobInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: s.updateMPIJob,
		},
	)

	return s, nil
}

func (s *Scheduler) Run() {
	klog.InfoS("Starting scheduler", "scheduler", s.SchedulerID)
	defer klog.InfoS("Stopping scheduler", "scheduler", s.SchedulerID)

	// defer close channels ..?

	stopTickerCh := make(chan bool)
	go s.updateTimeMetrics(stopTickerCh)

	stopInformerCh := make(chan struct{})
	go s.mpiJobInformer.Run(stopInformerCh)
	if !cache.WaitForCacheSync(
		stopInformerCh,
		s.mpiJobInformer.HasSynced) {
		err := errors.New("Failed to WaitForCacheSync")
		klog.ErrorS(err, "Scheduler failed to WaitForCacheSync", "scheduler", s.SchedulerID)
		klog.Flush()
		os.Exit(1)
	}

	for {
		select {
		case r := <-s.ReschedCh:
			if r.After(s.lastResched) {
				klog.V(4).InfoS("Received rescheduling event, may be blocked because of rate limit",
					"scheduler", s.SchedulerID, "receivedAtTimestamp", r, "lastReschedulingAtTimestamp", s.lastResched,
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
				klog.V(5).InfoS("Ignored rescheduling event", "scheduler", s.SchedulerID, "receivedAtTimestamp", r)
			}

		case _ = <-s.StopSchedulerCh:
			stopTickerCh <- true
			stopInformerCh <- struct{}{}
			return
		}
	}
}

func (s *Scheduler) resched() {
	klog.V(3).InfoS("Started rescheduling", "scheduler", s.SchedulerID)
	defer klog.V(3).InfoS("Finished rescheduling", "scheduler", s.SchedulerID)

	timer := prometheus.NewTimer(s.Metrics.reschedDuration)
	defer timer.ObserveDuration()

	s.SchedulerLock.Lock()
	oldJobNumGPU := s.JobNumGPU
	s.updateAllJobsInfoFromDB()

	queueCopied := make(algorithm.ReadyJobs, s.Queue.Size())
	copy(queueCopied, s.Queue.Queue)

	timerAlgo := prometheus.NewTimer(s.Metrics.reschedAlgoDuration)
	s.JobNumGPU = s.Algorithm.Schedule(queueCopied)
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
		klog.V(3).InfoS("Skipped ajust placement because nothing changed", "scheduler", s.SchedulerID)
	}

	s.Metrics.reschedCounter.Inc()
}

// updateAllJobsInfoFromDB finds information of all training jobs in mongodb
// and update the training jobs' info with retrieved information
func (s *Scheduler) updateAllJobsInfoFromDB() {
	sess := s.session.Clone()
	defer sess.Close()

	klog.V(4).InfoS("Updating all jobs info", "scheduler", s.SchedulerID)

	for i := 0; i < s.Queue.Size(); i++ {
		klog.V(5).InfoS("Updating job info", "job", s.Queue.Queue[i].JobName)

		t := &s.Queue.Queue[i]
		info := mongo.TrainingJobInfo{}
		err := sess.DB(s.database).C(t.JobCollection).Find(bson.M{"name": t.JobName}).One(&info)
		if err != nil {
			klog.ErrorS(err, "Failed to update job info", "job", t.JobName)
		}
		t.Info.EstimatedRemainningTimeSec = info.EstimatedRemainningTimeSec
		t.Info.Efficiency = info.Efficiency
		t.Info.Speedup = info.Speedup

		klog.V(5).InfoS("Updated job info", "job", s.Queue.Queue[i].JobName)
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
		klog.ErrorS(err, "Failed to remove all records in mongo collection", "scheduler", s.SchedulerID,
			"database", s.database, "collection", s.SchedulerID)
		return err
	}
	// insert running jobs
	for job, status := range s.JobStatuses {
		if status == types.JobRunning {
			entry := mongo.JobRunning{Name: job}
			err = sess.DB(databaseNameRunningJobs).C(s.SchedulerID).Insert(entry)
			if err != nil {
				klog.ErrorS(err, "Failed to insert record to mongo", "scheduler", s.SchedulerID, "database", s.database,
					"collection", s.SchedulerID, "entry", entry)
				return err // TODO: maybe should panic
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
				// don't delete a completed or failed job
				if s.JobStatuses[job] != types.JobCompleted && s.JobStatuses[job] != types.JobFailed {
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
	// Use term "start training job" in logging to distinguish between "create training job" in jobMaster
	klog.V(4).InfoS("Starting training jobs", "jobs", jobs, "scheduler", s.SchedulerID)

	for _, job := range jobs {
		s.startTrainingJob(job)

		// reset time metrics
		s.JobMetrics[job].LastGpuTime = 0
		s.JobMetrics[job].LastRunningTime = 0

		// set Trainingjob.FirstStarted if the job is first started
		if s.JobMetrics[job].RunningTime == 0 {
			if t, err := s.Queue.Get(job); err != nil {
				t.FirstStarted = time.Now()
			}
		}
	}
}

// startTrainingJob creates MPIJob of a training job, with the number of
// worker replicas equal to the number of GPU assigned.
// Should aquire lock before calling it.
func (s *Scheduler) startTrainingJob(job string) error {
	mpiJob := s.JobMPIJobs[job]
	s.setMPIJobWorkerReplicas(mpiJob)

	_, err := s.mpiClient.MPIJobs("default").Create(context.TODO(), mpiJob, metav1.CreateOptions{})
	if err != nil {
		// TODO(heyfey): SHOULD NOT HAPPEN, NEED TO REPAIR IF POSSIBLE
		klog.ErrorS(err, "Failed to start training job, this should not happen", "job", klog.KObj(mpiJob),
			"scheduler", s.SchedulerID)
		// https://github.com/kubeflow/mpi-operator/blob/master/pkg/controllers/v1/mpi_job_controller.go#L875
	} else {
		s.JobStatuses[job] = types.JobRunning
		klog.V(5).InfoS("Started training job", "job", klog.KObj(mpiJob), "scheduler", s.SchedulerID)
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
	klog.V(4).InfoS("Scaling training jobs", "jobs", jobs, "scheduler", s.SchedulerID)

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
		mpiJob, err := s.mpiClient.MPIJobs("default").Get(context.TODO(), job, metav1.GetOptions{}) //TODO: namespace
		if err != nil {
			return err
		}

		s.setMPIJobWorkerReplicas(mpiJob)

		_, err = s.mpiClient.MPIJobs("default").Update(context.TODO(), mpiJob, metav1.UpdateOptions{})
		return err
	})

	// TODO:(heyfey): namespace
	if err != nil {
		klog.ErrorS(err, "Failed to scale training job, this should not happen", "job", klog.KRef("default", job),
			"scheduler", s.SchedulerID) // TODO: SHOULD NOT HAPPEN, NEED TO REPAIR IF POSSIBLE
	} else {
		klog.V(5).InfoS("Scaled training job", "job", klog.KRef("default", job), "scheduler", s.SchedulerID)
	}
	return err
}

// haltTrainingJobMany deletes MPIJobs of training jobs
func (s *Scheduler) haltTrainingJobMany(jobs ...string) {
	// Use term "stop training job" in logging to distinguish between "delete training job" in jobMaster
	klog.V(4).InfoS("Stopping training jobs", "jobs", jobs, "scheduler", s.SchedulerID)

	for _, job := range jobs {
		s.haltTrainingJob(job)

		// reset time metrics
		s.JobMetrics[job].LastWaitingTime = 0
	}
}

// haltTrainingJob deletes MPIJob of a training job.
// Should acquire lock before calling it.
func (s *Scheduler) haltTrainingJob(job string) error {
	//TODO(heyfey): namespace
	err := s.mpiClient.MPIJobs("default").Delete(context.TODO(), job, metav1.DeleteOptions{})
	if err != nil {
		klog.ErrorS(err, "Failed to stop training job, this should not happen", "job", klog.KRef("default", job),
			"scheduler", s.SchedulerID) // TODO: SHOULD NOT HAPPEN, NEED TO REPAIR IF POSSIBLE
		// TODO: error handling if not delete
	} else {
		klog.V(5).InfoS("Stopped training job", "job", klog.KRef("default", job), "scheduler", s.SchedulerID)

		s.JobStatuses[job] = types.JobWaiting
	}
	return err
}

// updateMPIJob handles completion and failure of MPIJobs
func (s *Scheduler) updateMPIJob(oldObj interface{}, newObj interface{}) {
	mpiJob, ok := newObj.(*kubeflowv1.MPIJob)
	if !ok {
		klog.ErrorS(errors.New("unexpected MPIJob type"), "Failed to update MPIJob", "MPIJob", klog.KObj(mpiJob),
			"scheduler", s.SchedulerID)
		return
	}
	klog.V(5).InfoS("MPIJob updated", "mpijob", klog.KObj(mpiJob), "scheduler", s.SchedulerID)

	if isFinished(mpiJob.Status) {
		s.SchedulerLock.Lock()
		defer s.SchedulerLock.Unlock()

		job := mpiJob.GetName()
		if isSucceeded(mpiJob.Status) {
			if s.JobStatuses[job] != types.JobCompleted { // is the first succeeded event
				s.handleJobCompleted(job)
			}
		} else if isFailed(mpiJob.Status) {
			if s.JobStatuses[job] != types.JobFailed { // is the first failed event
				s.handleJobFailed(job)
			}
		}
	}
}

// handleJobCompleted makes essential updates and sends resched signal
// It should only be called by updateMPIJob, and should acquire lock before
// calling it
func (s *Scheduler) handleJobCompleted(job string) {
	klog.InfoS("Training job completed", "job", klog.KRef("default", job), "scheduler", s.SchedulerID,
		"waitedTotalSeconds", s.JobMetrics[job].WaitingTime.Seconds(),
		"ranTotalSeconds", s.JobMetrics[job].RunningTime.Seconds(),
		"gpuTotalSeconds", s.JobMetrics[job].GpuTime.Seconds(),
		"elaspedTotalSeconds", s.JobMetrics[job].TotalTime.Seconds()) // TODO(heyfey): namespace

	s.JobStatuses[job] = types.JobCompleted
	s.Queue.Delete(job)
	s.Metrics.jobsCompletedCounter.Inc()

	now := time.Now()
	s.ReschedCh <- now
	return
}

// handleJobFailed makes essential updates and sends rescheduling signal.
// Note that if a job uses OnFailure restart policy, the watcher won't receive
// a event with JobFailed phase even when the job fails, thus this function
// won't be called in this situation.
// (Do not use ExitCode since there are hanging issue when job fails)
// It should only be called by updateMPIJob, and should acquire lock before
// calling it
func (s *Scheduler) handleJobFailed(job string) {
	klog.InfoS("Training job failed", "job", job, "scheduler", s.SchedulerID)

	s.JobStatuses[job] = types.JobFailed
	s.Queue.Delete(job)
	s.Metrics.jobsFailedCounter.Inc()

	now := time.Now()
	s.ReschedCh <- now
}

// Update cluster view (e.g. availiable GPU count), may need clusterView structure
// func (s *Scheduler) updateClusterView() () {
// }

func (s *Scheduler) Stop() {
	s.StopSchedulerCh <- time.Now()
	return
}

// updateTimeMetrics updates time metrics of all training jobs every
// rateLimitTimeMetricsSeconds seconds.
// Depends on the scheduling algorithm, it may also checks for priority changes
// and/or triggers rescheduling.
func (s *Scheduler) updateTimeMetrics(stopTickerCh chan bool) {
	for {
		select {
		case <-stopTickerCh:
			klog.InfoS("Stopped ticker", "scheduler", s.SchedulerID)
			return
		case <-s.ticker.C:
			// Some algorithms change priority of training jobs according to time metrics,
			// and may trigger rescheduling if that happens.
			priorityChanged := false

			s.SchedulerLock.Lock()
			for job, status := range s.JobStatuses {
				elasped := time.Since(s.JobMetrics[job].LastUpdated)
				if status == types.JobRunning {
					s.JobMetrics[job].RunningTime += elasped
					s.JobMetrics[job].GpuTime += elasped * time.Duration(s.JobNumGPU[job])
					s.JobMetrics[job].TotalTime += elasped
					s.JobMetrics[job].LastRunningTime += elasped
					s.JobMetrics[job].LastGpuTime += elasped * time.Duration(s.JobNumGPU[job])
				} else if status == types.JobWaiting {
					s.JobMetrics[job].WaitingTime += elasped
					s.JobMetrics[job].TotalTime += elasped
					s.JobMetrics[job].LastWaitingTime += elasped
				}
				s.JobMetrics[job].LastUpdated = time.Now()

				// Tiresias' rules of priority changes
				if (s.Algorithm.GetName() == "Tiresias" || s.Algorithm.GetName() == "ElasticTiresias") &&
					(status == types.JobRunning || status == types.JobWaiting) {
					t, err := s.Queue.Get(job)
					if err != nil {
						klog.ErrorS(err, "Could not find training job in queue, this should not happen", "job", job,
							"scheduler", s.SchedulerID)
						continue
					}
					// demote the job if last GPU time crosses threshold
					if s.JobMetrics[job].LastGpuTime.Seconds() > algorithm.TiresiasThresholdsSec[t.Priority] {
						t.Priority = algorithm.TiresiasDemotePriority(t.Priority)
						priorityChanged = true
						klog.V(4).InfoS("Demoted priority to training job", "job", job, "priority", t.Priority,
							"scheduler", s.SchedulerID)

						// promote the job if last waiting time longer than STARVELIMIT
					} else if s.JobMetrics[job].LastWaitingTime.Seconds() >= s.JobMetrics[job].LastRunningTime.Seconds()*float64(algorithm.TiresiasPromoteKnob) &&
						t.Priority > 0 {
						t.Priority = algorithm.TiresiasPromotePriority(t.Priority)
						priorityChanged = true
						klog.V(4).InfoS("Promoted priority to training job", "job", job, "priority", t.Priority,
							"scheduler", s.SchedulerID)
					}
				}
			}
			s.SchedulerLock.Unlock()

			// trigger rescheduling if priority changed
			if priorityChanged {
				klog.V(3).InfoS("Triggered rescheduling because of priority changed", "scheduler", s.SchedulerID)
				s.ReschedCh <- time.Now()
			}
		}
	}
}
