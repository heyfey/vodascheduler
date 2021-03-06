// https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/core/generic_scheduler.go

package algorithm

import (
	"sort"

	"github.com/heyfey/vodascheduler/pkg/common/types"
	"k8s.io/klog/v2"
)

type FIFO struct {
	algorithm   string
	schedulerID string
	totalGPU    int
}

func NewFIFO(totalGPU int, id string) *FIFO {
	a := &FIFO{
		algorithm:   "FIFO",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return a
}

func (a *FIFO) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	result = make(map[string]int)
	freeGPU := a.totalGPU

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Submitted.Before(jobs[j].Submitted)
	})

	klog.V(5).InfoS("Started scheduling", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// allocate the basic portion
	for _, job := range jobs {
		result[job.JobName] = 0
		// if could allocate minGPU to the job, allocate it
		if freeGPU >= job.Config.MinGPU {
			result[job.JobName] = job.Config.MinGPU
			freeGPU -= job.Config.MinGPU
		}
	}

	klog.V(4).InfoS("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	validateResult(a.totalGPU, result, jobs)
	return result
}

func (a *FIFO) GetName() string {
	return a.algorithm
}
