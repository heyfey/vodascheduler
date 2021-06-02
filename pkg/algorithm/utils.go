package algorithm

import (
	"errors"

	"github.com/heyfey/celeste/pkg/common/types"
)

func allTrue(sastified map[string]bool) bool {
	for _, s := range sastified {
		if s == false {
			return false
		}
	}
	return true
}

func validateResult(totalGPU int, result types.JobScheduleResult) {
	allocatedGPU := 0
	for _, n := range result {
		if n < 0 {
			panic(errors.New("Invalid GPU allocations: can't be negative"))
		}
		// TODO: validate if > maxGPU or < minGPU
		allocatedGPU += int(n)
	}
	if allocatedGPU > totalGPU {
		panic(errors.New("Invalid GPU allocations: exceeded total GPUs"))
	}
}
