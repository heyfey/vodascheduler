package algorithm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/heyfey/celeste/pkg/common/logger"
	"github.com/heyfey/celeste/pkg/common/types"
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
	totalGPU    int
}

func NewFfDLOptimizer(totalGPU int, id string) *FfDLOptimizer {
	a := &FfDLOptimizer{
		algorithm:   "FfDLOptimizer",
		totalGPU:    totalGPU,
		schedulerID: id,
	}
	return a
}

func (a *FfDLOptimizer) Schedule(jobs ReadyJobs) (result types.JobScheduleResult) {
	log := logger.GetLogger()
	defer logger.Flush()

	result = make(map[string]int)
	if len(jobs) == 0 {
		log.V(4).Info("Finished scheduling", "result", result, "FreeGPU", a.totalGPU, "scheduler", a.schedulerID,
			"algorithm", a.algorithm)
		return result
	}
	for _, job := range jobs {
		result[job.JobName] = 0
	}
	defer validateResult(a.totalGPU, result, jobs)

	// sort the queue by submitted time
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Submitted.Before(jobs[j].Submitted)
	})

	K := a.totalGPU
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
	log.V(5).Info("DP initiated", "K", K, "J", J, "P", P, "SOL", SOL, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	for j := 1; j <= J; j++ {
		for k := 1; k <= K; k++ {
			for g := 1; g <= feasibleJobs[j].Config.MaxGPU; g++ {
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
	log.V(5).Info("Finished DP", "K", K, "J", J, "P", P, "SOL", SOL, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	if P[J][K] <= 0 {
		err := errors.New("infeasible")
		log.Error(err, "Failed in scheduling algorithm", "scheduler", a.schedulerID, "algorithm", a.algorithm)
		logger.Flush()
		panic(err)
	}

	j := J
	k := K
	for j > 0 {
		log.V(5).Info("allocating...", "j", j, "k", k, "SOL", SOL[j][k], "job", feasibleJobs[j].JobName,
			"speedup", feasibleJobs[j].Info.Speedup, "scheduler", a.schedulerID, "algorithm", a.algorithm)

		result[feasibleJobs[j].JobName] = SOL[j][k]
		k -= SOL[j][k]
		j -= 1
	}
	log.V(4).Info("Finished scheduling", "result", result, "FreeGPU", k, "scheduler", a.schedulerID, "algorithm", a.algorithm)

	return result
}

func (a *FfDLOptimizer) GetName() string {
	return a.algorithm
}
