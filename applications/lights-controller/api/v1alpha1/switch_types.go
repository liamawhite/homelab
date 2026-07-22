package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true

// SwitchAction is what a button binding does when it fires - a patch
// applied to every named TargetLights's Spec, exactly as if a user had
// edited it (internal/lightscontroller.Reconciler does the actual bridge
// enactment from there, unchanged).
type SwitchAction struct {
	// TargetLights are the names of Light CRs this action applies to.
	TargetLights []string `json:"targetLights,omitempty"`
	// On, if set, forces the target lights' desired on/off state.
	On *bool `json:"on,omitempty"`
	// Toggle, if true, flips each target light's desired on/off state
	// (based on its current Spec, not Status - see Reconciler). Ignored
	// if On is also set.
	Toggle bool `json:"toggle,omitempty"`
	// Brightness sets desired brightness to this absolute percentage,
	// clamped 0-100. No-op on a light that doesn't support dimming.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Brightness *int32 `json:"brightness,omitempty"`
	// BrightnessDelta adjusts desired brightness by this many percentage
	// points (can be negative), clamped 0-100 - for continuous dimming via
	// repeated "repeat" events while a button is held. No-op on a light
	// that doesn't support dimming.
	BrightnessDelta *int32 `json:"brightnessDelta,omitempty"`
	// Color sets desired color to this "#rrggbb" swatch. No-op on a light
	// that doesn't support color.
	Color *string `json:"color,omitempty"`
	// ColorTempK sets desired color temperature in Kelvin. No-op on a
	// light that doesn't support color temperature.
	ColorTempK *int32 `json:"colorTempK,omitempty"`
}

// +kubebuilder:object:generate=true

// SwitchBinding fires Action whenever this button reports Event.
type SwitchBinding struct {
	// Event is the Hue button event this binding fires on.
	// +kubebuilder:validation:Enum=initial_press;repeat;short_release;long_release;double_short_release;long_press
	Event string `json:"event"`
	// Action is what to do when Event fires.
	Action SwitchAction `json:"action"`
}

// +kubebuilder:object:generate=true

// SwitchSpec is the user-declared button->action configuration for a
// single physical button. Unlike LightSpec, this starts nil at creation -
// there is no seed-from-observed-state step, since a switch's desired
// bindings are pure user configuration from day one. Empty Bindings means
// this button is observed but does nothing.
type SwitchSpec struct {
	Bindings []SwitchBinding `json:"bindings,omitempty"`
}

// +kubebuilder:object:generate=true

// SwitchStatus mirrors github.com/liamawhite/homelab/pkg/lights/hue.Switch,
// plus Reachable/LastSynced (same convention as LightStatus) and
// LastHandledEventTime (internal/switchcontroller.Reconciler's own
// bookkeeping of which event it has already acted on).
type SwitchStatus struct {
	// Name is the owning device's name; buttons have no name of their own.
	Name string `json:"name,omitempty"`
	// BridgeID is the Hue bridge id this switch belongs to.
	BridgeID string `json:"bridgeId,omitempty"`
	// ControlID is which button/control this is on a multi-button device.
	ControlID int32 `json:"controlId,omitempty"`
	// LastEvent is the most recent button event reported by the bridge,
	// e.g. "short_release", "long_press" - empty if never reported.
	LastEvent string `json:"lastEvent,omitempty"`
	// LastEventTime is when LastEvent was reported.
	LastEventTime metav1.Time `json:"lastEventTime,omitempty"`
	// LastHandledEventTime is the LastEventTime of the most recent event
	// internal/switchcontroller.Reconciler has already acted on -
	// comparing it to LastEventTime distinguishes a genuinely new button
	// event from a repeat Reconcile delivery of an already-handled one.
	LastHandledEventTime metav1.Time `json:"lastHandledEventTime,omitempty"`
	// Battery is a percentage 0-100, or -1 if unknown (e.g. mains-powered).
	Battery int32 `json:"battery,omitempty"`
	// Product is the owning device's product name.
	Product string `json:"product,omitempty"`
	// Model is the owning device's model ID.
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
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".status.name"
// +kubebuilder:printcolumn:name="Control",type="integer",JSONPath=".status.controlId"
// +kubebuilder:printcolumn:name="Last Event",type="string",JSONPath=".status.lastEvent"
// +kubebuilder:printcolumn:name="Battery",type="integer",JSONPath=".status.battery"
// +kubebuilder:printcolumn:name="Bridge",type="string",JSONPath=".status.bridgeId"
// +kubebuilder:printcolumn:name="Reachable",type="boolean",JSONPath=".status.reachable",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Switch represents a single physical button's live, observed state plus
// its user-declared bindings. Cluster scoped, same reasoning as Light.
// Named after the button resource's own Hue UUID (stable, unique, valid as
// a Kubernetes object name) - a multi-button device (e.g. a 4-button Hue
// Dimmer Switch) produces one Switch CR per button, sharing BridgeID/
// Name/Product/Model/Battery but with distinct names/ControlID.
type Switch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwitchSpec   `json:"spec,omitempty"`
	Status SwitchStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SwitchList is a list of Switch resources.
type SwitchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Switch `json:"items"`
}
