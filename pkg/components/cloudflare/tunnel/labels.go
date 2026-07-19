package tunnel

// AccessLabelKey and AccessLabelValue together form the pod label a
// workload must carry to get egress access to the Cloudflare Tunnel edge -
// callers add this to their own pod template's Labels (alongside whatever
// labels that workload already sets) to opt in. Exported for consistency
// with pkg/components/dns/istio/apiserver, though today only this
// component's own cloudflared Deployment needs it.
const (
	AccessLabelKey   = "network.homelab.io/cloudflare-tunnel"
	AccessLabelValue = "true"
)

// WaypointAccessLabelKey and WaypointAccessLabelValue mark a waypoint proxy
// (pkg/components/istio/waypoint) as reachable from cloudflared - the
// destination-side counterpart to AccessLabelKey/AccessLabelValue above
// (which gates cloudflared's own egress to the *Cloudflare* edge, not to
// in-cluster destinations). Any app whose Service is routed through the
// Cloudflare Tunnel adds this to its own waypoint (via
// waypoint.WaypointArgs.Labels) to opt in - reusable across apps, unlike
// the waypoint-to-app leg, which is specific to each app's own pods and so
// isn't a shared label (see pkg/deploy/applications/home.go).
const (
	WaypointAccessLabelKey   = "network.homelab.io/cloudflare-tunnel-waypoint"
	WaypointAccessLabelValue = "true"
)

// ServiceAccountName is the name this component gives cloudflared's
// ServiceAccount (see NewTunnel) - exported so callers can build cloudflared's
// SPIFFE identity (spiffe://cluster.local/ns/<namespace>/sa/<this>) for their
// own AuthorizationPolicy source-principal checks (e.g.
// pkg/components/cloudflare/accessjwt) without hardcoding this name a second
// time.
const ServiceAccountName = "cloudflared"
