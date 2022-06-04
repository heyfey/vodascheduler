/*
https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/core/generic_scheduler.go
https://github.com/microsoft/hivedscheduler/blob/master/pkg/internal/types.go
https://github.com/microsoft/hivedscheduler/blob/master/pkg/algorithm/hived_algorithm.go
*/

package algorithm

import (
	"github.com/heyfey/vodascheduler/pkg/common/trainingjob"
	"github.com/heyfey/vodascheduler/pkg/common/types"
)

type ReadyJobs []trainingjob.TrainingJob

// ScheduleAlgorithm is an interface implemented by things that know how to schedule training jobs
type SchedulerAlgorithm interface {
	Schedule(ReadyJobs, int) types.JobScheduleResult
	GetName() string
}
