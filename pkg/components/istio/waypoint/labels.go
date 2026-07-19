package waypoint

// GatewayNameLabel is the label istiod stamps on a waypoint's own pod, set
// to the Gateway's name - a stable, already-unique identity a caller's own
// app-specific network policy can match on directly (see
// pkg/deploy/applications/home.go), with no new label of our own needed.
const GatewayNameLabel = "gateway.networking.k8s.io/gateway-name"
