package cilium

// K8sNamespaceLabel is Cilium's reserved label key for matching a pod's
// Kubernetes namespace via its own identity labels, used in endpoint
// selectors instead of a Namespace field (Cilium's endpointSelector has no
// separate namespace concept - namespace is just another label to match on,
// same as any other). Exported so every component writing a
// CiliumClusterwideNetworkPolicy references the same key rather than each
// hand-typing (and duplicating) the string.
const K8sNamespaceLabel = "k8s:io.kubernetes.pod.namespace"
