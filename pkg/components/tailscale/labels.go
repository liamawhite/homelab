package tailscale

// AccessLabelKey and AccessLabelValue gate egress to Tailscale's control
// plane/DERP/WireGuard under Cilium's default-deny baseline - applied to the
// operator's own pod (operatorConfig.podLabels) and to every dynamically
// created per-Ingress proxy pod (via the default ProxyClass this component
// creates), same opt-in-label convention as
// pkg/components/cloudflare/tunnel.
const (
	AccessLabelKey   = "network.homelab.io/tailscale"
	AccessLabelValue = "true"
)

// WaypointAccessLabelKey and WaypointAccessLabelValue mark a waypoint proxy
// (pkg/components/istio/waypoint) as reachable from Tailscale's proxy pods -
// the destination-side counterpart to AccessLabelKey/AccessLabelValue above,
// mirroring pkg/components/cloudflare/tunnel.WaypointAccessLabelKey. Any app
// whose Service is routed through Tailscale adds this to its own waypoint
// (via waypoint.WaypointArgs.Labels) to opt in.
const (
	WaypointAccessLabelKey   = "network.homelab.io/tailscale-waypoint"
	WaypointAccessLabelValue = "true"
)

// DefaultProxyClassName is the ProxyClass this component creates and points
// the tailscale-operator chart's proxyConfig.defaultProxyClass at, so every
// dynamically created per-Ingress proxy pod picks up AccessLabelKey without
// each app having to know about ProxyClass itself.
const DefaultProxyClassName = "homelab-default"

// IngressClassName is the IngressClass the tailscale-operator chart creates
// itself (ingressClass.name, default "tailscale") - exported so callers
// (pkg/components/tailscale/ingress) reference the same literal instead of
// retyping it.
const IngressClassName = "tailscale"

// ProxiesServiceAccountName is the ServiceAccount the tailscale-operator
// chart creates once and every dynamically created proxy pod runs as - the
// StatefulSet/pod name is unique per Ingress (e.g. "ts-private-lf7s4-0"),
// but the ServiceAccount is fixed and shared across every proxy the
// operator ever creates, confirmed live (kubectl get pod ... -o
// jsonpath='{.spec.serviceAccountName}' -> "proxies"). Exported so callers
// can build the exact SPIFFE identity
// (cluster.local/ns/<namespace>/sa/<this>) for their own
// AuthorizationPolicy source-principal checks (e.g.
// pkg/components/tailscale/ingress), the same way
// pkg/components/cloudflare/tunnel.ServiceAccountName is used - not a
// namespace-prefix wildcard.
const ProxiesServiceAccountName = "proxies"

// OperatorTag and ProxyTag are the tailnet ACL tags the tailscale-operator
// chart assigns by its own default (operatorConfig.defaultTags /
// proxyConfig.defaultTags - now set explicitly to these values in
// component.go rather than left to the chart's implicit default, so they
// can't silently drift from what pkg/components/tailscale/acl declares as
// tagOwners for them).
const (
	OperatorTag = "tag:k8s-operator"
	ProxyTag    = "tag:k8s"
)
