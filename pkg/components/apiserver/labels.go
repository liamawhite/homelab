package apiserver

// AccessLabelKey and AccessLabelValue together form the pod label a
// workload must carry to get egress access to the Kubernetes API server -
// callers add this to their own pod template's Labels (alongside whatever
// labels that workload already sets) to opt in. Exported so every
// component that needs the API server (istiod, metrics-server,
// local-path-provisioner, and anything else that watches/reads the K8s API
// directly) can reference the same key/value rather than each hand-typing
// the string.
const (
	AccessLabelKey   = "network.homelab.io/apiserver"
	AccessLabelValue = "true"
)
