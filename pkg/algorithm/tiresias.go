package algorithm

import (
	"math"
	"sort"

	"github.com/heyfey/celeste/pkg/common/logger"
	"github.com/heyfey/celeste/pkg/common/types"
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
	totalGPU    int
}

func NewTiresias(totalGPU int, id string) *Tiresias {
	a := &Tiresias{
		algorithm:   "Tiresias",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return a
}

func (a *Tiresias) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	log := logger.GetLogger()
	defer logger.Flush()

	result = make(map[string]int)
	freeGPU := a.totalGPU

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
			return queues[priority][i].FirstStarted.Before(queues[priority][j].FirstStarted)
		})
	}

	// TODO: unable to log TiresiasThresholdsSec correctly
	log.V(5).Info("Started scheduling", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm, "queueNum", TiresiasQueueNum, "thresholds", TiresiasThresholdsSec, "promoteKnob", TiresiasPromoteKnob)

	// allocate the basic portion
	for priority := 0; priority < TiresiasQueueNum; priority++ {
		for _, job := range queues[priority] {
			result[job.JobName] = 0
			// if could allocate NP to the job, allocate it
			if freeGPU >= job.Config.NP {
				result[job.JobName] = job.Config.NP
				freeGPU -= job.Config.NP
			}
		}
	}

	// TODO: unable to log TiresiasThresholdsSec correctly
	log.V(4).Info("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm, "queueNum", TiresiasQueueNum, "thresholds", TiresiasThresholdsSec, "promoteKnob", TiresiasPromoteKnob)

	validateResult(a.totalGPU, result, jobs)
	return result
}

func (a *Tiresias) GetName() string {
	return a.algorithm
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
