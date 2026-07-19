package dns

// AccessLabelKey and AccessLabelValue together form the pod label a
// workload must carry to get DNS egress access - callers add this to
// their own pod template's Labels (alongside whatever labels that
// workload already sets) to opt in. Exported so every component that
// needs DNS can reference the same key/value rather than each hand-typing
// the string.
const (
	AccessLabelKey   = "network.homelab.io/dns"
	AccessLabelValue = "true"
)
