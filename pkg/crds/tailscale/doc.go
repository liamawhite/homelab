// Package tailscale provides generated Go types for the Tailscale
// Kubernetes Operator's CRDs (ProxyClass, Connector, ProxyGroup, DNSConfig,
// Recorder, ...).
//
// The types are generated from the tailscale-operator Helm chart using
// gen-crds.sh. To regenerate the types, run: go generate ./...
//
// Unlike pkg/crds/istio and pkg/crds/gatewayapi, there is no InstallCRDs
// here - the tailscale-operator chart's own installCRDs value (true by
// default) already installs these CRDs as part of its own release, the same
// single-chart pattern pkg/crds/cilium documents.
package tailscale

//go:generate ./gen-crds.sh
