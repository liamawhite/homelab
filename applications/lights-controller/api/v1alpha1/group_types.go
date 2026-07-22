package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true

// GroupSpec is the user-declared list of Lights belonging to this group.
type GroupSpec struct {
	// Lights are the names of Light CRs that belong to this group.
	Lights []string `json:"lights,omitempty"`
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
	// LastSynced is when this status was last recomputed.
	LastSynced metav1.Time `json:"lastSynced,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Lights",type="integer",JSONPath=".status.lightCount"
// +kubebuilder:printcolumn:name="Missing",type="string",JSONPath=".status.missingLights"
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
