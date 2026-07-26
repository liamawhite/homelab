package statuscollector

import (
	"context"

	"github.com/go-logr/logr"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// circadianScheduleBrightnessDesc/circadianScheduleColorTempDesc are
	// skipped when the schedule failed to interpolate (Current* are nil) -
	// see CircadianScheduleStatus's own doc comment.
	circadianScheduleBrightnessDesc = prometheus.NewDesc(
		"lumenetes_circadianschedule_current_brightness_percent", "Currently interpolated brightness, 0-100. Absent if the schedule is invalid.",
		[]string{"circadian_schedule"}, nil,
	)
	circadianScheduleColorTempDesc = prometheus.NewDesc(
		"lumenetes_circadianschedule_current_color_temp_kelvin", "Currently interpolated color temperature in Kelvin. Absent if the schedule is invalid.",
		[]string{"circadian_schedule"}, nil,
	)
	circadianScheduleValidationErrorDesc = prometheus.NewDesc(
		"lumenetes_circadianschedule_validation_error", "Whether this schedule currently fails validation/interpolation (1) or not (0).",
		[]string{"circadian_schedule"}, nil,
	)
)

func (c *Collector) collectCircadianSchedules(ctx context.Context, logger logr.Logger, ch chan<- prometheus.Metric) {
	var list lumenetesv1alpha1.CircadianScheduleList
	if err := c.Client.List(ctx, &list); err != nil {
		logger.Error(err, "failed to list circadian schedules")
		return
	}

	for _, schedule := range list.Items {
		s := schedule.Status
		if s.CurrentBrightness != nil {
			ch <- prometheus.MustNewConstMetric(circadianScheduleBrightnessDesc, prometheus.GaugeValue, float64(*s.CurrentBrightness), schedule.Name)
		}
		if s.CurrentColorTempK != nil {
			ch <- prometheus.MustNewConstMetric(circadianScheduleColorTempDesc, prometheus.GaugeValue, float64(*s.CurrentColorTempK), schedule.Name)
		}
		ch <- prometheus.MustNewConstMetric(circadianScheduleValidationErrorDesc, prometheus.GaugeValue, boolToFloat(s.ValidationError != ""), schedule.Name)
	}
}
