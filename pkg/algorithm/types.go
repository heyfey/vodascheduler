/*
https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/core/generic_scheduler.go
https://github.com/microsoft/hivedscheduler/blob/master/pkg/internal/types.go
https://github.com/microsoft/hivedscheduler/blob/master/pkg/algorithm/hived_algorithm.go
*/

package algorithm

import (
	"errors"

	"github.com/heyfey/vodascheduler/pkg/common/trainingjob"
	"github.com/heyfey/vodascheduler/pkg/common/types"
)

type ReadyJobs []trainingjob.TrainingJob

// SchedulerAlgorithm is an interface implemented by things that know how to schedule training jobs
type SchedulerAlgorithm interface {
	Schedule(ReadyJobs, int) types.JobScheduleResult
	GetName() string
	// Whether need training job info for scheduling
	NeedJobInfo() bool
}

func NewAlgorithmFactory(algorithm string, schedulerID string) (SchedulerAlgorithm, error) {
	switch algorithm {
	case "FIFO":
		return NewFIFO(schedulerID), nil
	case "ElasticFIFO":
		return NewElasticFIFO(schedulerID), nil
	case "SRJF":
		return NewSRJF(schedulerID), nil
	case "ElasticSRJF":
		return NewElasticSRJF(schedulerID), nil
	case "Tiresias":
		return NewTiresias(schedulerID), nil
	case "ElasticTiresias":
		return NewElasticTiresias(schedulerID), nil
	case "FfDLOptimizer":
		return NewFfDLOptimizer(schedulerID), nil
	case "AFS-L":
		return NewAFSL(schedulerID), nil
	default:
		return nil, errors.New("Not found")
	}
}
