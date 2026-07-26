package grafana

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/cilium"
	"github.com/liamawhite/homelab/pkg/components/istio/waypoint"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ambientHBONEPort is ztunnel's HBONE mTLS tunnel port - see
// pkg/components/longhorn/network.go's identical constant for why every
// ambient-enrolled namespace with real pod-to-pod traffic needs egress/
// ingress CCNPs on this port. Grafana's datasource dials
// http://prometheus-<instance>.<namespace>:9090, but since that Service
// carries istio.io/use-waypoint, ambient always redirects the connection to
// Prometheus's own waypoint pod first - never straight to Prometheus's real
// pod - so the destination this policy actually has to allow is the
// waypoint's HBONE listener, not Prometheus's.
const ambientHBONEPort = "15008"

// newNetworkPolicy creates the CiliumClusterwideNetworkPolicy pair Grafana's
// pod needs to reach Prometheus's own waypoint pod at all. This is on top
// of - not a replacement for - the Istio AuthorizationPolicy bypass
// component.go's ingress.NewIngress call grants Grafana's ServiceAccount:
// Cilium's baseline only lets Tailscale's own proxy pods reach any
// waypoint's HBONE port in the first place (see
// pkg/components/tailscale's allow-{e,in}gress-tailscale-waypoints
// policies), so without this pair Grafana's connection times out before it
// ever reaches the point where that AuthorizationPolicy would even be
// evaluated - confirmed live via ztunnel's own access log ("connection
// timed out, maybe a NetworkPolicy is blocking HBONE port 15008").
// Requires the Cilium CiliumClusterwideNetworkPolicy CRD to already exist -
// callers must pass pulumi.DependsOn on the Cilium installation.
func newNetworkPolicy(ctx *pulumi.Context, name string, namespace, prometheusWaypointName pulumi.StringInput, opts ...pulumi.ResourceOption) error {
	grafanaSelector := &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
		MatchLabels: pulumi.StringMap{
			cilium.K8sNamespaceLabel: namespace,
			"app":                    pulumi.String("grafana"),
		},
	}
	waypointMatchLabels := pulumi.StringMap{
		cilium.K8sNamespaceLabel:  namespace,
		waypoint.GatewayNameLabel: prometheusWaypointName,
	}

	_, err := ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-prometheus-waypoint-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-grafana-to-prometheus-waypoint-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: grafanaSelector,
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{MatchLabels: waypointMatchLabels},
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

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-prometheus-waypoint-hbone", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-prometheus-waypoint-from-grafana-hbone"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{MatchLabels: waypointMatchLabels},
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
