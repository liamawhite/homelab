// Package dns provides the CiliumClusterwideNetworkPolicy resources that
// let cluster DNS keep working under pkg/components/cilium's default-deny
// baseline - split out from that package so each control-plane dependency's
// network policy lives with the concern it enables, rather than all being
// lumped into the CNI installer itself.
package dns

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ClusterDNS represents the cluster's CoreDNS network policy baseline.
type ClusterDNS struct {
	pulumi.ResourceState
}

// NewClusterDNS creates the CiliumClusterwideNetworkPolicy resources that allow
// DNS to keep working once egress/ingress default-deny is in effect
// cluster-wide: every pod can reach CoreDNS, CoreDNS accepts from
// everywhere, and CoreDNS can reach its own upstream resolver. Callers must
// pass pulumi.DependsOn on the Cilium installation (see
// pkg/components/cilium.NewCilium) so the CiliumClusterwideNetworkPolicy
// CRD Cilium's Helm chart installs already exists before these are applied.
func NewClusterDNS(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*ClusterDNS, error) {
	d := &ClusterDNS{}

	err := ctx.RegisterComponentResource("homelab:kubernetes:cluster-dns", name, d, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(d))

	// Only pods carrying AccessLabelKey/AccessLabelValue can reach CoreDNS -
	// DNS access is opt-in per workload, not blanket, so every consumer is
	// explicit about depending on it.
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-dns", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-dns"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					AccessLabelKey: pulumi.String(AccessLabelValue),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{
							MatchLabels: pulumi.StringMap{
								k8sNamespaceLabel: pulumi.String("kube-system"),
								"k8s-app":         pulumi.String("kube-dns"),
							},
						},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("53"), Protocol: pulumi.String("UDP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("53"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-dns", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-dns"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					k8sNamespaceLabel: pulumi.String("kube-system"),
					"k8s-app":         pulumi.String("kube-dns"),
				},
			},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String("53"), Protocol: pulumi.String("UDP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String("53"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// CoreDNS's own egress for its upstream forwarder. CoreDNS's Corefile
	// (K3s's default) has "forward . /etc/resolv.conf" - anything not
	// authoritatively answered by the "kubernetes" plugin (i.e. anything
	// outside cluster.local/in-addr.arpa/ip6.arpa) gets forwarded to the
	// node's own configured upstream resolver, an address outside the
	// cluster that isn't expressible as a fixed CIDR (whatever the node's
	// /etc/resolv.conf says).
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-coredns-upstream", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-coredns-upstream"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					k8sNamespaceLabel: pulumi.String("kube-system"),
					"k8s-app":         pulumi.String("kube-dns"),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("world")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("53"), Protocol: pulumi.String("UDP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("53"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// CoreDNS's own "kubernetes" plugin watches the K8s API directly (to
	// serve Service/Endpoint records) - needs apiserver access under
	// Cilium's default-deny egress baseline, same as any other apiserver
	// consumer. CoreDNS itself is a K3s-provided system add-on, not a
	// resource this repo's Pulumi program creates, so a Patch resource
	// (Server-Side Apply) is used to add just this one label to its pod
	// template rather than adopting/owning the whole Deployment.
	_, err = appsv1.NewDeploymentPatch(ctx, fmt.Sprintf("%s-coredns-apiserver-access", name), &appsv1.DeploymentPatchArgs{
		Metadata: &metav1.ObjectMetaPatchArgs{
			Name:      pulumi.String("coredns"),
			Namespace: pulumi.String("kube-system"),
		},
		Spec: &appsv1.DeploymentSpecPatchArgs{
			Template: &corev1.PodTemplateSpecPatchArgs{
				Metadata: &metav1.ObjectMetaPatchArgs{
					Labels: pulumi.StringMap{
						apiserver.AccessLabelKey: pulumi.String(apiserver.AccessLabelValue),
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	if err := ctx.RegisterResourceOutputs(d, pulumi.Map{}); err != nil {
		return nil, err
	}

	return d, nil
}
