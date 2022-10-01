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
}

func NewFIFO(id string) *FIFO {
	a := &FIFO{
		algorithm:   "FIFO",
		schedulerID: id,
	}
	return a
}

func (a *FIFO) Schedule(jobs ReadyJobs, totalGPU int) (result types.JobScheduleResult) {
	result = make(map[string]int)
	freeGPU := totalGPU

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].SubmitTime.Before(jobs[j].SubmitTime)
	})

	klog.V(5).InfoS("Started scheduling", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID,
		"algorithm", a.algorithm)

	// allocate the basic portion
	for _, job := range jobs {
		result[job.Name] = 0
		// if could allocate minGPU to the job, allocate it
		if freeGPU >= job.Config.MinNumProc {
			result[job.Name] = job.Config.MinNumProc
			freeGPU -= job.Config.MinNumProc
		}
	}

	klog.V(4).InfoS("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID,
		"algorithm", a.algorithm)

	validateResult(totalGPU, result, jobs)
	return result
}

func (a *FIFO) GetName() string {
	return a.algorithm
}
