package jobmaster

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"time"

	"github.com/heyfey/celeste/pkg/common/logger"
	"github.com/heyfey/celeste/pkg/common/mongo"
	"github.com/heyfey/celeste/pkg/common/trainingjob"
	"github.com/heyfey/celeste/pkg/common/types"
	"github.com/heyfey/celeste/pkg/scheduler"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	v1 "k8s.io/api/core/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	databaseNameJobInfo = "job_info"
)

// type JobMasterMetrics struct {
// }

type jobMaster struct {
	// schedulers and their names
	schedulers map[string]*scheduler.Scheduler
	// scheduler for each job
	jobScheduler map[string]string
	session      *mgo.Session
	// metrics      *JobMasterMetrics
}

func NewJobMaster() *jobMaster {
	logger.InitLogger()
	log := logger.GetLogger()
	defer logger.Flush()

	schedulers := make(map[string]*scheduler.Scheduler)

	kubeconfig := "/home/heyfey/.kube/config" // TODO: connect to k8s in container
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Error(err, "Failed to build config")
		logger.Flush()
		panic(err)
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

	jm := &jobMaster{
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

// CreateTrainingJob creates a new training job from yaml, assigns it
// to scheduler and triggers a resched
func (jm *jobMaster) CreateTrainingJob(file string) error {
	log := logger.GetLogger()
	defer logger.Flush()

	fmt.Printf("Parsing yaml: %s\n", file)
	log.Info("Parsing yaml", "file", file)
	mpijob, err := yamlToMPIJob(file)
	if err != nil {
		fmt.Printf("Failed to parse yaml: %s\n", file)
		log.Info("Failed to convert yaml to mpijob", "err", err, "file", file)
		return err
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
		return err // TODO: create basic training record in mongodb if not exist
	}

	// add timestamp to name of the training job
	now := time.Now()
	jobName = jobName + "-" + now.Format("20060102-030405")
	mpijob.SetName(jobName)
	setEnvJobName(mpijob, jobName)

	t, err := trainingjob.NewTrainingJob(*mpijob, jobCollection, now)
	if err != nil {
		log.Info("Failed to create training job", "err", err, "job", jobName)
		return err
	}

	info = initJobInfo(info, jobName, t.Config.Epochs)
	err = sess.DB(databaseNameJobInfo).C(jobCollection).Insert(info)
	if err != nil {
		log.Error(err, "Could not insert record to mongo", "database", databaseNameJobInfo, "collection", jobCollection, "job", jobName)
		return err
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
	sched.Queue.Enqueue(*t)
	sched.SchedulerLock.Unlock()

	// trigger resched
	sched.ReschedCh <- now

	fmt.Printf("Training job created: %s\n", jobName)
	fmt.Printf("View your logs by:\n%s\n", "    kubectl logs "+jobName+"-launcher")
	log.Info("Training job created", "job", jobName)
	return nil
}

func yamlToMPIJob(file string) (*kubeflowv1.MPIJob, error) {
	var (
		err  error
		data []byte
	)

	if data, err = ioutil.ReadFile(file); err != nil {
		return nil, err
	}

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
func (jm *jobMaster) DeleteTrainingJob(jobName string) error {
	log := logger.GetLogger()
	defer logger.Flush()

	// TODO: should this be locked?
	schedID := jm.jobScheduler[jobName]
	if schedID == "" {
		fmt.Printf("Training job not found: %s", jobName)
		return errors.New("Training job not found")
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

	fmt.Printf("Training job deleted: %s", jobName)
	log.Info("Training job deleted", "job", jobName)
	return nil
}

// GetTrainingJob lists a training job and its scheduler, status, and waiting/running/total time
func (jm *jobMaster) GetTrainingJob(jobName string) error {
	return nil
}

// GetAllTrainingJob lists all training jobs and their scheduler, status, and waiting/running/total time
func (jm *jobMaster) GetAllTrainingJob() {
	fmt.Printf("%-60s %-10s %-10s %-10s\n", "NAME", "STATUS", "WORKERS", "SCHEDULER")

	for _, scheduler := range jm.schedulers {
		buffer := make([]string, 0)

		scheduler.SchedulerLock.RLock()
		for job, status := range scheduler.JobStatuses {
			str := fmt.Sprintf("%-60s %-10s %-10d %-10s\n", job, string(status), scheduler.JobNumGPU[job], scheduler.SchedulerID)
			buffer = append(buffer, str)
		}
		scheduler.SchedulerLock.RUnlock()

		sort.Strings(buffer)
		for _, str := range buffer {
			fmt.Printf(str)
		}
	}
}

// GetScheduler lists scheduler's number of GPUs and waiting/running/completed/failed jobs
func (jm *jobMaster) GetScheduler(scheduler string) error {
	return nil
}

// GetAllScheduler lists all scheduler and their number of GPUs and waiting/running/completed/failed jobs
func (jm *jobMaster) GetAllScheduler(scheduler string) error {
	return nil
}
