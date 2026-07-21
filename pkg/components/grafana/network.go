package grafana

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/cilium"
	"github.com/liamawhite/homelab/pkg/components/prometheus"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ambientHBONEPort is ztunnel's HBONE mTLS tunnel port - see
// pkg/components/longhorn/network.go's identical constant for why every
// ambient-enrolled namespace with real pod-to-pod traffic needs egress/
// ingress CCNPs on this port rather than the destination's real container
// port. Grafana's datasource dials http://prometheus-<instance>.<namespace>:9090,
// which - because "monitoring" is ambient-enrolled - actually lands on
// Prometheus's own HBONE listener, not port 9090.
const ambientHBONEPort = "15008"

// newNetworkPolicy creates the CiliumClusterwideNetworkPolicy pair Grafana
// needs to reach Prometheus's own pod, both in the same ambient-enrolled
// "monitoring" namespace. Requires the Cilium CiliumClusterwideNetworkPolicy
// CRD to already exist - callers must pass pulumi.DependsOn on the Cilium
// installation.
func newNetworkPolicy(ctx *pulumi.Context, name string, namespace pulumi.StringInput, opts ...pulumi.ResourceOption) error {
	grafanaSelector := &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
		MatchLabels: pulumi.StringMap{
			cilium.K8sNamespaceLabel: namespace,
			"app":                    pulumi.String("grafana"),
		},
	}
	prometheusMatchLabels := pulumi.StringMap{
		cilium.K8sNamespaceLabel: namespace,
		"app.kubernetes.io/name": pulumi.String(prometheus.PrometheusPodLabel),
	}

	_, err := ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-prometheus-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-grafana-to-prometheus-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: grafanaSelector,
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{MatchLabels: prometheusMatchLabels},
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

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-prometheus-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-prometheus-from-grafana-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{MatchLabels: prometheusMatchLabels},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{MatchLabels: grafanaSelector.MatchLabels},
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
