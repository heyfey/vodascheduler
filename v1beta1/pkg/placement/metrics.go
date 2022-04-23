package placement

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PlacementManagerMetrics struct {
	placementAlgoDuration prometheus.Summary
}

func (pm *PlacementManager) initPlacementManagerMetrics() PlacementManagerMetrics {
	m := PlacementManagerMetrics{
		placementAlgoDuration: promauto.NewSummary(prometheus.SummaryOpts{
			Name: "voda_scheduler_placement_algorithm_duration_seconds",
			Help: "A summary of the duration of placement algorithm",
		}),
	}

	return m
}
