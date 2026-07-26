package lumenetescontroller

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/cilium"
	"github.com/liamawhite/homelab/pkg/components/prometheus"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	monitoringv1 "github.com/liamawhite/homelab/pkg/crds/prometheus/crds/kubernetes/monitoring/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const metricsPort = 8080

// ambientHBONEPort mirrors pkg/components/prometheus/network.go's identical
// constant (unexported there, so redeclared here rather than imported) -
// this pod is ambient-enrolled (the "lumenetes" namespace carries
// istio.io/dataplane-mode: ambient, see pkg/deploy/namespaces.go, and this
// Deployment's pod has no override), so ztunnel intercepts and
// HBONE-tunnels any inbound connection before it ever reaches metricsPort.
// The Cilium CCNP below must allow port 15008, not 8080, or the connection
// times out with ztunnel logging "maybe a NetworkPolicy is blocking HBONE
// port 15008" - confirmed live this exact reasoning applying to
// Grafana->Prometheus and Prometheus->KSM/Alertmanager traffic.
const ambientHBONEPort = "15008"

// newMetricsScrapeTarget creates the PodMonitor Prometheus needs to
// discover this pod's /metrics, plus the HBONE CiliumClusterwideNetworkPolicy
// egress/ingress pair letting Prometheus's own ambient-enrolled pod
// actually reach it under Cilium's default-deny baseline. Requires the
// Cilium CiliumClusterwideNetworkPolicy and Prometheus PodMonitor CRDs to
// already exist - callers must pass pulumi.DependsOn on both.
func newMetricsScrapeTarget(ctx *pulumi.Context, name string, namespace, prometheusNamespace pulumi.StringInput, opts ...pulumi.ResourceOption) error {
	targetSelector := pulumi.StringMap{"app": pulumi.String("lumenetes-controller")}

	_, err := monitoringv1.NewPodMonitor(ctx, fmt.Sprintf("%s-metrics-monitor", name), &monitoringv1.PodMonitorArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("lumenetes-controller"),
			Namespace: namespace,
		},
		Spec: &monitoringv1.PodMonitorSpecArgs{
			NamespaceSelector: &monitoringv1.PodMonitorSpecNamespaceSelectorArgs{
				MatchNames: pulumi.StringArray{namespace},
			},
			// Real Kubernetes pod labels only - deliberately NOT the same
			// map used for Cilium's EndpointSelector below (Cilium's
			// k8s:io.kubernetes.pod.namespace pseudo-label isn't a real pod
			// label and gets a PodMonitor silently rejected by the
			// Prometheus Operator - see pkg/components/istio/monitoring.go's
			// identical comment for the exact error this produces).
			Selector: &monitoringv1.PodMonitorSpecSelectorArgs{
				MatchLabels: targetSelector,
			},
			PodMetricsEndpoints: monitoringv1.PodMonitorSpecPodMetricsEndpointsArray{
				&monitoringv1.PodMonitorSpecPodMetricsEndpointsArgs{
					Port: pulumi.String("metrics"),
					Path: pulumi.String("/metrics"),
				},
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	promSelector := pulumi.StringMap{
		cilium.K8sNamespaceLabel: prometheusNamespace,
		"app.kubernetes.io/name": pulumi.String(prometheus.PrometheusPodLabel),
	}
	ciliumTargetSelector := pulumi.StringMap{
		cilium.K8sNamespaceLabel: namespace,
		"app":                    pulumi.String("lumenetes-controller"),
	}

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-prometheus-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-prometheus-to-lumenetes-controller-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{MatchLabels: promSelector},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{MatchLabels: ciliumTargetSelector},
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
			Name: pulumi.String("allow-ingress-lumenetes-controller-from-prometheus-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{MatchLabels: ciliumTargetSelector},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{MatchLabels: promSelector},
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
