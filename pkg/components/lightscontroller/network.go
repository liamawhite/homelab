package lightscontroller

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// newNetworkPolicy creates the CiliumClusterwideNetworkPolicy resources
// that let the controller reach the Kubernetes API server (to manage Light
// CRs and its leader-election Lease) and the Hue bridge(s) on the LAN,
// under pkg/components/cilium's default-deny baseline. Requires the Cilium
// CiliumClusterwideNetworkPolicy CRD to already exist - callers must pass
// pulumi.DependsOn on the Cilium installation.
func newNetworkPolicy(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) error {
	// kube-apiserver access - same pattern as every other apiserver
	// consumer in this repo (pkg/components/apiserver.NewClusterAPIServer).
	_, err := ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-apiserver", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-lights-controller-apiserver"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					apiserver.AccessLabelKey: pulumi.String(apiserver.AccessLabelValue),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("kube-apiserver")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("6443"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	// Hue bridge(s) on the LAN aren't Cilium-managed endpoints, so - same
	// reasoning as pkg/components/dns's allow-egress-coredns-upstream and
	// cloudflare/tunnel's allow-egress-cloudflare-tunnel - this is a
	// ToEntities "world" rule restricted by port rather than a fixed CIDR.
	// TCP/80+443 cover the bridge's unauthenticated /api/config, the
	// authenticated CLIP v2 HTTPS API, and N-UPnP's discovery.meethue.com
	// call (deploy.go pins DiscoveryMethod to "nupnp" for this reason).
	//
	// No UDP/1900 (SSDP) rule: confirmed live via `cilium monitor --type
	// drop` that SSDP can't work from a pod on this cluster regardless of
	// policy - its M-SEARCH needs an IGMP multicast group-join first, and
	// Cilium's eBPF datapath drops IGMP outright ("CT: Unknown L4
	// protocol") below the level any CiliumClusterwideNetworkPolicy rule
	// can act on. This was the one risk flagged as unverified in the plan
	// this was built from; it verified false.
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-hue-lan", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-lights-controller-hue-lan"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					AccessLabelKey: pulumi.String(AccessLabelValue),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("world")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("80"), Protocol: pulumi.String("TCP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("443"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, opts...)
	return err
}
