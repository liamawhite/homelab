package prometheus

import (
	monitoringv1 "github.com/liamawhite/homelab/pkg/crds/prometheus/crds/kubernetes/monitoring/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// newCadvisorServiceMonitor creates only a ServiceMonitor - no workload of
// its own - targeting the synthetic "kubelet" Service in kube-system that
// prometheus-operator itself creates/reconciles because of operator.go's
// "--kubelet-service=kube-system/kubelet" flag (see newOperator's
// ClusterRole for the get/create/update/delete grant on services/endpoints
// that Service needs). Container-level metrics come from the kubelet's own
// built-in cAdvisor, not a separate cadvisor Deployment.
func newCadvisorServiceMonitor(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) error {
	_, err := monitoringv1.NewServiceMonitor(ctx, name, &monitoringv1.ServiceMonitorArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("cadvisor"),
		},
		Spec: &monitoringv1.ServiceMonitorSpecArgs{
			JobLabel: pulumi.String("app.kubernetes.io/name"),
			NamespaceSelector: &monitoringv1.ServiceMonitorSpecNamespaceSelectorArgs{
				MatchNames: pulumi.StringArray{pulumi.String("kube-system")},
			},
			Selector: &monitoringv1.ServiceMonitorSpecSelectorArgs{
				MatchLabels: pulumi.StringMap{"app.kubernetes.io/name": pulumi.String("kubelet")},
			},
			Endpoints: monitoringv1.ServiceMonitorSpecEndpointsArray{
				// Kubelet's own /metrics.
				&monitoringv1.ServiceMonitorSpecEndpointsArgs{
					Port:            pulumi.String("https-metrics"),
					Scheme:          pulumi.String("https"),
					Interval:        pulumi.String("30s"),
					HonorLabels:     pulumi.Bool(true),
					BearerTokenFile: pulumi.String("/var/run/secrets/kubernetes.io/serviceaccount/token"),
					TlsConfig:       &monitoringv1.ServiceMonitorSpecEndpointsTlsConfigArgs{InsecureSkipVerify: pulumi.Bool(true)},
					Relabelings: monitoringv1.ServiceMonitorSpecEndpointsRelabelingsArray{
						&monitoringv1.ServiceMonitorSpecEndpointsRelabelingsArgs{Action: pulumi.String("labelmap"), Regex: pulumi.String("__meta_kubernetes_node_label_(.+)")},
					},
					MetricRelabelings: monitoringv1.ServiceMonitorSpecEndpointsMetricRelabelingsArray{
						&monitoringv1.ServiceMonitorSpecEndpointsMetricRelabelingsArgs{
							Action:       pulumi.String("drop"),
							Regex:        pulumi.String("container_(network_tcp_usage_total|network_udp_usage_total|tasks_state|cpu_load_average_10s)"),
							SourceLabels: pulumi.StringArray{pulumi.String("__name__")},
						},
					},
				},
				// cAdvisor's own container metrics, embedded in the kubelet
				// under /metrics/cadvisor.
				&monitoringv1.ServiceMonitorSpecEndpointsArgs{
					Port:            pulumi.String("https-metrics"),
					Path:            pulumi.String("/metrics/cadvisor"),
					Scheme:          pulumi.String("https"),
					Interval:        pulumi.String("30s"),
					HonorLabels:     pulumi.Bool(true),
					BearerTokenFile: pulumi.String("/var/run/secrets/kubernetes.io/serviceaccount/token"),
					TlsConfig:       &monitoringv1.ServiceMonitorSpecEndpointsTlsConfigArgs{InsecureSkipVerify: pulumi.Bool(true)},
					Relabelings: monitoringv1.ServiceMonitorSpecEndpointsRelabelingsArray{
						&monitoringv1.ServiceMonitorSpecEndpointsRelabelingsArgs{Action: pulumi.String("labelmap"), Regex: pulumi.String("__meta_kubernetes_node_label_(.+)")},
					},
					// Drops the highest-cardinality/least useful container
					// metrics - same set _migrateme's Cadvisor component
					// dropped.
					MetricRelabelings: monitoringv1.ServiceMonitorSpecEndpointsMetricRelabelingsArray{
						&monitoringv1.ServiceMonitorSpecEndpointsMetricRelabelingsArgs{
							Action:       pulumi.String("drop"),
							Regex:        pulumi.String("container_(.*_seconds_total|cpu_cfs_throttled_periods_total|cpu_cfs_periods_total|tasks_state|memory_failures_total|spec_.*)"),
							SourceLabels: pulumi.StringArray{pulumi.String("__name__")},
						},
					},
				},
			},
		},
	}, opts...)
	return err
}
