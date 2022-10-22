package placement

import (
	"strings"

	"github.com/heyfey/vodascheduler/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PlacementManagerMetrics struct {
	placementAlgoDuration            prometheus.Summary
	deletedWorkerForMigrationGauge   prometheus.Gauge
	deletedLauncherForMigrationGauge prometheus.Gauge
	crossNodeGauge                   prometheus.Gauge
}

func (pm *PlacementManager) initPlacementManagerMetrics() {
	subsystem := strings.Replace(pm.SchedulerID, "-", "_", -1)
	namespace := strings.Replace(config.Namespace, "-", "_", -1)

	m := PlacementManagerMetrics{
		placementAlgoDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "scheduler_placement_algorithm_duration_seconds",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "A summary of the duration of placement algorithm.",
		}),
		deletedWorkerForMigrationGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "scheduler_placement_workers_migrated",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Number of deleted worker pods for migration in last rescheduling.",
		}),
		deletedLauncherForMigrationGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "scheduler_placement_launchers_deleted",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Number of deleted launcher pods in last rescheduling.",
		}),
		crossNodeGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "scheduler_placement_jobs_cross_node",
			Subsystem: subsystem,
			Namespace: namespace,
			Help:      "Number of job that need cross-node communication.",
		}),
	}

	pm.metrics = m
}
