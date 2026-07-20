// Package lights installs the Light CRD (see light-crd.yaml).
//
// Unlike its siblings pkg/crds/istio and pkg/crds/gatewayapi, this
// manifest doesn't come from an upstream Helm chart or GitHub release -
// it's generated directly from this repo's own Go API types
// (applications/lights-controller/api/v1alpha1) via controller-gen, kept
// in sync with `go generate ./...` (or `make gen`) like every other CRD
// here, just from a different source.
package lights

//go:generate ./gen-crds.sh
