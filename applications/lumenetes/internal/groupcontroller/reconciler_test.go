package groupcontroller

import (
	"slices"
	"testing"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
)

func TestMissingLights(t *testing.T) {
	cases := []struct {
		name     string
		spec     []string
		existing map[string]bool
		want     []string
	}{
		{
			name: "empty spec",
			spec: nil,
			want: nil,
		},
		{
			name:     "all present",
			spec:     []string{"a", "b"},
			existing: map[string]bool{"a": true, "b": true},
			want:     nil,
		},
		{
			name:     "some missing",
			spec:     []string{"a", "b", "c"},
			existing: map[string]bool{"a": true, "c": true},
			want:     []string{"b"},
		},
		{
			name:     "all missing",
			spec:     []string{"a", "b"},
			existing: map[string]bool{},
			want:     []string{"a", "b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := missingLights(tc.spec, tc.existing)
			if !slices.Equal(got, tc.want) {
				t.Errorf("missingLights(%v, %v) = %v, want %v", tc.spec, tc.existing, got, tc.want)
			}
		})
	}
}

func boolPtr(b bool) *bool    { return &b }
func int32Ptr(v int32) *int32 { return &v }
func strPtr(s string) *string { return &s }

func TestApplySceneStateToSpec(t *testing.T) {
	baseline := lumenetesv1alpha1.LightSpec{
		Name: "Kitchen", On: false, Brightness: 50, Color: "#ffedbb", ColorTempK: 2700,
	}
	noDimming := lumenetesv1alpha1.LightSpec{Name: "Fan Light", On: true, Brightness: -1, Color: "", ColorTempK: 0}

	cases := []struct {
		name    string
		current lumenetesv1alpha1.LightSpec
		state   lumenetesv1alpha1.SceneLightState
		want    lumenetesv1alpha1.LightSpec
	}{
		{
			name:    "on true",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{On: boolPtr(true)},
			want:    withOn(baseline, true),
		},
		{
			name:    "on false",
			current: withOn(baseline, true),
			state:   lumenetesv1alpha1.SceneLightState{On: boolPtr(false)},
			want:    baseline,
		},
		{
			name:    "brightness set in range",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{Brightness: int32Ptr(80)},
			want:    withBrightness(baseline, 80),
		},
		{
			name:    "brightness set clamped above 100",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{Brightness: int32Ptr(150)},
			want:    withBrightness(baseline, 100),
		},
		{
			name:    "brightness set clamped below 0",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{Brightness: int32Ptr(-20)},
			want:    withBrightness(baseline, 0),
		},
		{
			name:    "brightness set no-op on non-dimmable light",
			current: noDimming,
			state:   lumenetesv1alpha1.SceneLightState{Brightness: int32Ptr(50)},
			want:    noDimming,
		},
		{
			name:    "color set on color-capable light",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{Color: strPtr("#ff0000")},
			want:    withColor(baseline, "#ff0000"),
		},
		{
			name:    "color set no-op on non-color light",
			current: noDimming,
			state:   lumenetesv1alpha1.SceneLightState{Color: strPtr("#ff0000")},
			want:    noDimming,
		},
		{
			name:    "colorTempK set on capable light",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{ColorTempK: int32Ptr(4000)},
			want:    withColorTempK(baseline, 4000),
		},
		{
			name:    "colorTempK set no-op when unsupported",
			current: noDimming,
			state:   lumenetesv1alpha1.SceneLightState{ColorTempK: int32Ptr(4000)},
			want:    noDimming,
		},
		{
			name:    "combined on and brightness",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{On: boolPtr(true), Brightness: int32Ptr(20)},
			want:    withBrightness(withOn(baseline, true), 20),
		},
		{
			name:    "empty state is a true no-op",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{},
			want:    baseline,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applySceneStateToSpec(tc.current, tc.state)
			if got != tc.want {
				t.Errorf("applySceneStateToSpec() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func withOn(s lumenetesv1alpha1.LightSpec, on bool) lumenetesv1alpha1.LightSpec {
	s.On = on
	return s
}

func withBrightness(s lumenetesv1alpha1.LightSpec, b int32) lumenetesv1alpha1.LightSpec {
	s.Brightness = b
	return s
}

func withColor(s lumenetesv1alpha1.LightSpec, c string) lumenetesv1alpha1.LightSpec {
	s.Color = c
	return s
}

func withColorTempK(s lumenetesv1alpha1.LightSpec, k int32) lumenetesv1alpha1.LightSpec {
	s.ColorTempK = k
	return s
}
