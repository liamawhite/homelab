package scenecontroller

import (
	"slices"
	"testing"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
)

func sceneLights(names ...string) []lumenetesv1alpha1.SceneLightState {
	states := make([]lumenetesv1alpha1.SceneLightState, len(names))
	for i, name := range names {
		states[i] = lumenetesv1alpha1.SceneLightState{Name: name}
	}
	return states
}

func TestInvalidLights(t *testing.T) {
	cases := []struct {
		name           string
		sceneLights    []lumenetesv1alpha1.SceneLightState
		groupMembers   map[string]bool
		existingLights map[string]bool
		want           []string
	}{
		{
			name: "empty spec",
			want: nil,
		},
		{
			name:           "all valid",
			sceneLights:    sceneLights("a", "b"),
			groupMembers:   map[string]bool{"a": true, "b": true},
			existingLights: map[string]bool{"a": true, "b": true},
			want:           nil,
		},
		{
			name:           "not a group member",
			sceneLights:    sceneLights("a", "b"),
			groupMembers:   map[string]bool{"a": true},
			existingLights: map[string]bool{"a": true, "b": true},
			want:           []string{"b"},
		},
		{
			name:           "light doesn't exist",
			sceneLights:    sceneLights("a", "b"),
			groupMembers:   map[string]bool{"a": true, "b": true},
			existingLights: map[string]bool{"a": true},
			want:           []string{"b"},
		},
		{
			name:           "group not found - every light invalid",
			sceneLights:    sceneLights("a", "b"),
			groupMembers:   nil,
			existingLights: map[string]bool{"a": true, "b": true},
			want:           []string{"a", "b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := invalidLights(tc.sceneLights, tc.groupMembers, tc.existingLights)
			if !slices.Equal(got, tc.want) {
				t.Errorf("invalidLights(%v, %v, %v) = %v, want %v", tc.sceneLights, tc.groupMembers, tc.existingLights, got, tc.want)
			}
		})
	}
}
