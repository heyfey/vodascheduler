package scheduler

import (
	"strings"

	"github.com/heyfey/vodascheduler/config"
	"github.com/heyfey/vodascheduler/pkg/common/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type SchedulerMetrics struct {
	schedulerInfoGauge        prometheus.GaugeVec
	jobsCreatedCounter        prometheus.Counter
	jobsDeletedCounter        prometheus.Counter
	jobsCompletedCounter      prometheus.Counter
	jobsFailedCounter         prometheus.Counter
	reschedCounter            prometheus.Counter
	reschedDuration           prometheus.Summary
	reschedAllocationDuration prometheus.Summary
	// fetchDBDuration =
	readyJobsGaugeFunc   prometheus.GaugeFunc
	waitingJobsGaugeFunc prometheus.GaugeFunc
	runningJobsGaugeFunc prometheus.GaugeFunc
	gpuTotalGaugeFunc    prometheus.GaugeFunc
	gpuInUseGaugeFunc    prometheus.GaugeFunc
}

func (s *Scheduler) initSchedulerMetrics() {
	subsystem := strings.Replace(s.SchedulerID, "-", "_", -1)
	namespace := strings.Replace(config.Namespace, "-", "_", -1)

	m := SchedulerMetrics{
		schedulerInfoGauge: *promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "scheduler_info",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Information about the scheduler.",
		}, []string{"version", "namespace", "scheduler"}),

		jobsCreatedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "scheduler_jobs_created_total",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Counts number of training jobs created.",
		}),
		jobsDeletedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "scheduler_jobs_deleted_total",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Counts number of training jobs deleted.",
		}),
		jobsCompletedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "scheduler_jobs_completed_total",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Counts number of training jobs completed.",
		}),
		jobsFailedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "scheduler_jobs_failed_total",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Counts number of training jobs failed.",
		}),
		reschedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "scheduler_resched_total",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Counts number of rescheduling.",
		}),
		reschedDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "scheduler_resched_duration_seconds",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "A summary of the duration of rescheduling.",
		}),
		reschedAllocationDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "scheduler_resched_allocator_duration_seconds",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "A summary of the duration of getting scheduling result from resource allocator.",
		}),
		readyJobsGaugeFunc: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "scheduler_jobs_ready",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Number of ready jobs.",
		},
			s.getNumReadyJobs,
		),
		waitingJobsGaugeFunc: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "scheduler_jobs_waiting",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Number of waiting jobs.",
		},
			s.getNumWaitingJobs,
		),
		runningJobsGaugeFunc: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "scheduler_jobs_running",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Number of running jobs.",
		},
			s.getNumRunningJobs,
		),
		gpuTotalGaugeFunc: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "scheduler_gpus",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Number of schedulable GPUs.",
		},
			s.getTotalGpus,
		),
		gpuInUseGaugeFunc: promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "scheduler_gpus_inuse",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Number of GPUs in use.",
		},
			s.getNumGpuInUse,
		),
	}
	m.schedulerInfoGauge.WithLabelValues(config.Version, config.Namespace, s.SchedulerID).Set(1)
	s.Metrics = m
}

func (s *Scheduler) getTotalGpus() float64 {
	return float64(s.TotalGpus)
}

// getNumReadyJobs calculates the number of ready (waiting or running) jobs.
// It is a questionable design because of 1. linear time complexity that may not
// necessary 2. it aquires lock, that may harm performence.
// Better use Gauge instead of GaugeFunc.
func (s *Scheduler) getNumReadyJobs() float64 {
	s.SchedulerLock.RLock()
	defer s.SchedulerLock.RUnlock()

	count := 0
	for _, job := range s.ReadyJobsMap {
		if job.Status == types.JobWaiting || job.Status == types.JobRunning {
			count += 1
		}
	}
	return float64(count)
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

// getNumRunningJobs calculates the number of running jobs.
// It is a questionable design because of 1. linear time complexity that may not
// necessary 2. it aquires lock, that may harm performence.
// Better use Gauge instead of GaugeFunc.
func (s *Scheduler) getNumRunningJobs() float64 {
	s.SchedulerLock.RLock()
	defer s.SchedulerLock.RUnlock()

	count := 0
	for _, job := range s.ReadyJobsMap {
		if job.Status == types.JobRunning {
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
