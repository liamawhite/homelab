package hubcontroller

import (
	"fmt"

	monitoringv1 "github.com/liamawhite/homelab/pkg/crds/prometheus/crds/kubernetes/monitoring/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const metricsPort = 8080

// newMetricsPodMonitor creates only the PodMonitor Prometheus needs to
// discover this pod's /metrics - deliberately NO CiliumClusterwideNetworkPolicy,
// unlike pkg/components/lumenetescontroller's identical-looking metrics
// wiring: this Deployment runs with HostNetwork: true (see this package's
// doc comment) and is therefore not subject to Cilium's default-deny
// baseline at all - same precedent as pkg/components/kubevip, which also
// has zero network-policy wiring for the same reason (this cluster never
// enabled Cilium's host-firewall feature). Requires the Prometheus
// PodMonitor CRD to already exist - callers must pass pulumi.DependsOn on
// it.
func newMetricsPodMonitor(ctx *pulumi.Context, name string, namespace pulumi.StringInput, opts ...pulumi.ResourceOption) error {
	_, err := monitoringv1.NewPodMonitor(ctx, fmt.Sprintf("%s-metrics-monitor", name), &monitoringv1.PodMonitorArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("hub-controller"),
			Namespace: namespace,
		},
		Spec: &monitoringv1.PodMonitorSpecArgs{
			NamespaceSelector: &monitoringv1.PodMonitorSpecNamespaceSelectorArgs{
				MatchNames: pulumi.StringArray{namespace},
			},
			Selector: &monitoringv1.PodMonitorSpecSelectorArgs{
				MatchLabels: pulumi.StringMap{"app": pulumi.String("hub-controller")},
			},
			PodMetricsEndpoints: monitoringv1.PodMonitorSpecPodMetricsEndpointsArray{
				&monitoringv1.PodMonitorSpecPodMetricsEndpointsArgs{
					Port: pulumi.String("metrics"),
					Path: pulumi.String("/metrics"),
				},
			},
		},
	}, opts...)
	return err
}
