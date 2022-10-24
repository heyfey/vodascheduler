package allocator

import (
	"strings"

	"github.com/heyfey/vodascheduler/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ResourceAllocatorMetrics struct {
	allocatorInfoGauge          prometheus.GaugeVec
	accessDBDuration            prometheus.Summary
	numReadyJobs                prometheus.Summary
	numGpus                     prometheus.Summary
	schedulingAlgorithmDuration prometheus.Summary

	// Metrics that are partitioned by scheduling algorithm
	numReadyJobsLabeled                prometheus.SummaryVec
	numGpusLabeled                     prometheus.SummaryVec
	schedulingAlgorithmDurationLabeled prometheus.SummaryVec
}

func (ra *ResourceAllocator) initResourceAllocatorMetrics() {
	namespace := strings.Replace(config.Namespace, "-", "_", -1)

	m := ResourceAllocatorMetrics{
		allocatorInfoGauge: *promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "resource_allocator_info",
			Namespace: namespace,
			Help:      "Information about the resource allocator.",
		}, []string{"version", "namespace"}),

		accessDBDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "resource_allocator_database_duration_seconds",
			Namespace: namespace,
			Help:      "A summary of the duration of fetching information from database.",
		}),

		numReadyJobs: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "resource_allocator_num_ready_jobs",
			Namespace: namespace,
			Help:      "A summary of the number of ready jobs of the request.",
		}),

		numGpus: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "resource_allocator_num_gpus",
			Namespace: namespace,
			Help:      "A summary of the number of GPUs of the request.",
		}),

		schedulingAlgorithmDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "resource_allocator_scheduling_algorithm_duration_seconds",
			Namespace: namespace,
			Help:      "A summary of the duration of scheduling algorithm.",
		}),

		// Metrics that are partitioned by scheduling algorithm
		numReadyJobsLabeled: *promauto.NewSummaryVec(prometheus.SummaryOpts{
			Name:      "resource_allocator_labeled_num_ready_jobs",
			Namespace: namespace,
			Help:      "A summary of the number of ready jobs of the request.",
		}, []string{"algorithm"}),

		numGpusLabeled: *promauto.NewSummaryVec(prometheus.SummaryOpts{
			Name:      "resource_allocator_labeled_num_gpus",
			Namespace: namespace,
			Help:      "A summary of the number of GPUs of the request.",
		}, []string{"algorithm"}),

		schedulingAlgorithmDurationLabeled: *promauto.NewSummaryVec(prometheus.SummaryOpts{
			Name:      "resource_allocator_labeled_scheduling_algorithm_duration_seconds",
			Namespace: namespace,
			Help:      "A summary of the duration of scheduling algorithm.",
		}, []string{"algorithm"}),
	}

	m.allocatorInfoGauge.WithLabelValues(config.Version, config.Namespace).Set(1)
	ra.Metrics = m
}
