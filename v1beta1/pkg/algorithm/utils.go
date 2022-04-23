package algorithm

import (
	"errors"

	"github.com/heyfey/vodascheduler/pkg/common/types"
)

func allTrue(sastified map[string]bool) bool {
	for _, s := range sastified {
		if s == false {
			return false
		}
	}
	return true
}

func validateResult(totalGPU int, result types.JobScheduleResult, jobs ReadyJobs) {
	jobMaxGPU := make(map[string]int)
	jobMinGPU := make(map[string]int)
	for _, job := range jobs {
		jobMaxGPU[job.JobName] = job.Config.MaxGPU
		jobMinGPU[job.JobName] = job.Config.MinGPU
	}

	allocatedGPU := 0
	for job, n := range result {
		if n < 0 {
			panic(errors.New("Invalid GPU allocations: can't be negative"))
		}
		if n > 0 && n < jobMinGPU[job] {
			panic(errors.New("Invalid GPU allocations: less than job min gpu"))
		}
		if n > jobMaxGPU[job] {
			panic(errors.New("Invalid GPU allocations: exceeded job max gpu"))
		}
		allocatedGPU += n
	}
	if allocatedGPU > totalGPU {
		panic(errors.New("Invalid GPU allocations: exceeded total GPUs"))
	}
}
