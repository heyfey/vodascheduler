// https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/core/generic_scheduler.go

package algorithm

import (
	"sort"

	"github.com/heyfey/celeste/pkg/common/logger"
	"github.com/heyfey/celeste/pkg/common/types"
)

type FIFO struct {
	algorithm   string
	schedulerID string
	totalGPU    int
}

func NewFIFO(totalGPU int, id string) *FIFO {
	f := &FIFO{
		algorithm:   "FIFO",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return f
}

func (f *FIFO) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	log := logger.GetLogger()
	defer logger.Flush()

	result = make(map[string]int)
	freeGPU := f.totalGPU

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Submitted.Before(jobs[j].Submitted)
	})

	log.V(5).Info("Started scheduling", "jobs", jobs, "freeGpu", freeGPU, "scheduler", f.schedulerID, "algorithm", f.algorithm)

	// allocate the basic portion
	for _, job := range jobs {
		result[job.JobName] = 0
		// if could allocate minGPU to the job, allocate it
		if freeGPU >= job.Config.MinGPU {
			result[job.JobName] = job.Config.MinGPU
			freeGPU -= job.Config.MinGPU
		}
	}

	log.V(4).Info("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", f.schedulerID, "algorithm", f.algorithm)

	validateResult(f.totalGPU, result)
	return result
}