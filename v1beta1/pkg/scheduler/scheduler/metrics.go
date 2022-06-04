package scheduler

import (
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type SchedulerMetrics struct {
	JobsCreatedCounter   prometheus.Counter
	JobsDeletedCounter   prometheus.Counter
	jobsCompletedCounter prometheus.Counter
	jobsFailedCounter    prometheus.Counter
	reschedCounter       prometheus.Counter
	reschedDuration      prometheus.Summary
	reschedAlgoDuration  prometheus.Summary
	// fetchDBDuration =
	waitingJobsGaugeFunc prometheus.GaugeFunc
	GPUInUseGaugeFunc    prometheus.GaugeFunc
}

func (s *Scheduler) initSchedulerMetrics() {
	m := SchedulerMetrics{
		JobsCreatedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name: "voda_scheduler_jobs_created_total",
			Help: "Counts number of training jobs created",
		}),
		JobsDeletedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name: "voda_scheduler_jobs_deleted_total",
			Help: "Counts number of training jobs deleted",
		}),
		jobsCompletedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name: "voda_scheduler_jobs_completed_total",
			Help: "Counts number of training jobs completed",
		}),
		jobsFailedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name: "voda_scheduler_jobs_failed_total",
			Help: "Counts number of training jobs failed",
		}),
		reschedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name: "voda_scheduler_resched_total",
			Help: "Counts number of rescheduling",
		}),
		reschedDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name: "voda_scheduler_resched_duration_seconds",
			Help: "A summary of the duration of rescheduling",
		}),
		reschedAlgoDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name: "voda_scheduler_resched_algorithm_duration_seconds",
			Help: "A summary of the duration of rescheduling algorithm",
		}),
		waitingJobsGaugeFunc: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "voda_scheduler_jobs_waiting",
			Help: "Number of waiting jobs",
		},
			s.getNumWaitingJobs,
		),
		GPUInUseGaugeFunc: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "voda_scheduler_gpus_inuse",
			Help: "Number of GPUs in use",
		},
			s.getNumGpuInUse,
		),
	}

	s.Metrics = m
}

// getNumWaitingJobs calculates the number of waiting jobs.
// It is a questionable design because of 1. linear time complexity that may not
// necessary 2. it aquires lock, that may harm performence.
// Better use Gauge instead of GaugeFunc.
func (s *Scheduler) getNumWaitingJobs() float64 {
	s.SchedulerLock.RLock()
	defer s.SchedulerLock.RUnlock()

	count := 0
	for _, job := range s.ReadyJobsMap {
		if job.Status == types.JobWaiting {
			count += 1
		}
	}
	return float64(count)
}

// getNumGpuInUse calculates the number of GPUs in use.
// It is a questionable design because of 1. linear time complexity that may not
// necessary 2. it aquires lock, that may harm performence.
// Better use Gauge instead of GaugeFunc.
func (s *Scheduler) getNumGpuInUse() float64 {
	s.SchedulerLock.RLock()
	defer s.SchedulerLock.RUnlock()

	count := 0
	for _, num := range s.JobNumGPU {
		count += num
	}
	return float64(count)
}
