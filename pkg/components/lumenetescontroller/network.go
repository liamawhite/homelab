package lumenetescontroller

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	securityv1 "github.com/liamawhite/homelab/pkg/crds/istio/crds/kubernetes/security/v1"
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
			Name: pulumi.String("allow-egress-lumenetes-controller-apiserver"),
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
			Name: pulumi.String("allow-egress-lumenetes-controller-hue-lan"),
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
	if err != nil {
		return err
	}

	// kube-apiserver ingress access - the mirror image of the egress rules
	// above: the Light validating webhook (see webhook.go) needs the API
	// server to be able to reach this pod on webhookPort, which the
	// default-deny baseline otherwise blocks (it only allows ingress from
	// "host"/"remote-node", not "kube-apiserver" - see
	// pkg/components/cilium's default-deny policy).
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-webhook-apiserver", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-lumenetes-controller-webhook"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app": pulumi.String("lumenetes-controller"),
				},
			},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEntities: pulumi.StringArray{pulumi.String("kube-apiserver")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String("9443"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, opts...)
	return err
}

// newPeerAuthentication overrides the mesh-wide STRICT PeerAuthentication
// (pkg/components/istio's istio-system/default) down to PERMISSIVE for just
// webhookPort on this workload - the Light validating webhook is called
// directly by kube-apiserver, which has no Istio identity/certificate, so
// mesh-wide STRICT mTLS otherwise has ztunnel reject every admission
// request outright. Confirmed live: ztunnel logged "connection closed due
// to policy rejection: explicitly denied by: istio-system/istio_converted_static_strict"
// for every webhook call crossing nodes (same-node calls were unaffected,
// which is what made this look like a network/MTU bug for a long time
// before ztunnel's own access log surfaced the real cause). Port-level
// mTLS is additive per Istio semantics - this doesn't touch or need to
// alias the mesh-wide policy.
func newPeerAuthentication(ctx *pulumi.Context, name string, namespace pulumi.StringInput, opts ...pulumi.ResourceOption) error {
	_, err := securityv1.NewPeerAuthentication(ctx, fmt.Sprintf("%s-webhook-permissive-mtls", name), &securityv1.PeerAuthenticationArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("lumenetes-controller-webhook-permissive"),
			Namespace: namespace,
		},
		Spec: &securityv1.PeerAuthenticationSpecArgs{
			Selector: &securityv1.PeerAuthenticationSpecSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app": pulumi.String("lumenetes-controller"),
				},
			},
			Mtls: &securityv1.PeerAuthenticationSpecMtlsArgs{
				Mode: pulumi.String("STRICT"),
			},
			PortLevelMtls: pulumi.StringMapMap{
				fmt.Sprintf("%d", webhookPort): pulumi.StringMap{
					"mode": pulumi.String("PERMISSIVE"),
				},
			},
		},
	}, opts...)
	return err
}
