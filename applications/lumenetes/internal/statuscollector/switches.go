package statuscollector

import (
	"context"

	"github.com/go-logr/logr"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	switchReachableDesc = prometheus.NewDesc(
		"lumenetes_switch_reachable", "Whether the switch was reachable as of the last sync.",
		[]string{"switch", "name", "bridge_id"}, nil,
	)
	// switchBatteryDesc is skipped for mains-powered switches - see
	// SwitchStatus's own doc comment for the -1 "unknown" sentinel.
	switchBatteryDesc = prometheus.NewDesc(
		"lumenetes_switch_battery_percent", "Battery level, 0-100. Absent if unknown (e.g. mains-powered).",
		[]string{"switch", "name", "bridge_id"}, nil,
	)
	switchLastEventDesc = prometheus.NewDesc(
		"lumenetes_switch_last_event_timestamp_seconds", "Unix timestamp of the last button event this switch reported.",
		[]string{"switch", "name", "bridge_id"}, nil,
	)
	switchLastSyncedDesc = prometheus.NewDesc(
		"lumenetes_switch_last_synced_timestamp_seconds", "Unix timestamp of the last successful status sync from the bridge.",
		[]string{"switch", "name", "bridge_id"}, nil,
	)
	switchInfoDesc = prometheus.NewDesc(
		"lumenetes_switch_info", "Static switch identity info, constant value 1.",
		[]string{"switch", "name", "bridge_id", "product", "model"}, nil,
	)
)

func (c *Collector) collectSwitches(ctx context.Context, logger logr.Logger, ch chan<- prometheus.Metric) {
	var list lumenetesv1alpha1.SwitchList
	if err := c.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list switches")
		return
	}

	for _, sw := range list.Items {
		s := sw.Status
		labels := []string{sw.Name, s.Name, s.BridgeID}

		ch <- prometheus.MustNewConstMetric(switchReachableDesc, prometheus.GaugeValue, boolToFloat(s.Reachable), labels...)
		if s.Battery != -1 {
			ch <- prometheus.MustNewConstMetric(switchBatteryDesc, prometheus.GaugeValue, float64(s.Battery), labels...)
		}
		if !s.LastEventTime.IsZero() {
			ch <- prometheus.MustNewConstMetric(switchLastEventDesc, prometheus.GaugeValue, float64(s.LastEventTime.Unix()), labels...)
		}
		if !s.LastSynced.IsZero() {
			ch <- prometheus.MustNewConstMetric(switchLastSyncedDesc, prometheus.GaugeValue, float64(s.LastSynced.Unix()), labels...)
		}
		ch <- prometheus.MustNewConstMetric(switchInfoDesc, prometheus.GaugeValue, 1,
			sw.Name, s.Name, s.BridgeID, s.Product, s.Model)
	}
}
