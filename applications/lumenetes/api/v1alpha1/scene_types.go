package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true

// SceneLightState is one light's desired absolute state within a Scene.
// Pointer fields mean "leave this light's corresponding Spec field
// untouched" (same convention as SwitchAction) - a Scene can address only
// a subset of a light's fields, or only a subset of its Group's lights.
// Unlike SwitchAction there is no Toggle/BrightnessDelta: those are
// relative/stateful adjustments appropriate for a physical button, not for
// "recall this absolute state."
type SceneLightState struct {
	// Name is the Light CR this state applies to - must be a member of the
	// owning Scene's Spec.Group (see SceneStatus.InvalidLights).
	Name string `json:"name"`
	// On, if set, forces this light's desired on/off state.
	On *bool `json:"on,omitempty"`
	// Brightness sets desired brightness to this absolute percentage,
	// clamped 0-100. No-op on a light that doesn't support dimming.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Brightness *int32 `json:"brightness,omitempty"`
	// Color sets desired color to this "#rrggbb" swatch. No-op on a light
	// that doesn't support color.
	Color *string `json:"color,omitempty"`
	// ColorTempK sets desired color temperature in Kelvin. No-op on a
	// light that doesn't support color temperature.
	ColorTempK *int32 `json:"colorTempK,omitempty"`
}

// +kubebuilder:object:generate=true

// SceneSpec declares a named, recallable state for some or all of a
// Group's lights. A Scene does no enactment itself - internal/
// groupcontroller.Reconciler applies Lights onto each target Light.Spec
// when the owning Group's Spec.ActiveScene names this Scene, and internal/
// lightscontroller.Reconciler does the actual bridge push from there,
// unchanged - a Scene has no bridge/hue dependency at all.
type SceneSpec struct {
	// Group is the name of the Group this scene applies to. A Group's
	// Spec.ActiveScene must match this Scene's own metadata.name for it to
	// ever be enacted - a Scene naming a Group that doesn't select it back
	// is inert.
	Group string `json:"group"`
	// Lights are this scene's per-light desired states. Entries not
	// naming a member of Group are reported in Status.InvalidLights and
	// ignored during enactment.
	Lights []SceneLightState `json:"lights,omitempty"`
}

// +kubebuilder:object:generate=true

// SceneStatus reports which of Spec.Lights don't currently resolve to a
// member of Spec.Group - a typo'd, deleted, or since-moved light reference
// would otherwise be silently ignored during enactment.
type SceneStatus struct {
	// InvalidLights are entries in Spec.Lights that don't currently exist
	// as a Light CR, or that aren't a member of Spec.Group.
	InvalidLights []string `json:"invalidLights,omitempty"`
	// LastSynced is when this status was last recomputed.
	LastSynced metav1.Time `json:"lastSynced,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Group",type="string",JSONPath=".spec.group"
// +kubebuilder:printcolumn:name="Invalid",type="string",JSONPath=".status.invalidLights",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Scene is a user-named, recallable lighting state for some or all of a
// Group's lights. Cluster scoped, user-chosen name (e.g. "movie-night"),
// same reasoning as Group - a Scene has no Hue-side identity of its own.
type Scene struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SceneSpec   `json:"spec,omitempty"`
	Status SceneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SceneList is a list of Scene resources.
type SceneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Scene `json:"items"`
}
