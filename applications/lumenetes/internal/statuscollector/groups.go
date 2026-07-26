package statuscollector

import (
	"context"

	"github.com/go-logr/logr"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	groupLightCountDesc = prometheus.NewDesc(
		"lumenetes_group_light_count", "Number of lights declared as members of this group.",
		[]string{"group"}, nil,
	)
	groupMissingLightCountDesc = prometheus.NewDesc(
		"lumenetes_group_missing_light_count", "Number of this group's declared member lights that don't exist as Light CRs.",
		[]string{"group"}, nil,
	)
	groupActiveSceneErrorDesc = prometheus.NewDesc(
		"lumenetes_group_active_scene_error", "Whether this group's ActiveScene failed to enact (1) or not (0).",
		[]string{"group"}, nil,
	)
)

func (c *Collector) collectGroups(ctx context.Context, logger logr.Logger, ch chan<- prometheus.Metric) {
	var list lumenetesv1alpha1.GroupList
	if err := c.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list groups")
		return
	}

	for _, group := range list.Items {
		s := group.Status
		ch <- prometheus.MustNewConstMetric(groupLightCountDesc, prometheus.GaugeValue, float64(s.LightCount), group.Name)
		ch <- prometheus.MustNewConstMetric(groupMissingLightCountDesc, prometheus.GaugeValue, float64(len(s.MissingLights)), group.Name)
		ch <- prometheus.MustNewConstMetric(groupActiveSceneErrorDesc, prometheus.GaugeValue, boolToFloat(s.ActiveSceneError != ""), group.Name)
	}
}
