package longhorn

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/cilium"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ambientHBONEPort is ztunnel's HBONE mTLS tunnel port. Confirmed live via
// `cilium monitor --type drop`: with the namespace ambient-enrolled,
// ztunnel transparently intercepts and mTLS-tunnels ALL of every pod's
// traffic - not just traffic destined through a dedicated waypoint (like
// the UI's) - so even a plain intra-namespace call (e.g.
// longhorn-driver-deployer polling longhorn-manager's own API) actually
// lands on the destination pod's own HBONE port and is blocked by
// default-deny there unless explicitly allowed.
const ambientHBONEPort = "15008"

// newNetworkPolicy creates the CiliumClusterwideNetworkPolicy resources
// Longhorn needs under pkg/components/cilium's default-deny baseline,
// beyond what the UI's own waypoint ingress already covers (see
// pkg/components/istio/waypoint and pkg/components/tailscale/ingress):
//
//  1. apiserver access - longhorn-manager watches its own CRDs/Nodes
//     directly, and longhorn-driver-deployer sets up the CSI driver
//     objects, both requiring the Kubernetes API under Cilium's
//     default-deny baseline the same as any other apiserver consumer.
//  2. Intra-namespace HBONE traffic (ambientHBONEPort) - see that
//     constant's doc comment for why every ambient-enrolled namespace
//     with real pod-to-pod traffic needs this, not just a
//     waypoint-fronted Service.
//
// Requires the Cilium CiliumClusterwideNetworkPolicy CRD to already exist
// - callers must pass pulumi.DependsOn on the Cilium installation.
func newNetworkPolicy(ctx *pulumi.Context, name string, namespace pulumi.StringInput, opts ...pulumi.ResourceOption) error {
	namespaceSelector := &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
		MatchLabels: pulumi.StringMap{
			cilium.K8sNamespaceLabel: namespace,
		},
	}

	_, err := ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-apiserver", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-longhorn-apiserver"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: namespaceSelector,
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

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-longhorn-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: namespaceSelector,
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{
							MatchLabels: pulumi.StringMap{
								cilium.K8sNamespaceLabel: namespace,
							},
						},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String(ambientHBONEPort), Protocol: pulumi.String("TCP")},
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

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-longhorn-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: namespaceSelector,
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{
							MatchLabels: pulumi.StringMap{
								cilium.K8sNamespaceLabel: namespace,
							},
						},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String(ambientHBONEPort), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, opts...)
	return err
}
