package lightscontroller

import (
	"testing"
	"time"

	lighthue "github.com/liamawhite/homelab/pkg/lights/hue"
	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMergeLightStatus(t *testing.T) {
	now := metav1.NewTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))

	baseline := lightsv1alpha1.LightStatus{
		Name: "Kitchen Sink", BridgeID: "ECB5FAFFFE9D9371", DeviceID: "dev-1",
		On: true, Brightness: 50, Color: "#ffedbb", ColorTempK: 2700,
		FixtureType: "recessed ceiling", Product: "Hue color downlight", Model: "LCD006",
		Reachable: true,
	}

	boolPtr := func(b bool) *bool { return &b }
	f64Ptr := func(f float64) *float64 { return &f }
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	t.Run("on-only event doesn't touch brightness/color/colorTempK", func(t *testing.T) {
		got := mergeLightStatus(baseline, lighthue.LightEvent{On: boolPtr(false)}, now)
		if got.On != false {
			t.Errorf("got On=%v, want false", got.On)
		}
		if got.Brightness != baseline.Brightness || got.Color != baseline.Color || got.ColorTempK != baseline.ColorTempK {
			t.Errorf("got %+v, want brightness/color/colorTempK unchanged from baseline", got)
		}
	})

	t.Run("dimming-only event doesn't touch on/color/colorTempK", func(t *testing.T) {
		got := mergeLightStatus(baseline, lighthue.LightEvent{Brightness: f64Ptr(75)}, now)
		if got.Brightness != 75 {
			t.Errorf("got Brightness=%d, want 75", got.Brightness)
		}
		if got.On != baseline.On || got.Color != baseline.Color || got.ColorTempK != baseline.ColorTempK {
			t.Errorf("got %+v, want on/color/colorTempK unchanged from baseline", got)
		}
	})

	t.Run("color event", func(t *testing.T) {
		got := mergeLightStatus(baseline, lighthue.LightEvent{Color: strPtr("#ff69b4")}, now)
		if got.Color != "#ff69b4" {
			t.Errorf("got Color=%q, want #ff69b4", got.Color)
		}
	})

	t.Run("colorTempK event", func(t *testing.T) {
		got := mergeLightStatus(baseline, lighthue.LightEvent{ColorTempK: intPtr(4000)}, now)
		if got.ColorTempK != 4000 {
			t.Errorf("got ColorTempK=%d, want 4000", got.ColorTempK)
		}
	})

	t.Run("empty event is a no-op on the observed fields", func(t *testing.T) {
		got := mergeLightStatus(baseline, lighthue.LightEvent{}, now)
		if got.On != baseline.On || got.Brightness != baseline.Brightness || got.Color != baseline.Color || got.ColorTempK != baseline.ColorTempK {
			t.Errorf("got %+v, want observed fields unchanged from baseline", got)
		}
	})

	t.Run("always sets Reachable true and LastSynced, preserves static metadata", func(t *testing.T) {
		unreachable := baseline
		unreachable.Reachable = false
		got := mergeLightStatus(unreachable, lighthue.LightEvent{On: boolPtr(true)}, now)
		if !got.Reachable {
			t.Error("got Reachable=false, want true")
		}
		if !got.LastSynced.Time.Equal(now.Time) {
			t.Errorf("got LastSynced=%v, want %v", got.LastSynced.Time, now.Time)
		}
		if got.Name != baseline.Name || got.BridgeID != baseline.BridgeID || got.DeviceID != baseline.DeviceID ||
			got.FixtureType != baseline.FixtureType || got.Product != baseline.Product || got.Model != baseline.Model {
			t.Errorf("got %+v, want static metadata untouched", got)
		}
	})
}
