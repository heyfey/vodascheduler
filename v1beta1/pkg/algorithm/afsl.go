package algorithm

import (
	"fmt"
	"math"
	"sort"

	"github.com/heyfey/vodascheduler/pkg/common/trainingjob"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"k8s.io/klog/v2"
)

// Implementation of the AFS-L scheduling algorithm presented in
// Hwang, Changho, et al. "Elastic Resource Sharing for Distributed Deep Learning."
// 18th {USENIX} Symposium on Networked Systems Design and Implementation ({NSDI} 21). 2021.
// https://www.usenix.org/conference/nsdi21/presentation/hwang

type AFSL struct {
	algorithm   string
	schedulerID string
	totalGPU    int
}

func NewAFSL(totalGPU int, id string) *AFSL {
	a := &AFSL{
		algorithm:   "AFS-L",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return a
}

func (a *AFSL) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	result = make(map[string]int)
	for _, job := range jobs {
		result[job.JobName] = 0
	}
	// keep the original jobs for later validation
	oriJobs := make(ReadyJobs, len(jobs))
	copy(oriJobs, jobs)
	defer validateResult(a.totalGPU, result, oriJobs)

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Submitted.Before(jobs[j].Submitted)
	})

	freeGPU := a.totalGPU
	for freeGPU > 0 && len(jobs) > 0 {
		job := a.topPriority(jobs, result)
		result[job.JobName] += 1
		freeGPU -= 1

		klog.V(5).InfoS("Allocated 1 GPU to the top priority job", "job", job.JobName, "result", result, "freeGpu", freeGPU,
			"scheduler", a.schedulerID, "algorithm", a.algorithm)

		if result[job.JobName] >= job.Config.MaxGPU {
			jobs = remove(jobs, index(jobs, job.JobName))
		}
	}

	klog.V(4).InfoS("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)
	return result
}

func (a *AFSL) GetName() string {
	return a.algorithm
}

func (a *AFSL) topPriority(jobs ReadyJobs, result types.JobScheduleResult) trainingjob.TrainingJob {
	j := jobs[0]
	for i := 1; i < len(jobs); i++ {
		jb := jobs[i]
		if result[j.JobName] == 0 && result[jb.JobName] == 0 {
			if j.Info.EstimatedRemainningTimeSec >= jb.Info.EstimatedRemainningTimeSec {
				j = jb
			}
		} else {
			lenA := a.jobLength(j, result[j.JobName])
			lenB := a.jobLength(jb, result[j.JobName])
			if lenA >= lenB {
				j, jb = jb, j // let j be the shorter job and jb be the longer job
			}
			if a.evaluate(j, jb, result) {
				j = jb
			}
		}
	}
	return j
}

func (a *AFSL) jobLength(job trainingjob.TrainingJob, workers int) float64 {
	if workers == 0 {
		return math.Inf(1)
	} else {
		return float64(job.Info.EstimatedRemainningTimeSec / job.Info.Speedup[fmt.Sprint(workers)])
	}
}

func (a *AFSL) evaluate(j trainingjob.TrainingJob, jb trainingjob.TrainingJob, result types.JobScheduleResult) bool {
	left := (jb.Info.Speedup[fmt.Sprint(result[jb.JobName]+1)] - jb.Info.Speedup[fmt.Sprint(result[jb.JobName])]) / jb.Info.Speedup[fmt.Sprint(result[jb.JobName]+1)]
	right := (j.Info.Speedup[fmt.Sprint(result[j.JobName]+1)] - j.Info.Speedup[fmt.Sprint(result[j.JobName])]) / j.Info.Speedup[fmt.Sprint(result[j.JobName])]
	return left > right
}
