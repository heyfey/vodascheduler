// https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/core/generic_scheduler.go

package algorithm

import (
	"sort"

	"github.com/heyfey/vodascheduler/pkg/common/types"
	"k8s.io/klog/v2"
)

type ElasticFIFO struct {
	algorithm   string
	schedulerID string
	totalGPU    int
}

func NewElasticFIFO(totalGPU int, id string) *ElasticFIFO {
	a := &ElasticFIFO{
		algorithm:   "ElasticFIFO",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return a
}

func (a *ElasticFIFO) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	result = make(map[string]int)
	sastified := make(map[string]bool)
	freeGPU := a.totalGPU

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Submitted.Before(jobs[j].Submitted)
	})

	klog.V(5).InfoS("Started scheduling", "jobs", jobs, "freeGPU", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// allocate the basic portion
	for _, job := range jobs {
		result[job.JobName] = 0
		sastified[job.JobName] = false
		// if could allocate minGPU to the job, allocate it
		if freeGPU >= job.Config.MinGPU {
			result[job.JobName] = job.Config.MinGPU
			freeGPU -= job.Config.MinGPU
			if result[job.JobName] == job.Config.MaxGPU {
				sastified[job.JobName] = true
			}
		} else {
			sastified[job.JobName] = true // unable to allocate minGPU to the job
		}
	}

	klog.V(5).InfoS("Finished phase one scheduling", "result", result, "freeGPU", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// allocate the remainning GPUs if there are any
	// TODO: don't allocate more GPUs to a job if there is no speedup
	for freeGPU > 0 && !allTrue(sastified) {
		for _, job := range jobs {
			if result[job.JobName] < job.Config.MaxGPU || !sastified[job.JobName] {
				result[job.JobName] += 1
				freeGPU -= 1
				if result[job.JobName] == job.Config.MaxGPU {
					sastified[job.JobName] = true
				}
				if freeGPU == 0 {
					break
				}
			}
		}
	}
	klog.V(4).InfoS("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	validateResult(a.totalGPU, result, jobs)
	return result
}

func (a *ElasticFIFO) GetName() string {
	return a.algorithm
}
