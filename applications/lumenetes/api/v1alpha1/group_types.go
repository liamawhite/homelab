package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true

// GroupSpec is the user-declared list of Lights belonging to this group.
type GroupSpec struct {
	// Lights are the names of Light CRs that belong to this group.
	Lights []string `json:"lights,omitempty"`
	// ActiveScene selects this group's current state. Empty means
	// unmanaged - internal/groupcontroller.Reconciler leaves this group's
	// lights alone entirely, which is the safe default and what every
	// group has today (deliberately NOT the same as "off": a newly added
	// field's zero value must be inert, not force every light off the
	// moment this field exists). The literal "off" is a reserved
	// pseudo-scene that forces every light in Spec.Lights off (On only -
	// brightness/color are left as they are). Any other value must name a
	// Scene whose own Spec.Group equals this Group's name; enactment
	// happens continuously (every reconcile, including the periodic
	// resync) rather than once-on-change - manually overriding a light
	// while a scene/off is selected will be corrected back.
	ActiveScene string `json:"activeScene,omitempty"`
}

// +kubebuilder:object:generate=true

// GroupStatus reports which of Spec.Lights don't currently resolve to a
// Light CR - a typo'd or since-deleted reference would otherwise be silent.
type GroupStatus struct {
	// MissingLights are entries in Spec.Lights that don't currently match
	// any Light CR name.
	MissingLights []string `json:"missingLights,omitempty"`
	// LightCount is len(Spec.Lights) - kept in Status (rather than only
	// computed client-side) purely so kubectl can print it as a column:
	// a CRD printer column's JSONPath can only select an existing field,
	// not derive one (e.g. no len()), so there's nowhere else to source
	// this from.
	LightCount int32 `json:"lightCount,omitempty"`
	// ActiveSceneError reports why Spec.ActiveScene couldn't be enacted -
	// e.g. it names a Scene that doesn't exist, or one whose Spec.Group
	// doesn't match this Group - empty means either ActiveScene is unset/
	// "off", or the named Scene was found and validated fine.
	ActiveSceneError string `json:"activeSceneError,omitempty"`
	// LastSynced is when this status was last recomputed.
	LastSynced metav1.Time `json:"lastSynced,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Lights",type="integer",JSONPath=".status.lightCount"
// +kubebuilder:printcolumn:name="Missing",type="string",JSONPath=".status.missingLights"
// +kubebuilder:printcolumn:name="Active Scene",type="string",JSONPath=".spec.activeScene"
// +kubebuilder:printcolumn:name="Scene Error",type="string",JSONPath=".status.activeSceneError",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Group is a user-named collection of Lights, for other resources (switch
// bindings, future circadian schedules) to target instead of enumerating
// individual Lights each time. Cluster scoped like Light/Switch/HueBridge,
// but metadata.name is user-chosen (e.g. "living-room") rather than a
// stable Hue UUID - a Group has no Hue-side identity at all.
type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupSpec   `json:"spec,omitempty"`
	Status GroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GroupList is a list of Group resources.
type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Group `json:"items"`
}
