// Package v1alpha1 contains the Light and HueBridge API types (see
// types.go and huebridge_types.go respectively). DeepCopy methods
// (zz_generated.deepcopy.go) and the CRD manifests
// (pkg/crds/lights/*.yaml, in the root module) are both generated from
// the markers below via controller-gen - see pkg/crds/lights/gen-crds.sh.
// Run `go generate ./...` from the repo root (or `make gen`) after
// changing these types.
//
// +groupName=lights.homelab.internal
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true

// LightSpec is deliberately empty in this phase: the controller only
// reports observed state (see LightStatus), it doesn't accept desired
// state yet. Future control fields (desired on/brightness/color) land
// here without requiring an API version bump.
type LightSpec struct{}

// +kubebuilder:object:generate=true

// LightStatus mirrors github.com/liamawhite/homelab/pkg/lights/hue.Light,
// plus Reachable/LastSynced which that type has no notion of.
type LightStatus struct {
	// Name is the light's human-readable Hue name (e.g. "Kitchen Sink") -
	// metadata.name is the Hue UUID instead (stable, valid as a k8s
	// object name), so this is the only place the friendly name appears.
	Name string `json:"name,omitempty"`
	// BridgeID is the Hue bridge id (see pkg/config.HueBridgeConfig) this
	// light belongs to.
	BridgeID string `json:"bridgeId,omitempty"`
	// On is the light's last-observed on/off state.
	On bool `json:"on,omitempty"`
	// Brightness is a percentage (0-100), or -1 if the light doesn't
	// support dimming - same sentinel convention as hue.Light.Brightness,
	// deliberately not a pointer so DeepCopyInto stays a trivial value copy.
	Brightness int32 `json:"brightness,omitempty"`
	// Color is an approximate "#rrggbb" swatch, or "" if the light doesn't
	// support color.
	Color string `json:"color,omitempty"`
	// ColorTempK is the color temperature in Kelvin, or 0 if the light
	// doesn't support color temperature - same sentinel convention as
	// hue.Light.ColorTempK.
	ColorTempK int32 `json:"colorTempK,omitempty"`
	// FixtureType is the light's archetype, e.g. "recessed ceiling".
	FixtureType string `json:"fixtureType,omitempty"`
	// Product is the owning device's product name, e.g. "Hue color lamp".
	Product string `json:"product,omitempty"`
	// Model is the owning device's model ID, e.g. "LCA005".
	Model string `json:"model,omitempty"`
	// Reachable is false when the owning bridge failed to respond on the
	// most recent poll - the rest of this status is then stale, left as
	// of the last successful sync rather than cleared.
	Reachable bool `json:"reachable,omitempty"`
	// LastSynced is when this status was last successfully updated from
	// the bridge.
	LastSynced metav1.Time `json:"lastSynced,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".status.name"
// +kubebuilder:printcolumn:name="On",type="boolean",JSONPath=".status.on"
// +kubebuilder:printcolumn:name="Brightness",type="integer",JSONPath=".status.brightness"
// +kubebuilder:printcolumn:name="Bridge",type="string",JSONPath=".status.bridgeId"
// +kubebuilder:printcolumn:name="Reachable",type="boolean",JSONPath=".status.reachable",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Light represents a single Hue light's live, observed state. Cluster
// scoped (like Node/PersistentVolume) since it represents a physical
// device, not a namespace-scoped app resource. Named after the light's own
// Hue UUID (stable, unique, valid as a Kubernetes object name).
type Light struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LightSpec   `json:"spec,omitempty"`
	Status LightStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LightList is a list of Light resources.
type LightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Light `json:"items"`
}
