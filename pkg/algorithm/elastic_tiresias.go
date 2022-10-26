package algorithm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/heyfey/vodascheduler/pkg/common/types"
	"k8s.io/klog/v2"
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
}

func NewElasticTiresias(id string) *ElasticTiresias {
	a := &ElasticTiresias{
		algorithm:   "ElasticTiresias",
		schedulerID: id,
	}
	return a
}

func (a *ElasticTiresias) Schedule(jobs ReadyJobs, totalGPU int) (result types.JobScheduleResult) {
	result = make(map[string]int)
	freeGPU := totalGPU
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
		gain[job.Name] = job.Info.Speedup[fmt.Sprint(job.Config.MinNumProc)] / float32(job.Config.MinNumProc)
		result[job.Name] = 0
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
	klog.V(5).InfoS("Started scheduling", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm, "queueNum", TiresiasQueueNum, "thresholds", TiresiasThresholdsSec, "promoteKnob", TiresiasPromoteKnob, "compactionThreshold", ElasticTiresiasCompactionThreshold)

	// allocate the basic portion
	for priority := 0; priority < TiresiasQueueNum; priority++ {
		for _, job := range queues[priority] {
			// if could allocate NP to the job, allocate it
			if freeGPU >= job.Config.NumProc {
				result[job.Name] = job.Config.NumProc
				freeGPU -= job.Config.NumProc
				pendings -= 1
				gain[job.Name] = calNextGain(job.Info.Speedup, result[job.Name])
			}
		}
	}

	// If the number of pending jobs > compaction threshold, trigger the compaction
	// by allocate each job with priority > 1 minGPU instead of NP.
	if pendings > ElasticTiresiasCompactionThreshold {
		klog.V(5).InfoS("Before compaction", "jobs", jobs, "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)
		// compact the jobs with priority >= 1 from NP to minGPU
		for priority := 1; priority < TiresiasQueueNum; priority++ {
			for _, job := range queues[priority] {
				if result[job.Name] != 0 {
					freeGPU += (result[job.Name] - job.Config.MinNumProc) // free GPUs
					result[job.Name] = job.Config.MinNumProc
					gain[job.Name] = calNextGain(job.Info.Speedup, result[job.Name])
				}
			}
		}
		klog.V(5).InfoS("After compaction", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)
	}

	klog.V(5).InfoS("Finished phase one scheduling", "result", result, "freeGPU", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	// remove the jobs that are already sastified or not possible to run
	toRemove := make([]string, 0)
	for _, job := range jobs {
		if result[job.Name] >= job.Config.MaxNumProc || freeGPU < job.Config.MinNumProc {
			toRemove = append(toRemove, job.Name)
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
			return gain[jobs[i].Name] > gain[jobs[j].Name]
		})
		job := jobs[0] // get the job with the most gain
		klog.V(5).InfoS("Found the next most gain job", "jobs", jobs, "freeGpu", freeGPU, "job", job.Name, "gain", gain, "scheduler", a.schedulerID, "algorithm", a.algorithm)
		if gain[job.Name] <= 0 {
			// there won't be any efficiency gain to allocate extra gpu to any job
			klog.V(5).InfoS("No more gains", "jobs", jobs, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm)
			break
		}
		// job should receive at least minGPU if it is scheduled
		if result[job.Name] == 0 {
			if freeGPU >= job.Config.MinNumProc {
				result[job.Name] = job.Config.MinNumProc
				freeGPU -= job.Config.MinNumProc
				gain[job.Name] = calNextGain(job.Info.Speedup, result[job.Name])
			} else {
				jobs = remove(jobs, index(jobs, job.Name))
			}
		} else {
			result[job.Name] += 1
			freeGPU -= 1
			gain[job.Name] = calNextGain(job.Info.Speedup, result[job.Name])
			if result[job.Name] >= job.Config.MaxNumProc {
				jobs = remove(jobs, index(jobs, job.Name))
			}
		}
	}

	// TODO(heyfey): unable to log TiresiasThresholdsSec correctly
	klog.V(4).InfoS("Finished scheduling", "result", result, "freeGpu", freeGPU, "scheduler", a.schedulerID, "algorithm", a.algorithm, "queueNum", TiresiasQueueNum, "thresholds", TiresiasThresholdsSec, "promoteKnob", TiresiasPromoteKnob, "compactionThreshold", ElasticTiresiasCompactionThreshold)

	validateResult(totalGPU, result, oriJobs)
	return result
}

func (a *ElasticTiresias) GetName() string {
	return a.algorithm
}

func (a *ElasticTiresias) NeedJobInfo() bool {
	return true
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
		if jobs[i].Name == name {
			return i
		}
	}
	klog.Flush()
	panic(errors.New("Not found"))
}
