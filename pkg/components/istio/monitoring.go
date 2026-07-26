package istio

import (
	"fmt"
	"maps"

	"github.com/liamawhite/homelab/pkg/components/cilium"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	monitoringv1 "github.com/liamawhite/homelab/pkg/crds/prometheus/crds/kubernetes/monitoring/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// waypointGatewayClassLabel is the label istiod stamps on every ambient
// waypoint pod it provisions, cluster-wide, regardless of which app's
// Service that particular waypoint fronts (unlike
// pkg/components/istio/waypoint.GatewayNameLabel, which identifies one
// specific waypoint) - matching on it lets a single PodMonitor/CCNP pair
// cover every waypoint present and future, without each
// pkg/components/istio/waypoint.NewWaypoint call needing its own.
const waypointGatewayClassLabel = "gateway.networking.k8s.io/gateway-class-name"
const waypointGatewayClassValue = "istio-waypoint"

// prometheusPodLabelKey/Value match pkg/components/prometheus.PrometheusPodLabel's
// key/value - hardcoded here rather than imported to avoid an import cycle
// (pkg/components/prometheus imports pkg/components/istio/waypoint, which
// imports this package).
const prometheusPodLabelKey = "app.kubernetes.io/name"
const prometheusPodLabelValue = "prometheus"

// scrapeTargetArgs describes one Prometheus scrape target newMonitoring
// exposes - istiod, ztunnel, and every waypoint's Envoy each need almost
// the same PodMonitor + CiliumClusterwideNetworkPolicy egress/ingress pair,
// differing only in these fields.
type scrapeTargetArgs struct {
	// TargetNamespace is the scraped pod's own namespace - a fixed literal
	// (istiod/ztunnel both live in istio-system). Leave nil (namespaceAny
	// true) for a target that can live in any namespace, like waypoints.
	TargetNamespace pulumi.StringInput
	NamespaceAny    bool
	// PodLabels are the target's real Kubernetes pod labels (e.g.
	// {"app": "istiod"}) - used directly as PodMonitor's own Selector.
	// Deliberately NOT the same map passed to Cilium's EndpointSelector
	// below: Cilium's k8s:io.kubernetes.pod.namespace pseudo-label isn't a
	// real pod label, and the Prometheus Operator rejects a PodMonitor
	// whose selector contains it outright ("failed to parse label
	// selector... name part must consist of alphanumeric characters" -
	// confirmed live, this exact mistake silently dropped the istiod and
	// ztunnel PodMonitors while the (correctly namespace-unconstrained)
	// waypoint one worked fine).
	PodLabels pulumi.StringMap
	Port      string
	Path      string
}

// newScrapeTarget creates the PodMonitor and Cilium egress/ingress CCNP
// pair letting Prometheus (identified by prometheusSelector, built once by
// newMonitoring) reach one mesh component's metrics endpoint under
// Cilium's default-deny baseline. podMonitorNamespace is only where the
// PodMonitor object itself lives - irrelevant to which pods it actually
// discovers, which args.NamespaceSelector controls.
// Requires the Cilium CiliumClusterwideNetworkPolicy and Prometheus
// PodMonitor CRDs to already exist - callers must pass pulumi.DependsOn on
// both (see pkg/components/cilium.NewCilium and pkg/crds/prometheus).
func newScrapeTarget(ctx *pulumi.Context, name string, podMonitorNamespace pulumi.StringInput, prometheusSelector pulumi.StringMap, args scrapeTargetArgs, opts ...pulumi.ResourceOption) error {
	nsSelector := &monitoringv1.PodMonitorSpecNamespaceSelectorArgs{}
	if args.NamespaceAny {
		nsSelector.Any = pulumi.Bool(true)
	} else {
		nsSelector.MatchNames = pulumi.StringArray{args.TargetNamespace}
	}

	_, err := monitoringv1.NewPodMonitor(ctx, name, &monitoringv1.PodMonitorArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: podMonitorNamespace,
		},
		Spec: &monitoringv1.PodMonitorSpecArgs{
			NamespaceSelector: nsSelector,
			Selector: &monitoringv1.PodMonitorSpecSelectorArgs{
				MatchLabels: args.PodLabels,
			},
			PodMetricsEndpoints: monitoringv1.PodMonitorSpecPodMetricsEndpointsArray{
				&monitoringv1.PodMonitorSpecPodMetricsEndpointsArgs{
					Port: pulumi.String(args.Port),
					Path: pulumi.String(args.Path),
				},
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	// Cilium's own endpoint selector, unlike PodMonitor's Selector above,
	// legitimately uses the k8s:io.kubernetes.pod.namespace pseudo-label to
	// scope by namespace (Cilium's own convention, not a real pod label) -
	// added on top of args.PodLabels only when the target lives in a fixed
	// namespace; the cluster-wide waypoint target's Cilium selector stays
	// namespace-unconstrained too, matching its PodMonitor.
	ciliumMatchLabels := args.PodLabels
	if !args.NamespaceAny {
		ciliumMatchLabels = pulumi.StringMap{cilium.K8sNamespaceLabel: args.TargetNamespace}
		maps.Copy(ciliumMatchLabels, args.PodLabels)
	}

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(fmt.Sprintf("allow-egress-prometheus-to-%s", name)),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: prometheusSelector,
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{MatchLabels: ciliumMatchLabels},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String(portToNumber[args.Port]), Protocol: pulumi.String("TCP")},
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

	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(fmt.Sprintf("allow-ingress-%s-from-prometheus", name)),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: ciliumMatchLabels,
			},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{MatchLabels: prometheusSelector},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String(portToNumber[args.Port]), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, opts...)
	return err
}

// portToNumber maps each scrape target's named container port to its
// numeric value - Cilium's ToPorts/FromPorts need the real number, not the
// Kubernetes port name PodMonitor's own Port field uses.
var portToNumber = map[string]string{
	"http-monitoring": "15014", // istiod's own control-plane metrics
	"ztunnel-stats":   "15020", // ztunnel's ambient dataplane metrics
	"http-envoy-prom": "15090", // every waypoint's Envoy L7 metrics
}

// newMonitoring wires Prometheus scraping for the mesh's own control and
// data plane: istiod (control-plane pilot_* metrics), ztunnel (ambient
// dataplane istio_tcp_*/dns_* metrics, one pod per node), and every
// waypoint's Envoy (istio_requests_total and friends, for whatever L7
// traffic actually routes through a waypoint - see
// pkg/components/istio/waypoint's own doc comment for what creates one).
// None of these are on the AccessLabelKey allowlist model that governs
// istiod-consumer access above (that's mesh components depending on
// istiod; this is the reverse - Prometheus reading from the mesh's own
// components), so each gets its own CCNP pair via newScrapeTarget.
//
// prometheusNamespace identifies where Prometheus's own pod runs -
// pkg/components/prometheus isn't imported directly (see
// prometheusPodLabelKey's doc comment for why), so the caller (
// pkg/deploy/deploy.go, which already imports both) passes this through.
// Requires the Cilium CiliumClusterwideNetworkPolicy and Prometheus
// PodMonitor CRDs to already exist - callers must pass pulumi.DependsOn on
// both.
func newMonitoring(ctx *pulumi.Context, name string, namespace, prometheusNamespace pulumi.StringInput, opts ...pulumi.ResourceOption) error {
	prometheusSelector := pulumi.StringMap{
		cilium.K8sNamespaceLabel: prometheusNamespace,
		prometheusPodLabelKey:    pulumi.String(prometheusPodLabelValue),
	}

	if err := newScrapeTarget(ctx, fmt.Sprintf("%s-istiod-monitor", name), namespace, prometheusSelector, scrapeTargetArgs{
		TargetNamespace: namespace,
		PodLabels:       pulumi.StringMap{"app": pulumi.String("istiod")},
		Port:            "http-monitoring",
		Path:            "/metrics",
	}, opts...); err != nil {
		return err
	}

	if err := newScrapeTarget(ctx, fmt.Sprintf("%s-ztunnel-monitor", name), namespace, prometheusSelector, scrapeTargetArgs{
		TargetNamespace: namespace,
		PodLabels:       pulumi.StringMap{"app": pulumi.String("ztunnel")},
		Port:            "ztunnel-stats",
		Path:            "/stats/prometheus",
	}, opts...); err != nil {
		return err
	}

	if err := newScrapeTarget(ctx, fmt.Sprintf("%s-waypoint-monitor", name), namespace, prometheusSelector, scrapeTargetArgs{
		NamespaceAny: true,
		PodLabels:    pulumi.StringMap{waypointGatewayClassLabel: pulumi.String(waypointGatewayClassValue)},
		Port:         "http-envoy-prom",
		Path:         "/stats/prometheus",
	}, opts...); err != nil {
		return err
	}

	return nil
}
