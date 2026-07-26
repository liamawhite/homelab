package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ActiveSceneKind identifies what kind of object an ActiveSceneRef.Name
// refers to - modeled on Gateway API's Group/Kind/Name reference
// convention (see HTTPRoute.ParentRefs) rather than a single string
// resolved by trying each kind of object in turn, so a Group's active
// target is always unambiguous even if a Scene and a CircadianSchedule
// happen to share a name.
type ActiveSceneKind string

const (
	// ActiveSceneKindScene references a Scene by ActiveSceneRef.Name.
	ActiveSceneKindScene ActiveSceneKind = "Scene"
	// ActiveSceneKindCircadianSchedule references a CircadianSchedule by
	// ActiveSceneRef.Name.
	ActiveSceneKindCircadianSchedule ActiveSceneKind = "CircadianSchedule"
	// ActiveSceneKindOff is the reserved pseudo-target that forces every
	// light in Spec.Lights off (On only - brightness/color/colorTempK are
	// left as they are). ActiveSceneRef.Name is ignored when Kind is Off.
	ActiveSceneKindOff ActiveSceneKind = "Off"
	// ActiveSceneKindReactive is the reserved pseudo-target that mirrors
	// each light's own observed Status back onto its Spec instead of
	// pushing a computed target down - the inverse of every other Kind,
	// so a manual/physical change simply sticks instead of being fought
	// on the next reconcile. ActiveSceneRef.Name is ignored, same as Off.
	ActiveSceneKindReactive ActiveSceneKind = "Reactive"
)

// +kubebuilder:object:generate=true

// ActiveSceneRef selects what a Group's lights should currently be doing -
// see GroupSpec.ActiveScene's doc comment.
type ActiveSceneRef struct {
	// Kind of the referenced object.
	// +kubebuilder:validation:Enum=Scene;CircadianSchedule;Off;Reactive
	// +kubebuilder:default=Scene
	Kind ActiveSceneKind `json:"kind,omitempty"`
	// Name of the referenced Scene or CircadianSchedule. Ignored when Kind
	// is Off or Reactive.
	Name string `json:"name,omitempty"`
}

// +kubebuilder:object:generate=true

// GroupSpec is the user-declared list of Lights belonging to this group.
type GroupSpec struct {
	// Lights are the names of Light CRs that belong to this group.
	Lights []string `json:"lights,omitempty"`
	// ActiveScene selects this group's current state. Nil means unmanaged -
	// internal/groupcontroller.Reconciler leaves this group's lights alone
	// entirely, which is the safe default and what every group has today
	// (deliberately NOT the same as Kind: Off: a newly added field's zero
	// value must be inert, not force every light off the moment this field
	// exists). Kind: Off forces every light in Spec.Lights off. Kind: Scene
	// or Kind: CircadianSchedule reference a Scene/CircadianSchedule whose
	// own Spec.Group equals this Group's name by Name; enactment happens
	// continuously (every reconcile, including the periodic resync) rather
	// than once-on-change - manually overriding a light while a scene/
	// schedule/off is selected will be corrected back. If a Scene and a
	// CircadianSchedule share a Name, Kind disambiguates which one is
	// meant - unlike a plain string, there's no lookup-order ambiguity.
	// Kind: Reactive inverts the direction entirely: each light's own
	// observed Status is mirrored onto its Spec instead of a computed
	// target being pushed down, so internal/lightscontroller.Reconciler
	// finds nothing to push to the bridge and a manual/physical change
	// simply sticks - internal/lightscontroller.Reconciler also refuses to
	// enact for any light belonging to a Reactive-mode Group, independent
	// of this Group's own reconcile timing (see that package's doc
	// comment).
	ActiveScene *ActiveSceneRef `json:"activeScene,omitempty"`
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
	// e.g. it names a Scene/CircadianSchedule that doesn't exist, or one
	// whose Spec.Group doesn't match this Group - empty means either
	// ActiveScene is unset/Off, or the named referent was found and
	// validated fine.
	ActiveSceneError string `json:"activeSceneError,omitempty"`
	// LastSynced is when this status was last recomputed.
	LastSynced metav1.Time `json:"lastSynced,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Lights",type="integer",JSONPath=".status.lightCount"
// +kubebuilder:printcolumn:name="Missing",type="string",JSONPath=".status.missingLights"
// +kubebuilder:printcolumn:name="Active Kind",type="string",JSONPath=".spec.activeScene.kind"
// +kubebuilder:printcolumn:name="Active Name",type="string",JSONPath=".spec.activeScene.name"
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
