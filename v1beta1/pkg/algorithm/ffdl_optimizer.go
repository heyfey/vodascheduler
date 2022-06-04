package algorithm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/heyfey/vodascheduler/pkg/common/types"
	"k8s.io/klog/v2"
)

// Implementation of the DP algorithm presented by IBM, which aims to maximize
// the overall throughput of the DL platform.
// Saxena, Vaibhav, et al. "Effective elastic scaling of deep learning workloads."
// 2020 28th International Symposium on Modeling, Analysis, and Simulation of
// Computer and Telecommunication Systems (MASCOTS). IEEE, 2020.
//https://ieeexplore.ieee.org/abstract/document/9285954

type FfDLOptimizer struct {
	algorithm   string
	schedulerID string
}

func NewFfDLOptimizer(id string) *FfDLOptimizer {
	a := &FfDLOptimizer{
		algorithm:   "FfDLOptimizer",
		schedulerID: id,
	}
	return a
}

func (a *FfDLOptimizer) Schedule(jobs ReadyJobs, totalGPU int) (result types.JobScheduleResult) {
	result = make(map[string]int)
	if len(jobs) == 0 {
		klog.V(4).InfoS("Finished scheduling", "result", result, "FreeGPU", totalGPU, "scheduler", a.schedulerID,
			"algorithm", a.algorithm)
		return result
	}
	for _, job := range jobs {
		result[job.Name] = 0
	}
	defer validateResult(totalGPU, result, jobs)

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].SubmitTime.Before(jobs[j].SubmitTime)
	})

	K := totalGPU
	// thrim the job queue to ensure it is feasible; note that we make index start from 1
	// We use FIFO policy to trim the job queue, though this part is not
	// explicitly addressed in the original paper. The benefit of FIFO is it
	// avoids starvation.
	feasibleJobs := make(ReadyJobs, 1, K+1)
	copy(feasibleJobs[:0], jobs[:0]) // index 0 will be ignored
	for i := 0; i < K; i++ {
		if i < len(jobs) {
			feasibleJobs = append(feasibleJobs, jobs[i])
		} else {
			break
		}
	}

	// ignored the first entry because index start from 1
	J := len(feasibleJobs) - 1
	// DP table of throughputs; column with index 0 will be discarded
	P := make([][]float32, J+1)
	// the solution table; index 0 will be discarded
	SOL := make([][]int, J+1)
	for i := 0; i <= J; i++ {
		P[i] = make([]float32, K+1)
		SOL[i] = make([]int, K+1)
	}
	// P[j][k] stores the max total throughput if allocated total k GPUs to j jobs
	// SOL[j][k] stores the number of GPUs job j will receive if allocated total k GPUs to j jobs

	// initiate the table
	for i := 0; i <= J; i++ {
		for j := 0; j <= K; j++ {
			if i == 0 {
				P[i][j] = 0 // no jobs mean zero utilization
			} else {
				P[i][j] = -10000 // a large negative number
			}
			SOL[i][j] = 0
		}
	}
	klog.V(5).InfoS("Initiated DP table", "K", K, "J", J, "P", P, "SOL", SOL, "scheduler", a.schedulerID,
		"algorithm", a.algorithm)

	for j := 1; j <= J; j++ {
		for k := 1; k <= K; k++ {
			for g := 1; g <= feasibleJobs[j].Config.MaxNumProc; g++ {
				if k-g >= 0 {
					// calculate the total throughput of relocating g GPUs from
					// previous (j-1) jobs to job j
					p := feasibleJobs[j].Info.Speedup[fmt.Sprint(g)] + P[j-1][k-g]
					if p > P[j][k] { // beter utilization found
						P[j][k] = p
						SOL[j][k] = g
					}
				}
			}
		}
	}
	klog.V(5).InfoS("Finished DP", "K", K, "J", J, "P", P, "SOL", SOL, "scheduler", a.schedulerID,
		"algorithm", a.algorithm)

	if P[J][K] <= 0 {
		err := errors.New("infeasible")
		klog.ErrorS(err, "Failed in scheduling algorithm", "scheduler", a.schedulerID, "algorithm", a.algorithm)
		klog.Flush()
		panic(err)
	}

	j := J
	k := K
	for j > 0 {
		klog.V(5).InfoS("Allocating...", "j", j, "k", k, "SOL", SOL[j][k], "job", feasibleJobs[j].Name,
			"speedup", feasibleJobs[j].Info.Speedup, "scheduler", a.schedulerID, "algorithm", a.algorithm)

		result[feasibleJobs[j].Name] = SOL[j][k]
		k -= SOL[j][k]
		j -= 1
	}
	klog.V(4).InfoS("Finished scheduling", "result", result, "FreeGPU", k, "scheduler", a.schedulerID,
		"algorithm", a.algorithm)

	return result
}

func (a *FfDLOptimizer) GetName() string {
	return a.algorithm
}
