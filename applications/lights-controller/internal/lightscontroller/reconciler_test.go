package lightscontroller

import (
	"slices"
	"testing"

	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	"github.com/liamawhite/lights-controller/internal/bridges"
)

func TestDiffLight(t *testing.T) {
	synced := lightsv1alpha1.LightSpec{Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700}
	syncedStatus := lightsv1alpha1.LightStatus{Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700}

	cases := []struct {
		name   string
		spec   lightsv1alpha1.LightSpec
		status lightsv1alpha1.LightStatus
		want   []string
	}{
		{
			name:   "no drift",
			spec:   synced,
			status: syncedStatus,
			want:   nil,
		},
		{
			name:   "name differs",
			spec:   lightsv1alpha1.LightSpec{Name: "Lounge"},
			status: lightsv1alpha1.LightStatus{Name: "Kitchen"},
			want:   []string{"name"},
		},
		{
			name:   "on differs",
			spec:   lightsv1alpha1.LightSpec{On: true},
			status: lightsv1alpha1.LightStatus{On: false},
			want:   []string{"on"},
		},
		{
			name:   "brightness differs",
			spec:   lightsv1alpha1.LightSpec{Brightness: 80},
			status: lightsv1alpha1.LightStatus{Brightness: 50},
			want:   []string{"brightness"},
		},
		{
			name:   "color differs",
			spec:   lightsv1alpha1.LightSpec{Color: "#ff0000"},
			status: lightsv1alpha1.LightStatus{Color: "#00ff00"},
			want:   []string{"color"},
		},
		{
			name:   "colorTempK differs",
			spec:   lightsv1alpha1.LightSpec{ColorTempK: 4000},
			status: lightsv1alpha1.LightStatus{ColorTempK: 2700},
			want:   []string{"colorTempK"},
		},
		{
			name:   "multi-field drift",
			spec:   lightsv1alpha1.LightSpec{Name: "Lounge", On: false, Brightness: 50, Color: "#ffffff", ColorTempK: 2700},
			status: syncedStatus,
			want:   []string{"name", "on"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := diffLight(tc.spec, tc.status)
			var gotFields []string
			for _, d := range got {
				gotFields = append(gotFields, d.Field)
			}
			if !slices.Equal(gotFields, tc.want) {
				t.Errorf("diffLight() fields = %v, want %v", gotFields, tc.want)
			}
		})
	}
}

func TestHasField(t *testing.T) {
	diffs := []fieldDiff{{Field: "on"}, {Field: "color"}}
	if !hasField(diffs, "on") {
		t.Error("hasField(diffs, \"on\") = false, want true")
	}
	if hasField(diffs, "brightness") {
		t.Error("hasField(diffs, \"brightness\") = true, want false")
	}
	if hasField(nil, "on") {
		t.Error("hasField(nil, \"on\") = true, want false")
	}
}

func TestBridgesFindByID(t *testing.T) {
	cfgs := []bridges.Config{{ID: "abc", AppKey: "key1"}, {ID: "def", AppKey: "key2"}}

	got, ok := bridges.FindByID(cfgs, "def")
	if !ok || got.AppKey != "key2" {
		t.Errorf("FindByID(cfgs, \"def\") = %+v, %v, want key2, true", got, ok)
	}
	if _, ok := bridges.FindByID(cfgs, "missing"); ok {
		t.Error("FindByID(cfgs, \"missing\") = true, want false")
	}
}
