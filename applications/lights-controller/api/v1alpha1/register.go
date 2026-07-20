package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion identifies this API's group/version, matching the group in
// pkg/crds/lights/*.yaml's spec.group.
var GroupVersion = schema.GroupVersion{Group: "lights.homelab.internal", Version: "v1alpha1"}

// SchemeBuilder collects this package's types for AddToScheme.
var SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

// AddToScheme registers Light/LightList/HueBridge/HueBridgeList with a
// runtime.Scheme, for controller-runtime's typed client to use. Both
// lights-controller and hub-controller call this - each only actually
// reads/writes one of the two kinds, but sharing one scheme is simpler
// than splitting it, and RBAC (not scheme registration) is what actually
// enforces which kind each binary may touch.
var AddToScheme = SchemeBuilder.AddToScheme

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, &Light{}, &LightList{}, &HueBridge{}, &HueBridgeList{})
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
