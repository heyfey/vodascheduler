// https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/core/generic_scheduler.go

package algorithm

import (
	"sort"

	"github.com/heyfey/celeste/pkg/common/logger"
	"github.com/heyfey/celeste/pkg/common/types"
)

type SRJF struct {
	algorithm   string
	schedulerID string
	totalGPU    int
}

func NewSRJF(totalGPU int, id string) *SRJF {
	a := &SRJF{
		algorithm:   "SRJF",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return a
}

func (a *SRJF) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	log := logger.GetLogger()
	defer logger.Flush()

	result = make(map[string]int)
	freeGPU := a.totalGPU

	// sort the queue by estimated remainning time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Info.EstimatedRemainningTimeSec < jobs[j].Info.EstimatedRemainningTimeSec
	})

	log.V(5).Info("Started scheduling", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// allocate the basic portion
	for _, job := range jobs {
		result[job.JobName] = 0
		// if could allocate minGPU to the job, allocate it
		if freeGPU >= job.Config.MinGPU {
			result[job.JobName] = job.Config.MinGPU
			freeGPU -= job.Config.MinGPU
		}
	}

	log.V(4).Info("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	validateResult(a.totalGPU, result, jobs)
	return result
}

func (a *SRJF) GetName() string {
	return a.algorithm
}