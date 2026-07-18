package istio

// k8sNamespaceLabel is Cilium's reserved label key for matching a pod's
// Kubernetes namespace via its own identity labels, used in endpoint
// selectors instead of a Namespace field (Cilium's endpointSelector has no
// separate namespace concept - namespace is just another label to match on,
// same as any other).
const k8sNamespaceLabel = "k8s:io.kubernetes.pod.namespace"

// AccessLabelKey and AccessLabelValue together form the pod label a
// workload must carry to get egress access to istiod - callers add this to
// their own pod template's Labels (alongside whatever labels that workload
// already sets) to opt in. Exported so every component that needs istiod
// (ztunnel, istio-cni, waypoints, gateways) can reference the same
// key/value rather than each hand-typing the string.
const (
	AccessLabelKey   = "network.homelab.io/istiod"
	AccessLabelValue = "true"
)

// DataplaneModeLabelKey is Istio ambient's own reserved label key
// controlling whether ztunnel transparently captures a pod's traffic for
// the ambient mesh data plane. DataplaneModeAmbient enrolls a namespace/pod
// (see pkg/deploy/namespaces.go); DataplaneModeNone opts a specific pod out
// even within an otherwise-ambient namespace - e.g. this mesh's own control
// plane components (istiod, ztunnel, istio-cni) and infra components like
// cloudflared whose own traffic shouldn't be intercepted (confirmed live:
// leaving cloudflared ambient-captured caused ztunnel to silently swallow
// kubelet's plain-HTTP liveness probe, which expects a direct connection,
// not HBONE).
const (
	DataplaneModeLabelKey = "istio.io/dataplane-mode"
	DataplaneModeAmbient  = "ambient"
	DataplaneModeNone     = "none"
)
