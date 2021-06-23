package scheduler

import (
	"errors"
	"sync"
	"time"

	"github.com/heyfey/celeste/pkg/algorithm"
	"github.com/heyfey/celeste/pkg/common/logger"
	"github.com/heyfey/celeste/pkg/common/mongo"
	"github.com/heyfey/celeste/pkg/common/trainingjob"
	"github.com/heyfey/celeste/pkg/common/types"
	kubeflowcommon "github.com/kubeflow/common/pkg/apis/common/v1"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	client "github.com/kubeflow/mpi-operator/pkg/client/clientset/versioned/typed/kubeflow/v1"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
)

const (
	databaseNameRunningJobs = "runnings"
	reschedChannelSize      = 100
	restartChannelSize      = 100
	// Number of GPUs of this scheduler
	// TODO: should be user specified or discover in runtime
	gpus = 8

	rateLimitTimeMetricsSeconds = 5
	reschedRateLimitSeconds     = 30
)

// type SchedulerMetrics struct {
// }

type Scheduler struct {
	SchedulerID  string
	GPUAvailable int
	mpiClient    *client.KubeflowV1Client

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

	// channels used for main logic of the scheduler, should only be cousumed by scheduler.Run()
	ReschedCh       chan time.Time
	StopSchedulerCh chan time.Time

	lastResched         time.Time
	reschedBlockedUntil time.Time

	session  *mgo.Session
	database string
	// metrics       *SchedulerMetrics
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

	// TODO: find GPUAvailable

	s := &Scheduler{
		SchedulerID:  id,
		GPUAvailable: gpus, // TODO
		mpiClient:    c,

		Queue:         q,
		JobMPIJobs:    map[string]*kubeflowv1.MPIJob{},
		JobNumGPU:     map[string]int{},
		JobStatuses:   map[string]types.JobStatusType{},
		JobMetrics:    map[string]*trainingjob.JobMetrics{},
		SchedulerLock: sync.RWMutex{},

		Algorithm: algorithm.NewFIFO(gpus, id), // TODO

		ReschedCh:           make(chan time.Time, reschedChannelSize),
		StopSchedulerCh:     make(chan time.Time),
		reschedBlockedUntil: time.Now(),
		lastResched:         time.Now(),

		session:  session,
		database: database,
	}
	return s, nil
}

func (s *Scheduler) Run() {
	log := logger.GetLogger()
	defer logger.Flush()

	log.Info("Starting scheduler", "scheduler", s.SchedulerID)
	defer log.Info("Stopping scheduler", "scheduler", s.SchedulerID)

	// defer close channels ..?
	go s.watchingMPIJobModified()
	go s.updateTimeMetrics()

	for {
		select {
		case r := <-s.ReschedCh:
			if r.After(s.lastResched) {
				log.V(4).Info("Received resched event, may be blocked because of rate limit", "scheduler", s.SchedulerID,
					"received", r, "lastResched", s.lastResched, "blockedUntil", s.reschedBlockedUntil)

				for time.Now().Before(s.reschedBlockedUntil) {
					time.Sleep(2)
				}
				s.resched()
				s.lastResched = time.Now()
				s.reschedBlockedUntil = s.lastResched.Add(time.Second * reschedRateLimitSeconds)
			} else {
				// The resched events with timestamp before s.lastResched are
				// considered sastified, simply ignore them.
				log.V(5).Info("Ignored resched event", "scheduler", s.SchedulerID, "received", r)
			}

		case _ = <-s.StopSchedulerCh:
			return
		}
	}
}

func (s *Scheduler) resched() {
	log := logger.GetLogger()
	defer logger.Flush()
	log.V(3).Info("Started resched", "scheduler", s.SchedulerID)

	s.SchedulerLock.Lock()
	oldJobNumGPU := s.JobNumGPU
	s.updateAllJobsInfoFromDB()

	queueCopied := make(algorithm.ReadyJobs, s.Queue.Size())
	copy(queueCopied, s.Queue.Queue)
	s.JobNumGPU = s.Algorithm.Schedule(queueCopied)
	// s.SchedulerLock.Unlock() // may want to unlock here to implement cancelling mechanism

	s.applySchedulerResults(oldJobNumGPU)
	s.recordRunningJobsInDB()
	s.SchedulerLock.Unlock()

	log.V(3).Info("Finished resched", "scheduler", s.SchedulerID)
}

// updateAllJobsInfoFromDB finds information of all training jobs in mongodb
// and update the training jobs' info with retrieved information
func (s *Scheduler) updateAllJobsInfoFromDB() {
	log := logger.GetLogger()
	defer logger.Flush()

	sess := s.session.Clone()
	defer sess.Close()

	log.V(4).Info("Updating all jobs info", "scheduler", s.SchedulerID)

	for i := 0; i < s.Queue.Size(); i++ {
		log.V(5).Info("Updating job info", "job", s.Queue.Queue[i].JobName)

		t := &s.Queue.Queue[i]
		info := mongo.TrainingJobInfo{}
		err := sess.DB(s.database).C(t.JobCollection).Find(bson.M{"name": t.JobName}).One(&info)
		if err != nil {
			log.Error(err, "Could not update job info", "job", t.JobName)
		}
		t.Info.EstimatedRemainningTimeSec = info.EstimatedRemainningTimeSec
		t.Info.Efficiency = info.Efficiency
		t.Info.Speedup = info.Speedup

		log.V(5).Info("Updated job info", "job", s.Queue.Queue[i].JobName)
	}
}

// recordRunningJobsToDB records witch jobs are currently running in mongodb,
// these records would be fetched by the metrics collector
// If we saves all jobs' scheduler status in mongodb, we may don't need this
func (s *Scheduler) recordRunningJobsInDB() error {
	log := logger.GetLogger()
	defer logger.Flush()

	sess := s.session.Clone()
	defer sess.Close()

	// clear the whole collection
	_, err := sess.DB(databaseNameRunningJobs).C(s.SchedulerID).RemoveAll(nil)
	if err != nil {
		log.Error(err, "Failed to remove all records in mongo collection", "scheduler", s.SchedulerID, "database", s.database, "collection", s.SchedulerID)
		return err
	}
	// insert running jobs
	for job, status := range s.JobStatuses {
		if status == types.JobRunning {
			entry := mongo.JobRunning{Name: job}
			err = sess.DB(databaseNameRunningJobs).C(s.SchedulerID).Insert(entry)
			if err != nil {
				log.Error(err, "Could not insert record to mongo", "scheduler", s.SchedulerID, "database", s.database, "collection", s.SchedulerID, "entry", entry)
				return err // TODO: maybe should panic
			}
		}
	}
	return nil
}

// applySchedulerResults performs required changes to achieve new JobScheduleResult
func (s *Scheduler) applySchedulerResults(oldResult map[string]int) {
	halts, scaleIns, scaleOuts, starts := s.compareResults(oldResult)
	s.haltTrainingJobMany(halts...)
	s.scaleTrainingJobMany(scaleIns...)
	// (optinal) wait for ajustments complete
	s.startTrainingJobMany(starts...)
	s.scaleTrainingJobMany(scaleOuts...)
	// (optinal) wait for ajustments complete
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

// startTrainingJobs creates MPIJobs of training jobs
func (s *Scheduler) startTrainingJobMany(jobs ...string) {
	log := logger.GetLogger()
	defer logger.Flush()

	log.V(4).Info("Trying to start multiple training jobs", "jobs", jobs, "scheduler", s.SchedulerID)

	for _, job := range jobs {
		log.V(5).Info("Trying to start training job", "job", job, "scheduler", s.SchedulerID)

		err := s.startTrainingJob(job)
		if err != nil {
			log.Error(err, "Could not start training job, this should not happen", "job", job, "scheduler", s.SchedulerID) // TODO: SHOULD NOT HAPPEN, NEED TO REPAIR IF POSSIBLE
			// https://github.com/kubeflow/mpi-operator/blob/master/pkg/controllers/v1/mpi_job_controller.go#L875
		}
		// reset time metrics
		s.JobMetrics[job].LastGpuTime = 0
		s.JobMetrics[job].LastRunningTime = 0

		// set Trainingjob.FirstStarted if the job is first started
		if s.JobMetrics[job].RunningTime == 0 {
			if t, err := s.Queue.Get(job); err != nil {
				t.FirstStarted = time.Now()
			}
		}

		log.V(5).Info("Training job started", "job", job, "scheduler", s.SchedulerID)
	}
}

// startTrainingJobs creates MPIJob of a training job, with the number of
// worker replicas equal to the number of GPU assigned
// should aquire lock before calling it
func (s *Scheduler) startTrainingJob(job string) error {
	mpiJob := s.JobMPIJobs[job]
	s.setMPIJobWorkerReplicas(mpiJob)

	_, err := s.mpiClient.MPIJobs("default").Create(mpiJob)
	if err == nil {
		s.JobStatuses[job] = types.JobRunning
	}
	return err
}

// setMPIJobWorkerReplicas sets the number of worker replicas of a MPIJob to the number of GPU assigned
func (s *Scheduler) setMPIJobWorkerReplicas(mpiJob *kubeflowv1.MPIJob) {
	workerSpec := mpiJob.Spec.MPIReplicaSpecs["Worker"]
	*workerSpec.Replicas = int32(s.JobNumGPU[mpiJob.GetName()])
}

// scaleTrainingJobs scales MPIJob of training jobs
func (s *Scheduler) scaleTrainingJobMany(jobs ...string) {
	log := logger.GetLogger()
	defer logger.Flush()

	log.V(4).Info("Trying to scale multiple training jobs", "jobs", jobs, "scheduler", s.SchedulerID)

	for _, job := range jobs {
		log.V(5).Info("Trying to scale training job", "job", job, "scheduler", s.SchedulerID)

		err := s.scaleTrainingJob(job)
		if err != nil {
			log.Error(err, "Could not scale training job, this should not happen", "job", job, "scheduler", s.SchedulerID) // TODO: SHOULD NOT HAPPEN, NEED TO REPAIR IF POSSIBLE
		}

		log.V(5).Info("Training job scaled", "job", job, "scheduler", s.SchedulerID)
	}
}

// scaleOneTrainingJob gets and updates worker replicas of a MPIJob, wrapped with retry.RetryOnConflict
// should acquire lock before calling it
// Though jobs may completed or failed during the scaling, it's no harm to update a completed/failed job,
// and the results would be fixed by the following resched
func (s *Scheduler) scaleTrainingJob(job string) error {
	// TODO: may want to check job status == types.JobRunning
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		mpiJob, err := s.mpiClient.MPIJobs("default").Get(job, metav1.GetOptions{}) //TODO: namespace
		if err != nil {
			return err
		}

		s.setMPIJobWorkerReplicas(mpiJob)

		_, err = s.mpiClient.MPIJobs("default").Update(mpiJob)
		if err != nil {
			return err
		}
		return err
	})
	return err
}

// haltTrainingJobs deletes MPIJobs of training jobs
func (s *Scheduler) haltTrainingJobMany(jobs ...string) {
	log := logger.GetLogger()
	defer logger.Flush()

	log.V(4).Info("Trying to delete multiple training jobs", "jobs", jobs, "scheduler", s.SchedulerID)

	for _, job := range jobs {
		log.V(5).Info("Trying to delete training job", "job", job, "scheduler", s.SchedulerID)

		err := s.haltTrainingJob(job)
		if err != nil {
			log.Error(err, "Could not delete training job, this should not happen", "job", job, "scheduler", s.SchedulerID) // TODO: SHOULD NOT HAPPEN, NEED TO REPAIR IF POSSIBLE
			// TODO: error handling if not delete
		}
		// reset time metrics
		s.JobMetrics[job].LastWaitingTime = 0

		log.V(5).Info("Training job deleted", "job", job, "scheduler", s.SchedulerID)
	}
}

// haltOneTrainingJob deletes MPIJob of a training job
// should acquire lock before calling it
func (s *Scheduler) haltTrainingJob(job string) error {
	err := s.mpiClient.MPIJobs("default").Delete(job, &metav1.DeleteOptions{})
	if err == nil {
		s.JobStatuses[job] = types.JobWaiting
	}
	return err
}

// watchingMPIJobModified keep watching events of MPIJobs and handles it if needed
func (s *Scheduler) watchingMPIJobModified() {
	log := logger.GetLogger()
	defer logger.Flush()

	// mpijobs will have the latest rescouce version
	mpijobs, err := s.mpiClient.MPIJobs("").List(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Failed to list MPIJobs", "scheduler", s.SchedulerID)
		logger.Flush()
		panic(err)
	}

	// TODO: may want to specify namespace
	watcher, err := watchtools.NewRetryWatcher(mpijobs.ResourceVersion, s.mpiClient.MPIJobs(""))
	if err != nil {
		log.Error(err, "Failed create watcher", "scheduler", s.SchedulerID)
		logger.Flush()
		panic(err)
	}

	log.Info("Start watching mpijob", "scheduler", s.SchedulerID)

	for event := range watcher.ResultChan() {
		if event.Type == watch.Modified {
			m, ok := event.Object.(*kubeflowv1.MPIJob)
			if !ok {
				log.Error(errors.New("unexpected type"), "Watcher got unexpected type of event", "scheduler", s.SchedulerID)
				continue
			}
			// get current phase from the last element of Conditions
			// TODO: determine phase using existing api
			// https://github.com/kubeflow/mpi-operator/blob/6ee71d45dde0e71229b7fa91065e0c6bb503cd92/pkg/controllers/v1/mpi_job_controller_status.go#L86
			last := len(m.Status.Conditions) - 1
			if last < 0 {
				continue
			}
			phase := m.Status.Conditions[last].Type
			name := m.GetName()
			if phase == kubeflowcommon.JobSucceeded {
				s.SchedulerLock.Lock()
				if s.JobStatuses[name] != types.JobCompleted { // is the first succeeded event
					s.handleJobCompleted(name)
				}
				s.SchedulerLock.Unlock()
			} else if phase == kubeflowcommon.JobFailed {
				s.SchedulerLock.Lock()
				if s.JobStatuses[name] != types.JobFailed { // is the first failed event
					s.handleJobFailed(name)
				}
				s.SchedulerLock.Unlock()
			}
		}
	}
}

// handleJobCompleted makes essential updates and sends resched signal
// It should only be called by watchingMPIJobModified, and should
// acquire lock before calling it
func (s *Scheduler) handleJobCompleted(job string) {
	log := logger.GetLogger()
	defer logger.Flush()

	log.Info("Training job completed", "job", job, "scheduler", s.SchedulerID,
		"waitedTotalSeconds", s.JobMetrics[job].WaitingTime.Seconds(),
		"ranTotalSeconds", s.JobMetrics[job].RunningTime.Seconds(),
		"gpuTotalSeconds", s.JobMetrics[job].GpuTime.Seconds(),
		"elaspedTotalSeconds", s.JobMetrics[job].TotalTime.Seconds())

	s.JobStatuses[job] = types.JobCompleted
	s.Queue.Delete(job)

	now := time.Now()
	s.ReschedCh <- now
	return
}

// handleJobFailed makes essential updates and sends resched signal.
// Note that if a job uses OnFailure restart policy, the watcher won't receive
// a event with JobFailed phase even when the job fails, thus this function
// won't be called in this situation.
// (Do not use ExitCode since there are hanging issue when job fails)
// It should only be called by watchingMPIJobModified, and should
// acquire lock before calling it
func (s *Scheduler) handleJobFailed(job string) {
	log := logger.GetLogger()
	defer logger.Flush()

	log.Info("Training job failed", "job", job, "scheduler", s.SchedulerID)

	s.JobStatuses[job] = types.JobFailed
	s.Queue.Delete(job)

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
// and/or triggers resched.
func (s *Scheduler) updateTimeMetrics() {
	log := logger.GetLogger()
	defer logger.Flush()

	for {
		time.Sleep(time.Duration(rateLimitTimeMetricsSeconds) * time.Second)

		// some algorithms change priority of training jobs according to time metrics,
		// and may trigger resched if that happens.
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
			if (s.Algorithm.GetName() == "Tiresias" || s.Algorithm.GetName() == "ElasticTiresias") && (status == types.JobRunning || status == types.JobWaiting) {
				t, err := s.Queue.Get(job)
				if err != nil {
					log.Error(err, "This should not happen", "job", job, "scheduler", s.SchedulerID)
					continue
				}
				// demote the job if last GPU time crosses threshold
				if s.JobMetrics[job].LastGpuTime.Seconds() > algorithm.TiresiasThresholdsSec[t.Priority] {
					t.Priority = algorithm.TiresiasDemotePriority(t.Priority)
					priorityChanged = true
					log.V(4).Info("Priority demoted to training job", "job", job, "priority", t.Priority, "scheduler", s.SchedulerID)

					// promote the job if last waiting time longer than STARVELIMIT
				} else if s.JobMetrics[job].LastWaitingTime.Seconds() >= s.JobMetrics[job].LastRunningTime.Seconds()*float64(algorithm.TiresiasPromoteKnob) && t.Priority > 0 {
					t.Priority = algorithm.TiresiasPromotePriority(t.Priority)
					priorityChanged = true
					log.V(4).Info("Priority promoted to training job", "job", job, "priority", t.Priority, "scheduler", s.SchedulerID)
				}
			}
		}
		s.SchedulerLock.Unlock()

		// trigger resched if priority changed
		if priorityChanged {
			log.V(3).Info("Priority changed", "scheduler", s.SchedulerID)
			s.ReschedCh <- time.Now()
		}
	}
}
