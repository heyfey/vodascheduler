package algorithm

import (
	"math"
	"sort"

	"github.com/heyfey/vodascheduler/pkg/common/types"
	"k8s.io/klog/v2"
)

// Implementation of the Tiresias-L scheduling algorithm presented in
// Gu, Juncheng, et al. "Tiresias: A {GPU} cluster manager for distributed deep learning."
// 16th {USENIX} Symposium on Networked Systems Design and Implementation ({NSDI} 19). 2019.
// https://www.usenix.org/conference/nsdi19/presentation/gu

// Settings described in the original Tiresias paper.
var (
	// Tiresias matains a number of logical queues instead of using a continuous
	// priority spectrum, and demote the job to the next queue when its GPU time
	// crosses queue threshold.
	// TiresiasQueueNum of 2 means priority could be value of {0, 1}, and
	// priority 0 has the highest priority.
	TiresiasQueueNum = 2

	// Threshold of each queue (secs).
	// map{priority: threshold}
	TiresiasThresholdsSec = map[int]float64{
		0: 3600, // 1 hour
		1: math.Inf(1),
	}

	// Tiresias defines the STARVELIMIT as promoteKnob * last execution time.
	// training job will be promoted th the highest priority queue if it has been
	// waiting longer than the STARVELIMIT.
	TiresiasPromoteKnob = 8
)

type Tiresias struct {
	algorithm   string
	schedulerID string
}

func NewTiresias(id string) *Tiresias {
	a := &Tiresias{
		algorithm:   "Tiresias",
		schedulerID: id,
	}
	return a
}

func (a *Tiresias) Schedule(jobs ReadyJobs, totalGPU int) (result types.JobScheduleResult) {
	result = make(map[string]int)
	freeGPU := totalGPU

	// initiate the queues
	queues := make(map[int]ReadyJobs)
	for priority := 0; priority < TiresiasQueueNum; priority++ {
		queues[priority] = make(ReadyJobs, 0)
	}

	// put jobs into queues depends on its priority
	for _, job := range jobs {
		queues[job.Priority] = append(queues[job.Priority], job)
	}

	// Sort each queue by first started time if no distribution provided.
	// FIFO ordering on submission time instead of start time may lead to
	// unnecessary preemptions.
	for priority := 0; priority < TiresiasQueueNum; priority++ {
		sort.SliceStable(queues[priority], func(i, j int) bool {
			return queues[priority][i].Metrics.FirstStartTime.Before(queues[priority][j].Metrics.FirstStartTime)
		})
	}

	// TODO(heyfey): unable to log TiresiasThresholdsSec correctly
	klog.V(5).InfoS("Started scheduling", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID,
		"algorithm", a.algorithm, "queueNum", TiresiasQueueNum, "thresholds", TiresiasThresholdsSec,
		"promoteKnob", TiresiasPromoteKnob)

	// allocate the basic portion
	for priority := 0; priority < TiresiasQueueNum; priority++ {
		for _, job := range queues[priority] {
			result[job.Name] = 0
			// if could allocate NP to the job, allocate it
			if freeGPU >= job.Config.NumProc {
				result[job.Name] = job.Config.NumProc
				freeGPU -= job.Config.NumProc
			}
		}
	}

	// TODO: unable to log TiresiasThresholdsSec correctly
	klog.V(4).InfoS("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID,
		"algorithm", a.algorithm, "queueNum", TiresiasQueueNum, "thresholds", TiresiasThresholdsSec,
		"promoteKnob", TiresiasPromoteKnob)

	validateResult(totalGPU, result, jobs)
	return result
}

func (a *Tiresias) GetName() string {
	return a.algorithm
}

func (a *Tiresias) NeedJobInfo() bool {
	return false
}

func TiresiasDemotePriority(priority int) int {
	if priority < TiresiasQueueNum-1 {
		return priority + 1
	} else {
		return priority
	}
}

func TiresiasPromotePriority(priority int) int {
	return 0
}
