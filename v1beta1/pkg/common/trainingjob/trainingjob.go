package trainingjob

import (
	"errors"
	"strconv"
	"time"

	"github.com/heyfey/vodascheduler/pkg/common/types"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
)

const (
	maxNumGpu    = 32
	gpuNameLabel = "vodascheduler/accelerator"
)

type TrainingJob struct {
	Name       string              `bson:"job_name" json:"job_name"`
	Category   string              `bson:"job_category" json:"job_category"`
	User       string              `bson:"user" json:"user"`
	Kind       types.JobKindType   `bson:"kind" json:"kind"`
	Spec       *kubeflowv1.MPIJob  `bson:"spec" json:"spec"`
	GpuType    string              `bson:"gpu_type" json:"gpu_type"`
	Priority   int                 `bson:"priority" json:"priority"`
	Status     types.JobStatusType `bson:"status" json:"status"`
	SubmitTime time.Time           `bson:"submit_timestamp" json:"submit_timestamp"`
	FinishTime time.Time           `bson:"finish_timestamp" json:"finish_timestamp"`
	Config     JobConfig           `bson:"config" json:"config"`
	Metrics    *JobMetrics         `bson:"time_metrics" json:"time_metrics"`
	Info       JobInfo             `bson:"info" json:"info"`
}

// JobConfig represents user training configurations specified by user
type JobConfig struct {
	NumProc    int `bson:"num_proc" json:"num_proc"`
	MinNumProc int `bson:"min_num_proc" json:"min_num_proc"`
	MaxNumProc int `bson:"max_num_proc" json:"max_num_proc"`
	Epochs     int `bson:"epochs" json:"epochs"`
}

// JobMetrics represents time metrics of a job.
type JobMetrics struct {
	RunningDuration time.Duration `bson:"running_time" json:"running_time"`
	WaitingDuration time.Duration `bson:"waiting_time" json:"waiting_time"`
	GpuDuration     time.Duration `bson:"gpu_time" json:"gpu_time"`
	TotalDuration   time.Duration `bson:"total_time" json:"total_time"`

	LastRunningDuration time.Duration `bson:"last_running_time" json:"last_running_time"`
	LastWaitingDuration time.Duration `bson:"last_waiting_time" json:"last_waiting_time"`
	LastGpuDuration     time.Duration `bson:"last_gpu_time" json:"last_gpu_time"`

	// first started time of the training job, used in Tiresias algorithm
	FirstStartTime time.Time `bson:"first_start_timestamp" json:"first_start_timestamp"`

	LastUpdateTime time.Time `bson:"last_update_timestamp" json:"last_update_timestamp"`
}

// JobInfo represents history/estimated information of a training job
type JobInfo struct {
	Name                       string             `bson:"job_name" json:"job_name"`
	Category                   string             `bson:"job_category" json:"job_category"`
	GpuType                    string             `bson:"gpu_type" json:"gpu_type"`
	EstimatedRemainningTimeSec float32            `bson:"estimate_remainning_time_seconds" json:"estimate_remainning_time_seconds"`
	Speedup                    map[string]float32 `bson:"speedup" json:"speedup"`
	Efficiency                 map[string]float32 `bson:"efficiency" json:"efficiency"`
}

// newTrainingJob creates a new training job according to MPIJob representation
func NewTrainingJob(mpijob *kubeflowv1.MPIJob, category string, submitTime time.Time) (*TrainingJob, error) {

	var (
		numProc    int
		minNumProc int
		maxNumProc int
		epochs     int
		priority   int
		err        error
	)

	// Read job config from job spec
	launcherSpec := mpijob.Spec.MPIReplicaSpecs["Launcher"]
	// Iterate over EnVar to get job config
	// There should be only one container in the spec
	env := launcherSpec.Template.Spec.Containers[0].Env
	for i := 0; i < len(env); i++ {
		if env[i].Name == string(types.JobMinNumProc) || env[i].Name == string(types.JobMinNumProcDeprecated) {
			if minNumProc, err = strconv.Atoi(env[i].Value); err != nil {
				return nil, err
			}
		} else if env[i].Name == string(types.JobMaxNumProc) || env[i].Name == string(types.JobMaxNumProcDeprecated) {
			if maxNumProc, err = strconv.Atoi(env[i].Value); err != nil {
				return nil, err
			}
		} else if env[i].Name == string(types.JobNumProc) || env[i].Name == string(types.JobNumProcDeprecated) {
			if numProc, err = strconv.Atoi(env[i].Value); err != nil {
				return nil, err
			}
		} else if env[i].Name == string(types.JobEpochs) {
			if epochs, err = strconv.Atoi(env[i].Value); err != nil {
				return nil, err
			}
		} else if env[i].Name == string(types.JobName) {
			if env[i].Value != mpijob.ObjectMeta.Name {
				return nil, errors.New("environment variable JOB_NAME and mpijob.ObjectMeta.Name missmatched")
			}
		} else if env[i].Name == string(types.JobPriority) {
			if priority, err = strconv.Atoi(env[i].Value); err != nil {
				return nil, err
			}
		}
	}
	// TODO(heyfey): error handling if either "MIN_NUM_PROC", "MAX_NUM_PROC", "EPOCHs" or "JOB_NAME" is missing or invalid
	// TODO(heyfey): currently ignore cases that both "MIN_NP" and "MIN_NUM_PROC" present at the same time

	// if NUM_PROC not specified
	if numProc == 0 {
		numProc = minNumProc
	}

	config := JobConfig{
		NumProc:    numProc,
		MinNumProc: minNumProc,
		MaxNumProc: maxNumProc,
		Epochs:     epochs,
	}

	// Get GPU type from job spec
	workerSpec := mpijob.Spec.MPIReplicaSpecs["Worker"]
	gpuType, ok := workerSpec.Template.Spec.NodeSelector[gpuNameLabel]
	if !ok {
		return nil, errors.New("gpu type not specified")
	}

	// JobInfo would be updated from mongodb by scheduler during resched
	info := NewBaseJobInfo(mpijob.GetName(), category, gpuType)

	t := &TrainingJob{
		Name:       mpijob.ObjectMeta.GetName(),
		Category:   category,
		User:       "heyfey", // TODO(heyfey)
		Kind:       types.JobMPIJob,
		Spec:       mpijob,
		GpuType:    gpuType,
		Priority:   priority,
		Status:     types.JobSubmitted,
		SubmitTime: submitTime,
		FinishTime: types.MaxTime,
		Config:     config,
		Metrics:    NewJobMetrics(),
		Info:       info,
	}
	return t, nil
}

func NewJobMetrics() *JobMetrics {
	m := &JobMetrics{
		RunningDuration:     0,
		WaitingDuration:     0,
		GpuDuration:         0,
		TotalDuration:       0,
		LastRunningDuration: 0,
		LastWaitingDuration: 0,
		LastGpuDuration:     0,
		FirstStartTime:      types.MaxTime,
		LastUpdateTime:      time.Now(),
	}
	return m
}

func NewBaseJobInfo(name string, category string, gpuType string) JobInfo {
	speedup := map[string]float32{"0": 0.0}
	efficiency := map[string]float32{"0": 0.0}

	// assume linear speedup
	for i := 1; i <= maxNumGpu+1; i++ {
		speedup[strconv.Itoa(i)] = float32(i)
		efficiency[strconv.Itoa(i)] = float32(1)
	}

	i := JobInfo{
		Name:                       name,
		Category:                   category,
		GpuType:                    gpuType,
		EstimatedRemainningTimeSec: 0,
		Speedup:                    speedup,
		Efficiency:                 efficiency,
	}
	return i
}
