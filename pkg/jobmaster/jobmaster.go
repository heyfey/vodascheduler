package jobmaster

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/heyfey/vodascheduler/pkg/common/logger"
	"github.com/heyfey/vodascheduler/pkg/common/mongo"
	"github.com/heyfey/vodascheduler/pkg/common/trainingjob"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"github.com/heyfey/vodascheduler/pkg/scheduler"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/client-go/rest"
)

const (
	databaseNameJobInfo = "job_info"
)

// type JobMasterMetrics struct {
// }

type JobMaster struct {
	// schedulers and their names
	schedulers map[string]*scheduler.Scheduler
	// scheduler for each job
	jobScheduler map[string]string
	session      *mgo.Session
	// metrics      *JobMasterMetrics
}

func NewJobMaster() *JobMaster {
	log := logger.GetLogger()
	defer logger.Flush()

	schedulers := make(map[string]*scheduler.Scheduler)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Failed to build config")
		logger.Flush()
		panic(err.Error())
	}

	session := mongo.ConnectMongo()

	// TODO: create schedulers
	// by searching node labels (e.g. "nvidia-gtx-1080ti", "nvidia-tesla-v100"):
	// gpuTypes := getAllResourceTypes()
	// for _, gpuType := range gpuTypes { ...create scheduler
	//
	// or by arguments:
	// gpuTypes := ["nvidia-gtx-1080ti", ... ]
	// for _, gpuType := range gpuTypes { ...create scheduler
	gpuType := "default"
	sched, err := scheduler.NewScheduler(gpuType, config, session.Copy(), databaseNameJobInfo)
	if err != nil {
		log.Error(err, "Failed to create scheduler", "gpuType", gpuType)
		logger.Flush()
		panic(err)
	} else {
		schedulers[gpuType] = sched
	}

	jm := &JobMaster{
		schedulers:   schedulers,
		jobScheduler: map[string]string{},
		session:      session,
	}

	// start all schedulers
	for gpuType, sched := range jm.schedulers {
		log.Info("Starting scheduler", "gpuType", gpuType)
		go sched.Run()
	}
	return jm
}

// CreateTrainingJob creates a new training job from bytes data, assigns it
// to scheduler, triggers a resched and returns the name of the training job.
func (jm *JobMaster) CreateTrainingJob(data []byte) (string, error) {
	log := logger.GetLogger()
	defer logger.Flush()

	mpijob, err := bytesToMPIJob(data)
	if err != nil {
		log.Info("Failed to convert data to mpijob", "err", err)
		return "", err
	}

	// find history information of the training job by its name (assume history exists //TODO)
	// and insert a new record to mongodb with modified job name (with timestamp added)
	jobName := mpijob.GetName()
	jobCollection := jobName

	sess := jm.session.Clone()
	defer sess.Close()
	info := mongo.TrainingJobInfo{}
	err = sess.DB(databaseNameJobInfo).C(jobCollection).Find(bson.M{"name": jobName}).One(&info)
	if err != nil {
		log.Info("Could not find job info in mongo", "err", err, "database", databaseNameJobInfo, "collection", jobCollection, "job", jobName)
		return "", err // TODO: create basic training record in mongodb if not exist
	}

	// add timestamp to name of the training job
	now := time.Now()
	jobName = jobName + "-" + now.Format("20060102-030405")
	mpijob.SetName(jobName)
	setEnvJobName(mpijob, jobName)
	addPodAffinity(mpijob, jobName)

	t, err := trainingjob.NewTrainingJob(*mpijob, jobCollection, now)
	if err != nil {
		log.Info("Failed to create training job", "err", err, "job", jobName)
		return "", err
	}

	info = initJobInfo(info, jobName, t.Config.Epochs)
	err = sess.DB(databaseNameJobInfo).C(jobCollection).Insert(info)
	if err != nil {
		log.Error(err, "Could not insert record to mongo", "database", databaseNameJobInfo, "collection", jobCollection, "job", jobName)
		return "", err
	}
	// sess.Close()

	// TODO: find gpuType from yaml
	gpuType := "nvidia-gtx-1080ti"
	sched := jm.schedulers[gpuType]
	if sched == nil {
		sched = jm.schedulers["default"]
	}

	// submit training job to scheduler
	jm.jobScheduler[jobName] = sched.SchedulerID
	sched.SchedulerLock.Lock()
	sched.JobMPIJobs[jobName] = mpijob
	sched.JobNumGPU[jobName] = 0
	sched.JobStatuses[jobName] = types.JobWaiting
	sched.JobMetrics[jobName] = trainingjob.NewJobMetrics(jobName)
	sched.Queue.Enqueue(*t)
	sched.SchedulerLock.Unlock()

	// trigger resched
	sched.ReschedCh <- now

	log.Info("Training job created", "job", jobName)
	return jobName, nil
}

func bytesToMPIJob(data []byte) (*kubeflowv1.MPIJob, error) {
	var err error

	if data, err = yaml2.ToJSON(data); err != nil {
		return nil, err
	}

	mpijob := &kubeflowv1.MPIJob{}
	if err = json.Unmarshal(data, mpijob); err != nil {
		return nil, err
	}

	return mpijob, nil
}

// setEnvJobName sets the environment variable "JOB_NAME" of mpijob
func setEnvJobName(mpijob *kubeflowv1.MPIJob, name string) {
	launcherSpec := mpijob.Spec.MPIReplicaSpecs["Launcher"]
	// there should be only one container in the spec
	env := launcherSpec.Template.Spec.Containers[0].Env
	for i := 0; i < len(env); i++ {
		if env[i].Name == string(types.JobName) {
			env[i].Value = name
			return
		}
	}
	// environment variable "JOB_NAME" not found, add it by ourselves
	env = append(env, v1.EnvVar{Name: string(types.JobName), Value: name})
}

// addPodAffinity adds pod affinity to all worker pods.
// Cautious that it will erase all affinity in the origianl MPIJob.
func addPodAffinity(mpijob *kubeflowv1.MPIJob, name string) {
	workerSpec := mpijob.Spec.MPIReplicaSpecs["Worker"]
	// TODO: check workerSpec.Template.Spec.Affinity == nil; we don't want to erase it if not nil
	requirement := []metav1.LabelSelectorRequirement{{
		Key:      "mpi-job-name",
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{name},
	}}

	term := v1.WeightedPodAffinityTerm{
		Weight: 90,
		PodAffinityTerm: v1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{MatchExpressions: requirement},
			TopologyKey:   "kubernetes.io/hostname",
		},
	}

	affinity := &v1.Affinity{PodAffinity: &v1.PodAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{term},
	}}

	workerSpec.Template.Spec.Affinity = affinity
}

// TODO: Some jobs need to calcaulate info base on steps instead of epochs
func initJobInfo(basicInfo mongo.TrainingJobInfo, jobName string, epochs int) mongo.TrainingJobInfo {
	info := basicInfo
	info.Name = jobName
	info.CurrentEpoch = 0
	info.ElaspedTimeSec = 0.0
	info.EstimatedRemainningTimeSec = float32(epochs) * basicInfo.EpochTimeSec["1"]
	info.GpuTimeSec = 0.0
	info.RemainningEpochs = int32(epochs)
	info.RunningTimeSec = 0.0
	info.TotalEpochs = int32(epochs)
	return info
}

// DeleteTrainingJob deletes a training job from scheduler, and triggers a resched
func (jm *JobMaster) DeleteTrainingJob(jobName string) error {
	log := logger.GetLogger()
	defer logger.Flush()

	// TODO: should this be locked?
	schedID := jm.jobScheduler[jobName]
	if schedID == "" {
		return errors.New(fmt.Sprintf("Training job not found: %s", jobName))
	}
	delete(jm.jobScheduler, jobName)

	sched := jm.schedulers[schedID]
	if sched == nil {
		err := errors.New("Scheduler not found, this should not happen")
		log.Error(err, "Scheduler not found, this should not happen", "scheduler", schedID)
		logger.Flush()
		panic(err)
	}
	delete(jm.schedulers, schedID)

	locked := true
	sched.SchedulerLock.Lock()
	defer func() {
		if locked {
			sched.SchedulerLock.Unlock()
		}
	}()

	scheduled := sched.JobNumGPU[jobName] != 0
	delete(sched.JobMPIJobs, jobName)
	delete(sched.JobNumGPU, jobName)
	delete(sched.JobStatuses, jobName)
	delete(sched.JobMetrics, jobName)
	err := sched.Queue.Delete(jobName)
	if err != nil {
		log.Info("Deleting a completed job", "job", jobName)
	}

	// TODO: remove job info from mongodb

	sched.SchedulerLock.Unlock()
	locked = false

	// trigger resched if delete a scheduled job
	if scheduled {
		sched.ReschedCh <- time.Now()
	}

	log.Info("Training job deleted", "job", jobName)
	return nil
}

// GetTrainingJob lists a training job and its scheduler, status, and waiting/running/total time
func (jm *JobMaster) GetTrainingJob(jobName string) error {
	return nil
}

// GetAllTrainingJob lists all training jobs and their scheduler, status, and waiting/running/total time
func (jm *JobMaster) GetAllTrainingJob() string {
	result := fmt.Sprintf("%-60s %-10s %-10s %-10s %-10s %-10s %-10s\n", "NAME", "STATUS", "WORKERS", "SCHEDULER", "WAITING", "RUNNING", "TOTAL")

	for _, scheduler := range jm.schedulers {
		buffer := make([]string, 0)

		scheduler.SchedulerLock.RLock()
		for job, status := range scheduler.JobStatuses {
			str := fmt.Sprintf("%-60s %-10s %-10d %-10s %-10s %-10s %-10s\n", job, string(status), scheduler.JobNumGPU[job], scheduler.SchedulerID, scheduler.JobMetrics[job].WaitingTime.Round(time.Second), scheduler.JobMetrics[job].RunningTime.Round(time.Second), scheduler.JobMetrics[job].TotalTime.Round(time.Second))
			buffer = append(buffer, str)
		}
		scheduler.SchedulerLock.RUnlock()

		sort.Strings(buffer)
		for _, str := range buffer {
			result = fmt.Sprintf("%s%s", result, str)
		}
	}
	return result
}

// GetScheduler lists scheduler's number of GPUs and waiting/running/completed/failed jobs
func (jm *JobMaster) GetScheduler(scheduler string) error {
	return nil
}

// GetAllScheduler lists all scheduler and their number of GPUs and waiting/running/completed/failed jobs
func (jm *JobMaster) GetAllScheduler(scheduler string) error {
	return nil
}
