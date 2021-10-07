package algorithm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/heyfey/vodascheduler/pkg/common/logger"
	"github.com/heyfey/vodascheduler/pkg/common/types"
)

// Implementation of the E-Tiresias scheduling algorithm presented in
// Wu, Yidi, et al. "Elastic Deep Learning in Multi-Tenant GPU Clusters."
// IEEE Transactions on Parallel and Distributed Systems (2021).
// https://ieeexplore.ieee.org/abstract/document/9373916

// Settings described in the original EDL paper.
var (
	// E-Tiresias triggers compaction if the number of pending jobs > the
	// compaction threshold.
	ElasticTiresiasCompactionThreshold = 10
)

type ElasticTiresias struct {
	algorithm   string
	schedulerID string
	totalGPU    int
}

func NewElasticTiresias(totalGPU int, id string) *ElasticTiresias {
	a := &ElasticTiresias{
		algorithm:   "ElasticTiresias",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return a
}

func (a *ElasticTiresias) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	log := logger.GetLogger()
	defer logger.Flush()

	result = make(map[string]int)
	freeGPU := a.totalGPU
	pendings := len(jobs)
	// efficiency gain if allocate 1 more GPU to the job
	gain := make(map[string]float32)

	// keep the original jobs for later validation
	oriJobs := make(ReadyJobs, len(jobs))
	copy(oriJobs, jobs)

	// initiate the queues
	queues := make(map[int]ReadyJobs)
	for priority := 0; priority < TiresiasQueueNum; priority++ {
		queues[priority] = make(ReadyJobs, 0)
	}

	for _, job := range jobs {
		// put jobs into queues depends on its priority
		queues[job.Priority] = append(queues[job.Priority], job)
		// initiate gain; do interpolation because minGPU may not equal to 1
		gain[job.JobName] = job.Info.Speedup[fmt.Sprint(job.Config.MinGPU)] / float32(job.Config.MinGPU)
		result[job.JobName] = 0
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
	log.V(5).Info("Started scheduling", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm, "queueNum", TiresiasQueueNum, "thresholds", TiresiasThresholdsSec, "promoteKnob", TiresiasPromoteKnob, "compactionThreshold", ElasticTiresiasCompactionThreshold)

	// allocate the basic portion
	for priority := 0; priority < TiresiasQueueNum; priority++ {
		for _, job := range queues[priority] {
			// if could allocate NP to the job, allocate it
			if freeGPU >= job.Config.NP {
				result[job.JobName] = job.Config.NP
				freeGPU -= job.Config.NP
				pendings -= 1
				gain[job.JobName] = calNextGain(job.Info.Speedup, result[job.JobName])
			}
		}
	}

	// If the number of pending jobs > compaction threshold, trigger the compaction
	// by allocate each job with priority > 1 minGPU instead of NP.
	if pendings > ElasticTiresiasCompactionThreshold {
		log.V(5).Info("Before compaction", "jobs", jobs, "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)
		// compact the jobs with priority >= 1 from NP to minGPU
		for priority := 1; priority < TiresiasQueueNum; priority++ {
			for _, job := range queues[priority] {
				if result[job.JobName] != 0 {
					freeGPU += (result[job.JobName] - job.Config.MinGPU) // free GPUs
					result[job.JobName] = job.Config.MinGPU
					gain[job.JobName] = calNextGain(job.Info.Speedup, result[job.JobName])
				}
			}
		}
		log.V(5).Info("After compaction", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)
	}

	log.V(5).Info("Finished phase one scheduling", "result", result, "freeGPU", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// remove the jobs that are already sastified or not possible to run
	toRemove := make([]string, 0)
	for _, job := range jobs {
		if result[job.JobName] >= job.Config.MaxGPU || freeGPU < job.Config.MinGPU {
			toRemove = append(toRemove, job.JobName)
		}
	}
	for _, job := range toRemove {
		jobs = remove(jobs, index(jobs, job))
	}

	// Allocate the remainning GPUs to the job with the most efficiency gain.
	// Note that we will gain most efficiency by allocate minGPU to the pending jobs in theory.
	for freeGPU > 0 && len(jobs) > 0 {
		// sort the queue by priority
		sort.SliceStable(jobs, func(i, j int) bool {
			return jobs[i].Priority < jobs[j].Priority
		})
		// sort the queue by gain in descending order
		sort.SliceStable(jobs, func(i, j int) bool {
			return gain[jobs[i].JobName] > gain[jobs[j].JobName]
		})
		job := jobs[0] // get the job with the most gain
		log.V(5).Info("Found the next most gain job", "jobs", jobs, "freeGpu", freeGPU, "job", job.JobName, "gain", gain, "scheduler", a.schedulerID, "algorithm", a.algorithm)
		if gain[job.JobName] <= 0 {
			// there won't be any efficiency gain to allocate extra gpu to any job
			log.V(5).Info("No more gains", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)
			break
		}
		// job should receive at least minGPU if it is scheduled
		if result[job.JobName] == 0 {
			if freeGPU >= job.Config.MinGPU {
				result[job.JobName] = job.Config.MinGPU
				freeGPU -= job.Config.MinGPU
				gain[job.JobName] = calNextGain(job.Info.Speedup, result[job.JobName])
			} else {
				jobs = remove(jobs, index(jobs, job.JobName))
			}
		} else {
			result[job.JobName] += 1
			freeGPU -= 1
			gain[job.JobName] = calNextGain(job.Info.Speedup, result[job.JobName])
			if result[job.JobName] >= job.Config.MaxGPU {
				jobs = remove(jobs, index(jobs, job.JobName))
			}
		}
	}

	// TODO: unable to log TiresiasThresholdsSec correctly
	log.V(4).Info("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm, "queueNum", TiresiasQueueNum, "thresholds", TiresiasThresholdsSec, "promoteKnob", TiresiasPromoteKnob, "compactionThreshold", ElasticTiresiasCompactionThreshold)

	validateResult(a.totalGPU, result, oriJobs)
	return result
}

func (a *ElasticTiresias) GetName() string {
	return a.algorithm
}

// calNextGain returns the efficiency gain if one more worker allocated
func calNextGain(speedup map[string]float32, workers int) float32 {
	return speedup[fmt.Sprint(workers+1)] - speedup[fmt.Sprint(workers)]
}

// remove removes a element by index from the slice (keeping the original order)
// and retruns the new slice.
func remove(slice ReadyJobs, s int) ReadyJobs {
	return append(slice[:s], slice[s+1:]...)
}

// index return index of a training job in the queue, must make sure the
// training job exist, or it will panic.
func index(jobs ReadyJobs, name string) int {
	for i := 0; i < len(jobs); i++ {
		if jobs[i].JobName == name {
			return i
		}
	}
	panic(errors.New("Not found"))
}
