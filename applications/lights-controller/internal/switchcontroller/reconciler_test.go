package switchcontroller

import (
	"testing"
	"time"

	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
)

func boolPtr(b bool) *bool     { return &b }
func int32Ptr(v int32) *int32  { return &v }
func strPtr(s string) *string  { return &s }

func TestApplyActionToSpec(t *testing.T) {
	baseline := lightsv1alpha1.LightSpec{
		Name: "Kitchen", On: false, Brightness: 50, Color: "#ffedbb", ColorTempK: 2700,
	}
	noDimming := lightsv1alpha1.LightSpec{Name: "Fan Light", On: true, Brightness: -1, Color: "", ColorTempK: 0}

	cases := []struct {
		name    string
		current lightsv1alpha1.LightSpec
		action  lightsv1alpha1.SwitchAction
		want    lightsv1alpha1.LightSpec
	}{
		{
			name:    "on true",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{On: boolPtr(true)},
			want:    withOn(baseline, true),
		},
		{
			name:    "on false",
			current: withOn(baseline, true),
			action:  lightsv1alpha1.SwitchAction{On: boolPtr(false)},
			want:    baseline,
		},
		{
			name:    "toggle flips true to false",
			current: withOn(baseline, true),
			action:  lightsv1alpha1.SwitchAction{Toggle: true},
			want:    baseline,
		},
		{
			name:    "toggle flips false to true",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{Toggle: true},
			want:    withOn(baseline, true),
		},
		{
			name:    "brightness set in range",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{Brightness: int32Ptr(80)},
			want:    withBrightness(baseline, 80),
		},
		{
			name:    "brightness set clamped above 100",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{Brightness: int32Ptr(150)},
			want:    withBrightness(baseline, 100),
		},
		{
			name:    "brightness set clamped below 0",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{Brightness: int32Ptr(-20)},
			want:    withBrightness(baseline, 0),
		},
		{
			name:    "brightness set no-op on non-dimmable light",
			current: noDimming,
			action:  lightsv1alpha1.SwitchAction{Brightness: int32Ptr(50)},
			want:    noDimming,
		},
		{
			name:    "brightness delta positive with clamping at 100",
			current: withBrightness(baseline, 95),
			action:  lightsv1alpha1.SwitchAction{BrightnessDelta: int32Ptr(10)},
			want:    withBrightness(baseline, 100),
		},
		{
			name:    "brightness delta negative with clamping at 0",
			current: withBrightness(baseline, 5),
			action:  lightsv1alpha1.SwitchAction{BrightnessDelta: int32Ptr(-10)},
			want:    withBrightness(baseline, 0),
		},
		{
			name:    "brightness delta no-op on non-dimmable light",
			current: noDimming,
			action:  lightsv1alpha1.SwitchAction{BrightnessDelta: int32Ptr(10)},
			want:    noDimming,
		},
		{
			name:    "color set on color-capable light",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{Color: strPtr("#ff0000")},
			want:    withColor(baseline, "#ff0000"),
		},
		{
			name:    "color set no-op on non-color light",
			current: noDimming,
			action:  lightsv1alpha1.SwitchAction{Color: strPtr("#ff0000")},
			want:    noDimming,
		},
		{
			name:    "colorTempK set on capable light",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{ColorTempK: int32Ptr(4000)},
			want:    withColorTempK(baseline, 4000),
		},
		{
			name:    "colorTempK set no-op when unsupported",
			current: noDimming,
			action:  lightsv1alpha1.SwitchAction{ColorTempK: int32Ptr(4000)},
			want:    noDimming,
		},
		{
			name:    "combined on and brightnessDelta",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{On: boolPtr(true), BrightnessDelta: int32Ptr(10)},
			want:    withBrightness(withOn(baseline, true), 60),
		},
		{
			name:    "nil action is a true no-op",
			current: baseline,
			action:  lightsv1alpha1.SwitchAction{},
			want:    baseline,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyActionToSpec(tc.current, tc.action)
			if got != tc.want {
				t.Errorf("applyActionToSpec() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func withOn(s lightsv1alpha1.LightSpec, on bool) lightsv1alpha1.LightSpec {
	s.On = on
	return s
}

func withBrightness(s lightsv1alpha1.LightSpec, b int32) lightsv1alpha1.LightSpec {
	s.Brightness = b
	return s
}

func withColor(s lightsv1alpha1.LightSpec, c string) lightsv1alpha1.LightSpec {
	s.Color = c
	return s
}

func withColorTempK(s lightsv1alpha1.LightSpec, k int32) lightsv1alpha1.LightSpec {
	s.ColorTempK = k
	return s
}

func TestIsNewEvent(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		lastEvent   time.Time
		lastHandled time.Time
		want        bool
	}{
		{"zero lastEvent", time.Time{}, base, false},
		{"lastEvent after lastHandled", base.Add(time.Second), base, true},
		{"equal timestamps", base, base, false},
		{"lastEvent before lastHandled (stale/out-of-order)", base, base.Add(time.Second), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNewEvent(tc.lastEvent, tc.lastHandled); got != tc.want {
				t.Errorf("isNewEvent() = %v, want %v", got, tc.want)
			}
		})
	}
}
