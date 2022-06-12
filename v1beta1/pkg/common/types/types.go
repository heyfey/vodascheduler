package types

import "time"

// JobConfigType represents the job arguments required by voda-scheduler.
// Users are required to specify these arguments by environment variables in the
// yaml. See example at: https://github.com/heyfey/vodascheduler/examples/
type JobConfigType string

const (
	// NUM_PROC, MIN_NUM_PROC and MAX_NUM_PROC are used to specified the
	// requested, minimum and maximum number of processes to run with during the
	// training job.
	JobNumProc    JobConfigType = "NUM_PROC"
	JobMinNumProc JobConfigType = "MIN_NUM_PROC"
	JobMaxNumProc JobConfigType = "MAX_NUM_PROC"

	JobNumProcDeprecated    JobConfigType = "NP"
	JobMinNumProcDeprecated JobConfigType = "MIN_NP"
	JobMaxNumProcDeprecated JobConfigType = "MAX_NP"

	// EPOCHS is used to estimate training time
	JobEpochs JobConfigType = "EPOCHS"
	// JOB_NAME is used to recgonize training job, witch would be set by JobMaster
	JobName JobConfigType = "JOB_NAME"
	// JOB_PRIORITY is used to specify priority of the training job. Valid
	// priority is a non-negative integer. 0 has the highest priority. Default 0.
	JobPriority JobConfigType = "JOB_PRIORITY"
)

type JobStatusType string

const (
	// JobSubmitted means the job has been accepted by the training service, but
	// not accepted by any scheduler yet.
	JobSubmitted JobStatusType = "Submitted"

	// JobWaiting means the job has been accepted by the scheduler, but not
	// being allocated any GPU now.
	JobWaiting JobStatusType = "Waiting"

	// JobRunning means the training job is allocated with at least one GPU.
	JobRunning JobStatusType = "Running"

	JobCompleted JobStatusType = "Completed"
	JobFailed    JobStatusType = "Failed"
	JobCanceled  JobStatusType = "Canceled"
)

type JobKindType string

const (
	JobMPIJob     JobKindType = "MPIJob"
	JobTFJob      JobKindType = "TFJob"
	JobPyTorchJob JobKindType = "PyTorchJob"
)

// Number of allocated GPUs of each training job
// TODO: Considering rename to JobAllocateResult
// (JobAllocateResult + JobPlacementResult = JobScheduleResult)
type JobScheduleResult map[string]int

// https://stackoverflow.com/questions/25065055/what-is-the-maximum-time-time-in-go/32620397#32620397
// MaxTime should not be modify by anyone.
var MaxTime time.Time = time.Unix(1<<63-62135596801, 999999999)
