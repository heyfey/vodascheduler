// https://github.com/kubernetes/kubernetes/blob/master/pkg/scheduler/core/generic_scheduler.go

package algorithm

import (
	"sort"

	"github.com/heyfey/celeste/pkg/common/logger"
	"github.com/heyfey/celeste/pkg/common/types"
)

type ElasticSRJF struct {
	algorithm   string
	schedulerID string
	totalGPU    int
}

func NewElasticSRJF(totalGPU int, id string) *ElasticSRJF {
	a := &ElasticSRJF{
		algorithm:   "ElasticSRJF",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return a
}

func (a *ElasticSRJF) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	log := logger.GetLogger()
	defer logger.Flush()

	result = make(map[string]int)
	sastified := make(map[string]bool)
	freeGPU := a.totalGPU

	// sort the queue by estimated remainning time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Info.EstimatedRemainningTimeSec < jobs[j].Info.EstimatedRemainningTimeSec
	})

	log.V(5).Info("Started scheduling", "jobs", jobs, "freeGPU", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// allocate the basic portion
	for _, job := range jobs {
		result[job.JobName] = 0
		sastified[job.JobName] = false
		// if could allocate minGPU to the job, allocate it
		if freeGPU >= job.Config.MinGPU {
			result[job.JobName] = job.Config.MinGPU
			freeGPU -= job.Config.MinGPU
		} else {
			sastified[job.JobName] = true // unable to allocate minGPU to the job
		}
	}

	log.V(5).Info("Finished phase one scheduling", "result", result, "freeGPU", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

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
	log.V(4).Info("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	validateResult(a.totalGPU, result, jobs)
	return result
}

func (a *ElasticSRJF) GetName() string {
	return a.algorithm
}
