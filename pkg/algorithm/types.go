/*
https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/core/generic_scheduler.go
https://github.com/microsoft/hivedscheduler/blob/master/pkg/internal/types.go
https://github.com/microsoft/hivedscheduler/blob/master/pkg/algorithm/hived_algorithm.go
*/

package algorithm

import (
	"github.com/heyfey/celeste/pkg/common/trainingjob"
	"github.com/heyfey/celeste/pkg/common/types"
)

type ReadyJobs []trainingjob.TrainingJob

// ScheduleAlgorithm is an interface implemented by things that know how to schedule training jobs
type SchedulerAlgorithm interface {
	Schedule(ReadyJobs) types.JobScheduleResult
	GetName() string
}
