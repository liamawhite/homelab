package lightwebhook

import (
	"testing"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		color      string
		colorTempK int32
		wantErr    bool
	}{
		{name: "neither set", color: "", colorTempK: 0, wantErr: false},
		{name: "only color set", color: "#ffffff", colorTempK: 0, wantErr: false},
		{name: "only colorTempK set", color: "", colorTempK: 4000, wantErr: false},
		{name: "both set", color: "#ffffff", colorTempK: 4000, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			light := &lumenetesv1alpha1.Light{
				Spec: lumenetesv1alpha1.LightSpec{
					Color:      tt.color,
					ColorTempK: tt.colorTempK,
				},
			}
			_, err := validate(light)
			if tt.wantErr && err == nil {
				t.Errorf("validate() = nil error, want an error for color=%q colorTempK=%d", tt.color, tt.colorTempK)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validate() = %v, want no error for color=%q colorTempK=%d", err, tt.color, tt.colorTempK)
			}
		})
	}
}

func TestValidate_WrongType(t *testing.T) {
	_, err := validate(&lumenetesv1alpha1.Group{})
	if err == nil {
		t.Error("validate() = nil error, want an error for a non-Light object")
	}
}
