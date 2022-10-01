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
}

func NewAFSL(id string) *AFSL {
	a := &AFSL{
		algorithm:   "AFS-L",
		schedulerID: id,
	}
	return a
}

func (a *AFSL) Schedule(jobs ReadyJobs, totalGPU int) (result types.JobScheduleResult) {
	result = make(map[string]int)
	for _, job := range jobs {
		result[job.Name] = 0
	}
	// keep the original jobs for later validation
	oriJobs := make(ReadyJobs, len(jobs))
	copy(oriJobs, jobs)
	defer validateResult(totalGPU, result, oriJobs)

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].SubmitTime.Before(jobs[j].SubmitTime)
	})

	freeGPU := totalGPU
	for freeGPU > 0 && len(jobs) > 0 {
		job := a.topPriority(jobs, result)
		result[job.Name] += 1
		freeGPU -= 1

		klog.V(5).InfoS("Allocated 1 GPU to the top priority job", "job", job.Name, "result", result, "freeGpu", freeGPU,
			"scheduler", a.schedulerID, "algorithm", a.algorithm)

		if result[job.Name] >= job.Config.MaxNumProc {
			jobs = remove(jobs, index(jobs, job.Name))
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
		if result[j.Name] == 0 && result[jb.Name] == 0 {
			if j.Info.EstimatedRemainningTimeSec >= jb.Info.EstimatedRemainningTimeSec {
				j = jb
			}
		} else {
			lenA := a.jobLength(j, result[j.Name])
			lenB := a.jobLength(jb, result[j.Name])
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
	left := (jb.Info.Speedup[fmt.Sprint(result[jb.Name]+1)] - jb.Info.Speedup[fmt.Sprint(result[jb.Name])]) / jb.Info.Speedup[fmt.Sprint(result[jb.Name]+1)]
	right := (j.Info.Speedup[fmt.Sprint(result[j.Name]+1)] - j.Info.Speedup[fmt.Sprint(result[j.Name])]) / j.Info.Speedup[fmt.Sprint(result[j.Name])]
	return left > right
}
