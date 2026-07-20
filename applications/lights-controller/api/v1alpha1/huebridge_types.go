package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=true

// HueBridgeSpec is deliberately empty - see LightSpec's doc comment for
// why.
type HueBridgeSpec struct{}

// +kubebuilder:object:generate=true

// HueBridgeStatus mirrors
// github.com/liamawhite/homelab/pkg/lights/hue.Bridge, plus
// Reachable/LastResolved which that type has no notion of.
type HueBridgeStatus struct {
	// IP is the bridge's current LAN address, re-resolved by
	// hub-controller whenever it changes (e.g. a new DHCP lease) - this
	// is the whole point of this CRD: a durable, inspectable answer to
	// "where is this bridge right now."
	IP string `json:"ip,omitempty"`
	// Name is the bridge's own display name, e.g. "Hue Bridge".
	Name string `json:"name,omitempty"`
	// ModelID is the bridge's model, e.g. "BSB002".
	ModelID string `json:"modelId,omitempty"`
	// APIVersion is the bridge's CLIP API version.
	APIVersion string `json:"apiVersion,omitempty"`
	// SWVersion is the bridge's firmware version.
	SWVersion string `json:"swVersion,omitempty"`
	// MAC is the bridge's MAC address.
	MAC string `json:"mac,omitempty"`
	// Reachable is false when the most recent discovery round didn't
	// find this bridge - the rest of this status is then stale, left as
	// of the last successful resolution rather than cleared.
	Reachable bool `json:"reachable,omitempty"`
	// LastResolved is when this status was last successfully updated.
	LastResolved metav1.Time `json:"lastResolved,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".status.name"
// +kubebuilder:printcolumn:name="IP",type="string",JSONPath=".status.ip"
// +kubebuilder:printcolumn:name="Reachable",type="boolean",JSONPath=".status.reachable"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// HueBridge represents a single Hue bridge's live-resolved network
// location, maintained by hub-controller (see
// applications/lights-controller/cmd/hub-controller) so lights-controller
// can reach it without doing its own discovery. Cluster scoped, same
// reasoning as Light. Named after the bridge's stable Hue ID (matches
// config.HueBridgeConfig.ID and Light.status.bridgeId values).
type HueBridge struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HueBridgeSpec   `json:"spec,omitempty"`
	Status HueBridgeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HueBridgeList is a list of HueBridge resources.
type HueBridgeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []HueBridge `json:"items"`
}
