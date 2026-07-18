// Package cilium provides generated Go types for Cilium's CiliumNetworkPolicy
// and CiliumClusterwideNetworkPolicy CRDs.
//
// The types are generated from Cilium's own release manifests using
// gen-crds.sh. To regenerate the types, run: go generate ./...
//
// Unlike pkg/crds/istio and pkg/crds/gatewayapi, there is no InstallCRDs
// here - Cilium's own Helm chart (pkg/components/cilium) already installs
// these CRDs as part of the agent/operator deployment.
package cilium

//go:generate ./gen-crds.sh
