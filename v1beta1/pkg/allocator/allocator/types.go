package allocator

import "github.com/heyfey/vodascheduler/pkg/algorithm"

type AllocationRequest struct {
	NumGpu        int
	AlgorithmName string
	ReadyJobs     algorithm.ReadyJobs
}
