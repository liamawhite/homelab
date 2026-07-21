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

// LightSpec is the desired/controllable subset of light state. It is
// seeded exactly once - from the live light state observed at the moment
// its Light CR is first created (see Poller.upsert in
// internal/lightscontroller/poller.go) - after which the poller never
// touches it again. Any later difference between Spec and Status
// therefore reflects either a user edit (kubectl edit/apply) or a change
// made directly at the bridge/app. internal/lightscontroller.Reconciler
// enacts that difference back onto the physical bridge (unless run with
// --dry-run, which only logs it).
type LightSpec struct {
	// Name is the desired human-readable Hue name. NOTE: actually renaming
	// a Hue light requires a PUT to the owning *device* resource, not this
	// light resource (a light's own metadata.name is deprecated/read-only
	// in the Hue API) - see Status.DeviceID and Reconciler.
	Name string `json:"name,omitempty"`
	// On is the desired on/off state.
	On bool `json:"on,omitempty"`
	// Brightness is the desired percentage (0-100), or -1 if the light
	// doesn't support dimming - same sentinel convention as
	// LightStatus.Brightness.
	Brightness int32 `json:"brightness,omitempty"`
	// Color is the desired approximate "#rrggbb" swatch, or "" if the
	// light doesn't support color.
	Color string `json:"color,omitempty"`
	// ColorTempK is the desired color temperature in Kelvin, or 0 if the
	// light doesn't support color temperature.
	ColorTempK int32 `json:"colorTempK,omitempty"`
}

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
	// DeviceID is the RID of the device owning this light - needed to
	// enact Name changes, which the light resource's own PUT doesn't
	// support (see Reconciler).
	DeviceID string `json:"deviceId,omitempty"`
	// LastEnactAttempt is when the controller last attempted to push Spec
	// to the bridge (success or failure) - also the debounce clock:
	// Reconcile won't re-attempt within --enact-cooldown of this timestamp.
	LastEnactAttempt metav1.Time `json:"lastEnactAttempt,omitempty"`
	// EnactError is the most recent enactment failure, or "" if the last
	// attempt succeeded (or none has been made, or nothing currently
	// differs between Spec and Status).
	EnactError string `json:"enactError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".status.name"
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
