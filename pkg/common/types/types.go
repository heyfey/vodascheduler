// https://pkg.go.dev/github.com/kubeflow/common/pkg/apis/common/v1

package types

// JobConfigType represents the job arguments required by the system
// Users are required to specify these arguments by environment variables in the yaml
// see example at:
// https://github.com/heyfey/celeste/blob/main/examples/yaml/tensorflow2/tensorflow2-keras-mnist-elastic.yaml
type JobConfigType string

const (
	// MIN_NP and MAX_NP are used to specified the minimum and maximum number
	// of processes to run with during the training job
	JobMinNP JobConfigType = "MIN_NP"
	JobMaxNP JobConfigType = "MAX_NP"

	// EPOCHS is used to estimate training time
	JobEpochs JobConfigType = "EPOCHS"
	// JOB_NAME is used to recgonize training job, witch would be set by JobMaster
	JobName JobConfigType = "JOB_NAME"
)

type JobStatusType string

const (
	// JobWaiting means the job has been accepted by the system,
	// but the mpijob has not been started.
	// This includes time before mpijob being scheduled and launched.
	JobWaiting JobStatusType = "Waiting"

	// JobRunning means all sub-resources (e.g. services/pods) of this job
	// have been successfully scheduled and launched.
	// The training is running without error.
	JobRunning JobStatusType = "Running"

	// JobCompleted means the mpijob of this job, reached phase have terminated in success.
	// The training is complete without error.
	JobCompleted JobStatusType = "Completed"

	// JobFailed means one or more sub-resources (e.g. services/pods) of this job
	// reached phase failed.
	// The scheduler would try to restart the training.
	JobFailed JobStatusType = "Failed"
)

// Number of allocated GPUs of each training job
// TODO: Considering rename to JobAllocateResult
// (JobAllocateResult + JobPlacementResult = JobScheduleResult)
type JobScheduleResult map[string]int
