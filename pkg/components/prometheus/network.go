package prometheus

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	"github.com/liamawhite/homelab/pkg/components/cilium"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ambientHBONEPort is ztunnel's HBONE mTLS tunnel port - see
// pkg/components/longhorn/network.go's identical constant for why every
// ambient-enrolled namespace with real pod-to-pod traffic needs egress/
// ingress CCNPs on this port rather than the destination's real container
// port.
const ambientHBONEPort = "15008"

// PrometheusPodLabel is the "app.kubernetes.io/name" value instance.go's
// PodMetadata.Labels stamps onto every pod the operator generates from the
// Prometheus CR - exported so other packages (e.g. pkg/components/grafana's
// network.go, for its own Grafana<->Prometheus HBONE CCNP pair) can match
// Prometheus's pods without needing to know any other internal detail of
// this package.
const PrometheusPodLabel = "prometheus"

// prometheusPodSelector matches Prometheus's own pod, via the
// app.kubernetes.io/name label instance.go's PodMetadata.Labels sets -
// deliberately not the operator's own implicit "prometheus: <name>" label,
// so this only ever depends on a label this package itself controls.
func prometheusPodSelector(namespace pulumi.StringInput) *ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs {
	return &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
		MatchLabels: pulumi.StringMap{
			cilium.K8sNamespaceLabel: namespace,
			"app.kubernetes.io/name": pulumi.String(PrometheusPodLabel),
		},
	}
}

// newNetworkPolicy creates the CiliumClusterwideNetworkPolicy resources this
// stack needs under pkg/components/cilium's default-deny baseline, beyond
// what apiserver.AccessLabelKey/dns.AccessLabelKey already cover (see each
// workload's own pod-label comment):
//
//  1. Prometheus -> node-exporter and Prometheus -> kube-system's kubelet
//     Service: both destinations are hostNetwork pods, i.e. not
//     Cilium-managed endpoints in their own right, so this is expressed as
//     egress to Cilium's "host"/"remote-node" entities rather than a
//     ToEndpoints selector - the same kind of "not a Cilium endpoint"
//     reasoning pkg/components/lightscontroller's allow-egress-hue-lan CCNP
//     uses for the (Cilium-external) Hue bridge, just with node/host
//     identities instead of "world". Unverified against a live cluster as
//     of writing - check `cilium monitor --type drop` on first deploy in
//     case node/host traffic instead needs a "world"-shaped rule here.
//  2. Prometheus <-> kube-state-metrics and Prometheus <-> Alertmanager:
//     all in the same ambient-enrolled "monitoring" namespace, so - per
//     ambientHBONEPort's reasoning above - these are port-15008 CCNP pairs,
//     not 8080/8081 (KSM) or 9093 (Alertmanager) ones.
//
// Requires the Cilium CiliumClusterwideNetworkPolicy CRD to already exist -
// callers must pass pulumi.DependsOn on the Cilium installation.
func newNetworkPolicy(ctx *pulumi.Context, name string, namespace pulumi.StringInput, opts ...pulumi.ResourceOption) error {
	promSelector := prometheusPodSelector(namespace)

	_, err := ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-node-exporter", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-prometheus-node-exporter"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: promSelector,
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("host"), pulumi.String("remote-node")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("9100"), Protocol: pulumi.String("TCP")},
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

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-kubelet", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-prometheus-kubelet"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: promSelector,
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("host"), pulumi.String("remote-node")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("10250"), Protocol: pulumi.String("TCP")},
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

	ksmSelector := pulumi.StringMap{
		cilium.K8sNamespaceLabel: namespace,
		apiserver.AccessLabelKey: pulumi.String(apiserver.AccessLabelValue),
		"app.kubernetes.io/name": pulumi.String("kube-state-metrics"),
	}

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-ksm-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-prometheus-to-ksm-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: promSelector,
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{MatchLabels: ksmSelector},
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

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-ksm-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-ksm-from-prometheus-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{MatchLabels: ksmSelector},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{MatchLabels: promSelector.MatchLabels},
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
	if err != nil {
		return err
	}

	amSelector := pulumi.StringMap{
		cilium.K8sNamespaceLabel: namespace,
		"app.kubernetes.io/name": pulumi.String(AlertmanagerPodLabel),
	}

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-alertmanager-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-prometheus-to-alertmanager-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: promSelector,
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{MatchLabels: amSelector},
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

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-alertmanager-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-alertmanager-from-prometheus-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{MatchLabels: amSelector},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{MatchLabels: promSelector.MatchLabels},
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
