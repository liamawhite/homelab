package statuscollector

import (
	"context"

	"github.com/go-logr/logr"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	bridgeReachableDesc = prometheus.NewDesc(
		"lumenetes_bridge_reachable", "Whether the bridge was reachable as of the last hub-controller SSDP discovery round.",
		[]string{"bridge"}, nil,
	)
	bridgeLastResolvedDesc = prometheus.NewDesc(
		"lumenetes_bridge_last_resolved_timestamp_seconds", "Unix timestamp this bridge's IP/info was last successfully resolved.",
		[]string{"bridge"}, nil,
	)
	bridgeInfoDesc = prometheus.NewDesc(
		"lumenetes_bridge_info", "Static bridge identity info, constant value 1.",
		[]string{"bridge", "name", "ip", "model_id", "api_version", "sw_version", "mac"}, nil,
	)
)

func (c *Collector) collectBridges(ctx context.Context, logger logr.Logger, ch chan<- prometheus.Metric) {
	var list lumenetesv1alpha1.HueBridgeList
	if err := c.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list hue bridges")
		return
	}

	for _, bridge := range list.Items {
		s := bridge.Status
		ch <- prometheus.MustNewConstMetric(bridgeReachableDesc, prometheus.GaugeValue, boolToFloat(s.Reachable), bridge.Name)
		if !s.LastResolved.IsZero() {
			ch <- prometheus.MustNewConstMetric(bridgeLastResolvedDesc, prometheus.GaugeValue, float64(s.LastResolved.Unix()), bridge.Name)
		}
		ch <- prometheus.MustNewConstMetric(bridgeInfoDesc, prometheus.GaugeValue, 1,
			bridge.Name, s.Name, s.IP, s.ModelID, s.APIVersion, s.SWVersion, s.MAC)
	}
}
