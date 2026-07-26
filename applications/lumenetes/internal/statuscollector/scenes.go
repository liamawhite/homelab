package statuscollector

import (
	"context"

	"github.com/go-logr/logr"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	sceneInvalidLightCountDesc = prometheus.NewDesc(
		"lumenetes_scene_invalid_light_count", "Number of this scene's declared lights that don't exist or aren't members of its group.",
		[]string{"scene"}, nil,
	)
	sceneLastSyncedDesc = prometheus.NewDesc(
		"lumenetes_scene_last_synced_timestamp_seconds", "Unix timestamp of the last status sync for this scene.",
		[]string{"scene"}, nil,
	)
)

func (c *Collector) collectScenes(ctx context.Context, logger logr.Logger, ch chan<- prometheus.Metric) {
	var list lumenetesv1alpha1.SceneList
	if err := c.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list scenes")
		return
	}

	for _, scene := range list.Items {
		s := scene.Status
		ch <- prometheus.MustNewConstMetric(sceneInvalidLightCountDesc, prometheus.GaugeValue, float64(len(s.InvalidLights)), scene.Name)
		if !s.LastSynced.IsZero() {
			ch <- prometheus.MustNewConstMetric(sceneLastSyncedDesc, prometheus.GaugeValue, float64(s.LastSynced.Unix()), scene.Name)
		}
	}
}
