package allocator

import "github.com/heyfey/vodascheduler/pkg/algorithm"

type AllocationRequest struct {
	SchedulerID   string
	NumGpu        int
	AlgorithmName string
	ReadyJobs     algorithm.ReadyJobs
}
