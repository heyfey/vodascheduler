package service

import (
	"strings"

	"github.com/heyfey/vodascheduler/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ServiceMetrics struct {
	serviceInfoGauge         prometheus.GaugeVec
	jobsCreatedCounter       prometheus.Counter
	jobsDeletedCounter       prometheus.Counter
	createJobDuration        prometheus.Summary
	createJobSuccessDuration prometheus.Summary
	deleteJobDuration        prometheus.Summary
	deleteJobSuccessDuration prometheus.Summary
}

func (s *Service) initServiceMetrics() {
	namespace := strings.Replace(config.Namespace, "-", "_", -1)

	m := ServiceMetrics{
		serviceInfoGauge: *promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "training_service_info",
			Namespace: namespace,
			Help:      "Information about the training service.",
		}, []string{"version", "namespace"}),

		jobsCreatedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "training_service_jobs_created_total",
			Namespace: namespace,
			Help:      "Counts number of training jobs created.",
		}),

		jobsDeletedCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "training_service_jobs_deleted_total",
			Namespace: namespace,
			Help:      "Counts number of training jobs deleted.",
		}),

		createJobDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "training_service_create_job_duration_seconds",
			Namespace: namespace,
			Help:      "A summary of the duration of creating training job.",
		}),

		createJobSuccessDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "training_service_create_job_success_duration_seconds",
			Namespace: namespace,
			Help:      "A summary of the duration of successfully creating training job.",
		}),

		deleteJobDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "training_service_delete_job_duration_seconds",
			Namespace: namespace,
			Help:      "A summary of the duration of deleting training job.",
		}),

		deleteJobSuccessDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "training_service_delete_job_success_duration_seconds",
			Namespace: namespace,
			Help:      "A summary of the duration of successfully deleting training job.",
		}),
	}
	m.serviceInfoGauge.WithLabelValues(config.Version, config.Namespace).Set(1)
	s.Metrics = m
}
