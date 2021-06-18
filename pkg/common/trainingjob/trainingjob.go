package trainingjob

import (
	"errors"
	"strconv"
	"time"

	"github.com/heyfey/celeste/pkg/common/types"
	kubeflowv1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v1"
	// "github.com/prometheus/client_golang/prometheus"
)

// JobMetrics represents metrics of a job.
type JobMetrics struct {
	Name        string `bson:"name" json:"name"`
	LastUpdated time.Time

	RunningTime time.Duration
	WaitingTime time.Duration
	GpuTime     time.Duration
	TotalTime   time.Duration

	LastRunningTime time.Duration
	LastWaitingTime time.Duration
	LastGpuTime     time.Duration

	// Preemption times of the job.
	// Preemptions prometheus.Counter
	// Number of GPUs for the job.
	// workers prometheus.Gauge
}

// TrainingJob represents a single training job in the queue
type TrainingJob struct {
	// ID        bson.ObjectId `bson:"_id" json:"id"`
	JobName       string    `bson:"name" json:"name"`
	JobCollection string    `bson:"collection" json:"collection"`
	Submitted     time.Time `bson:"submitted" json:"submitted"`
	Config        JobConfig `bson:"config" json:"config"`
	Info          JobInfo   `bson:"info" json:"info"`
	Priority      int
}

// JobConfig represents user training configurations specified by user
type JobConfig struct {
	MinGPU int `bson:"min_np" json:"min_np"`
	MaxGPU int `bson:"max_np" json:"max_np"`
	Epochs int `bson:"epochs" json:"epochs"`
}

// JobInfo represents history/estimated information of a training job
type JobInfo struct {
	EstimatedRemainningTimeSec float32
	Efficiency                 map[string]float32
	Speedup                    map[string]float32
}

// newTrainingJob creates a new training job according to MPIJob representation
func NewTrainingJob(mpijob kubeflowv1.MPIJob, collection string, submitted time.Time) (*TrainingJob, error) {

	var (
		minGPU int
		maxGPU int
		epochs int
		err    error
	)

	launcherSpec := mpijob.Spec.MPIReplicaSpecs["Launcher"]
	// Parse through EnVar to get job config
	// There should be only one container in the spec
	env := launcherSpec.Template.Spec.Containers[0].Env
	for i := 0; i < len(env); i++ {
		if env[i].Name == string(types.JobMinNP) {
			if minGPU, err = strconv.Atoi(env[i].Value); err != nil {
				return nil, err
			}
		} else if env[i].Name == string(types.JobMaxNP) {
			if maxGPU, err = strconv.Atoi(env[i].Value); err != nil {
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
		}
	}
	// TODO: error handling if either "MIN_NP", "MAX_NP", "EPOCHs" or "JOB_NAME" is missing

	config := JobConfig{
		MinGPU: minGPU,
		MaxGPU: maxGPU,
		Epochs: epochs,
	}

	// JobInfo would be updated from mongodb by scheduler during resched
	info := JobInfo{}

	t := &TrainingJob{
		JobName:       mpijob.ObjectMeta.Name,
		JobCollection: collection,
		Submitted:     submitted,
		Config:        config,
		Info:          info,
		Priority:      0,
	}
	return t, nil
}

func NewJobMetrics(name string) *JobMetrics {
	// preemptions := prometheus.NewCounter(prometheus.CounterOpts{
	// 	Namespace: name,
	// 	Subsystem: "celeste",
	// 	Name:      "preemptions",
	// 	Help:      "Preemption times of the job.",
	// })
	// prometheus.MustRegister(preemptions)

	// workers := prometheus.NewGauge(prometheus.GaugeOpts{
	// 	Namespace: name,
	// 	Subsystem: "celeste",
	// 	Name:      "workers",
	// 	Help:      "Number of GPUs of the job.",
	// })
	// prometheus.MustRegister(workers)

	m := &JobMetrics{
		Name:            name,
		LastUpdated:     time.Now(),
		RunningTime:     0,
		WaitingTime:     0,
		GpuTime:         0,
		TotalTime:       0,
		LastRunningTime: 0,
		LastWaitingTime: 0,
		LastGpuTime:     0,
		// Preemptions: preemptions,
		// workers:     workers,
	}
	return m
}
