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
