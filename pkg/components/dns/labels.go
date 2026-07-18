package dns

// k8sNamespaceLabel is Cilium's reserved label key for matching a pod's
// Kubernetes namespace via its own identity labels, used in endpoint
// selectors instead of a Namespace field (Cilium's endpointSelector has no
// separate namespace concept - namespace is just another label to match on,
// same as any other).
const k8sNamespaceLabel = "k8s:io.kubernetes.pod.namespace"

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
