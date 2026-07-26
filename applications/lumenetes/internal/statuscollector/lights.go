package statuscollector

import (
	"context"

	"github.com/go-logr/logr"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	lightOnDesc = prometheus.NewDesc(
		"lumenetes_light_on", "Whether the light is currently on (1) or off (0).",
		[]string{"light", "name", "bridge_id"}, nil,
	)
	lightReachableDesc = prometheus.NewDesc(
		"lumenetes_light_reachable", "Whether the light was reachable as of the last sync.",
		[]string{"light", "name", "bridge_id"}, nil,
	)
	// lightBrightnessDesc/lightColorTempDesc are skipped entirely for a
	// light that doesn't support the dimension - see LightStatus's own
	// doc comment for the -1/0 "unsupported" sentinel convention.
	lightBrightnessDesc = prometheus.NewDesc(
		"lumenetes_light_brightness_percent", "Current brightness, 0-100. Absent if the light doesn't support dimming.",
		[]string{"light", "name", "bridge_id"}, nil,
	)
	lightColorTempDesc = prometheus.NewDesc(
		"lumenetes_light_color_temp_kelvin", "Current color temperature in Kelvin. Absent if the light doesn't support it.",
		[]string{"light", "name", "bridge_id"}, nil,
	)
	lightEnactErrorDesc = prometheus.NewDesc(
		"lumenetes_light_enact_error", "Whether the last attempt to enact Spec onto the bridge failed (1) or not (0).",
		[]string{"light", "name", "bridge_id"}, nil,
	)
	lightLastSyncedDesc = prometheus.NewDesc(
		"lumenetes_light_last_synced_timestamp_seconds", "Unix timestamp of the last successful status sync from the bridge.",
		[]string{"light", "name", "bridge_id"}, nil,
	)
	lightInfoDesc = prometheus.NewDesc(
		"lumenetes_light_info", "Static light identity/capability info, constant value 1.",
		[]string{"light", "name", "bridge_id", "product", "model", "fixture_type", "color"}, nil,
	)
)

func (c *Collector) collectLights(ctx context.Context, logger logr.Logger, ch chan<- prometheus.Metric) {
	var list lumenetesv1alpha1.LightList
	if err := c.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list lights")
		return
	}

	for _, light := range list.Items {
		s := light.Status
		labels := []string{light.Name, s.Name, s.BridgeID}

		ch <- prometheus.MustNewConstMetric(lightOnDesc, prometheus.GaugeValue, boolToFloat(s.On), labels...)
		ch <- prometheus.MustNewConstMetric(lightReachableDesc, prometheus.GaugeValue, boolToFloat(s.Reachable), labels...)
		if s.Brightness != -1 {
			ch <- prometheus.MustNewConstMetric(lightBrightnessDesc, prometheus.GaugeValue, float64(s.Brightness), labels...)
		}
		if s.ColorTempK != 0 {
			ch <- prometheus.MustNewConstMetric(lightColorTempDesc, prometheus.GaugeValue, float64(s.ColorTempK), labels...)
		}
		ch <- prometheus.MustNewConstMetric(lightEnactErrorDesc, prometheus.GaugeValue, boolToFloat(s.EnactError != ""), labels...)
		if !s.LastSynced.IsZero() {
			ch <- prometheus.MustNewConstMetric(lightLastSyncedDesc, prometheus.GaugeValue, float64(s.LastSynced.Unix()), labels...)
		}
		ch <- prometheus.MustNewConstMetric(lightInfoDesc, prometheus.GaugeValue, 1,
			light.Name, s.Name, s.BridgeID, s.Product, s.Model, s.FixtureType, s.Color)
	}
}
