package groupcontroller

import (
	"slices"
	"testing"
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
