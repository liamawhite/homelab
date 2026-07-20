package lightscontroller

// AccessLabelKey and AccessLabelValue together form the pod label the
// controller carries to get egress access to the Hue bridge(s) on the LAN
// - see network.go's allow-egress-hue-lan CiliumClusterwideNetworkPolicy.
const (
	AccessLabelKey   = "network.homelab.io/lights-controller"
	AccessLabelValue = "true"
)
