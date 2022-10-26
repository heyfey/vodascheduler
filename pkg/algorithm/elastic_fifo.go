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
}

func NewElasticFIFO(id string) *ElasticFIFO {
	a := &ElasticFIFO{
		algorithm:   "ElasticFIFO",
		schedulerID: id,
	}
	return a
}

func (a *ElasticFIFO) Schedule(jobs ReadyJobs, totalGPU int) (result types.JobScheduleResult) {
	result = make(map[string]int)
	sastified := make(map[string]bool)
	freeGPU := totalGPU

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].SubmitTime.Before(jobs[j].SubmitTime)
	})

	klog.V(5).InfoS("Started scheduling", "jobs", jobs, "freeGPU", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// allocate the basic portion
	for _, job := range jobs {
		result[job.Name] = 0
		sastified[job.Name] = false
		// if could allocate minGPU to the job, allocate it
		if freeGPU >= job.Config.MinNumProc {
			result[job.Name] = job.Config.MinNumProc
			freeGPU -= job.Config.MinNumProc
			if result[job.Name] == job.Config.MaxNumProc {
				sastified[job.Name] = true
			}
		} else {
			sastified[job.Name] = true // unable to allocate minGPU to the job
		}
	}

	klog.V(5).InfoS("Finished phase one scheduling", "result", result, "freeGPU", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// allocate the remainning GPUs if there are any
	// TODO: don't allocate more GPUs to a job if there is no speedup
	for freeGPU > 0 && !allTrue(sastified) {
		for _, job := range jobs {
			if result[job.Name] < job.Config.MaxNumProc || !sastified[job.Name] {
				result[job.Name] += 1
				freeGPU -= 1
				if result[job.Name] == job.Config.MaxNumProc {
					sastified[job.Name] = true
				}
				if freeGPU == 0 {
					break
				}
			}
		}
	}
	klog.V(4).InfoS("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	validateResult(totalGPU, result, jobs)
	return result
}

func (a *ElasticFIFO) GetName() string {
	return a.algorithm
}

func (a *ElasticFIFO) NeedJobInfo() bool {
	return false
}
