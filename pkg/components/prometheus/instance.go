package prometheus

import (
	"fmt"
	"maps"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	monitoringv1 "github.com/liamawhite/homelab/pkg/crds/prometheus/crds/kubernetes/monitoring/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	instanceName    = "k8s"
	prometheusImage = "quay.io/prometheus/prometheus"

	// ServiceName is the fixed name of the Service newInstance creates for
	// the Prometheus CR - exported so callers (pkg/deploy/deploy.go, to
	// wire pkg/components/grafana's datasource at
	// <ServiceName>.<namespace>:9090) can reference it without needing an
	// extra component output plumbed through.
	ServiceName = "prometheus-" + instanceName
)

// instanceArgs bundles the knobs newInstance needs beyond the operator's own
// (namespace, image version, and this stack's data retention strategy).
type instanceArgs struct {
	Namespace        pulumi.StringInput
	Version          string
	StorageClassName pulumi.StringInput
	StorageSize      string
	Retention        string
	RetentionSize    string
	// WaypointName routes this Service through Prometheus's own dedicated
	// waypoint (component.go creates it) - we own this Service, so the
	// label is set directly at creation, same as Grafana's Service.
	WaypointName pulumi.StringOutput
}

// newInstance creates the Prometheus custom resource itself (reconciled by
// the operator into a StatefulSet), its own ServiceAccount/ClusterRole
// (Kubernetes service-discovery needs direct API read access - nodes,
// services, endpoints, pods - independent of the operator's own RBAC), and a
// stable Service to reach it at (used by Grafana's datasource and, if ever
// added, an external ServiceMonitor scraping Prometheus's own /metrics).
//
// Retention/RetentionSize together are this stack's data retention
// strategy (see the package doc comment): time-based retention alone can
// let a cardinality spike fill the PVC to 100% between GC cycles and
// crash-loop Prometheus, so a size cap is set too, letting Prometheus
// proactively compact away old blocks the moment *either* limit is hit.
func newInstance(ctx *pulumi.Context, name string, args instanceArgs, opts ...pulumi.ResourceOption) (*monitoringv1.Prometheus, *corev1.Service, error) {
	labels := pulumi.StringMap{
		"app.kubernetes.io/name":      pulumi.String("prometheus"),
		"app.kubernetes.io/instance":  pulumi.String(instanceName),
		"app.kubernetes.io/component": pulumi.String("prometheus"),
		"app.kubernetes.io/part-of":   pulumi.String("kube-prometheus"),
		"app.kubernetes.io/version":   pulumi.String(args.Version),
	}

	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fmt.Sprintf("prometheus-%s", instanceName)),
			Namespace: args.Namespace,
		},
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	// Prometheus's own Kubernetes service-discovery watches nodes/services/
	// endpoints/pods directly (independent of anything the operator's own
	// RBAC grants it) - mirrors
	// _migrateme/components/kubernetes/prometheus's "prometheus-k8s"
	// ClusterRole verbatim.
	clusterRole, err := rbacv1.NewClusterRole(ctx, fmt.Sprintf("%s-cr", name), &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(fmt.Sprintf("prometheus-%s", instanceName)),
		},
		Rules: rbacv1.PolicyRuleArray{
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{
					pulumi.String("nodes"), pulumi.String("nodes/metrics"), pulumi.String("services"),
					pulumi.String("endpoints"), pulumi.String("pods"),
				},
				Verbs: pulumi.StringArray{pulumi.String("get"), pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("configmaps")},
				Verbs:     pulumi.StringArray{pulumi.String("get")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("networking.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("ingresses")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				NonResourceURLs: pulumi.StringArray{pulumi.String("/metrics"), pulumi.String("/metrics/slis")},
				Verbs:           pulumi.StringArray{pulumi.String("get")},
			},
		},
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	_, err = rbacv1.NewClusterRoleBinding(ctx, fmt.Sprintf("%s-crb", name), &rbacv1.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(fmt.Sprintf("prometheus-%s", instanceName)),
		},
		RoleRef: &rbacv1.RoleRefArgs{
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			Kind:     pulumi.String("ClusterRole"),
			Name:     clusterRole.Metadata.Name().Elem(),
		},
		Subjects: rbacv1.SubjectArray{
			&rbacv1.SubjectArgs{
				Kind:      pulumi.String("ServiceAccount"),
				Name:      serviceAccount.Metadata.Name().Elem(),
				Namespace: args.Namespace,
			},
		},
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	podLabels := maps.Clone(labels)
	podLabels[apiserver.AccessLabelKey] = pulumi.String(apiserver.AccessLabelValue)

	prom, err := monitoringv1.NewPrometheus(ctx, name, &monitoringv1.PrometheusArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(instanceName),
			Namespace: args.Namespace,
			Labels:    labels,
		},
		Spec: &monitoringv1.PrometheusSpecArgs{
			Image:              pulumi.Sprintf("%s:v%s", prometheusImage, args.Version),
			Version:            pulumi.String(args.Version),
			Replicas:           pulumi.Int(1),
			ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
			Retention:          pulumi.String(args.Retention),
			RetentionSize:      pulumi.String(args.RetentionSize),
			// Routes fired alerts to the bare Alertmanager alertmanager.go
			// deploys - see that file's doc comment for why it has no
			// notification receivers wired yet.
			Alerting: &monitoringv1.PrometheusSpecAlertingArgs{
				Alertmanagers: monitoringv1.PrometheusSpecAlertingAlertmanagersArray{
					&monitoringv1.PrometheusSpecAlertingAlertmanagersArgs{
						Namespace: args.Namespace,
						Name:      pulumi.String(AlertmanagerServiceName),
						Port:      pulumi.String("web"),
					},
				},
			},
			SecurityContext: &monitoringv1.PrometheusSpecSecurityContextArgs{
				FsGroup:      pulumi.Int(2000),
				RunAsNonRoot: pulumi.Bool(true),
				RunAsUser:    pulumi.Int(1000),
			},
			Resources: &monitoringv1.PrometheusSpecResourcesArgs{
				Requests: pulumi.Map{"cpu": pulumi.String("100m"), "memory": pulumi.String("512Mi")},
				// Kept at 2Gi even on this small a cluster - CLAUDE.md
				// documents this as a deliberate floor for
				// container-metrics (cAdvisor/kubelet) cardinality, not
				// something tied to workload count.
				Limits: pulumi.Map{"cpu": pulumi.String("1000m"), "memory": pulumi.String("2Gi")},
			},
			Storage: &monitoringv1.PrometheusSpecStorageArgs{
				VolumeClaimTemplate: &monitoringv1.PrometheusSpecStorageVolumeClaimTemplateArgs{
					Spec: &monitoringv1.PrometheusSpecStorageVolumeClaimTemplateSpecArgs{
						StorageClassName: args.StorageClassName,
						AccessModes:      pulumi.StringArray{pulumi.String("ReadWriteOnce")},
						Resources: &monitoringv1.PrometheusSpecStorageVolumeClaimTemplateSpecResourcesArgs{
							Requests: pulumi.Map{"storage": pulumi.String(args.StorageSize)},
						},
					},
				},
			},
			// Empty selectors = cluster-wide auto-discovery of
			// ServiceMonitors/PodMonitors/Rules/Probes/ScrapeConfigs from
			// any namespace, no label restriction - matches legacy.
			ServiceMonitorSelector:          &monitoringv1.PrometheusSpecServiceMonitorSelectorArgs{},
			ServiceMonitorNamespaceSelector: &monitoringv1.PrometheusSpecServiceMonitorNamespaceSelectorArgs{},
			PodMonitorSelector:              &monitoringv1.PrometheusSpecPodMonitorSelectorArgs{},
			PodMonitorNamespaceSelector:     &monitoringv1.PrometheusSpecPodMonitorNamespaceSelectorArgs{},
			RuleSelector:                    &monitoringv1.PrometheusSpecRuleSelectorArgs{},
			RuleNamespaceSelector:           &monitoringv1.PrometheusSpecRuleNamespaceSelectorArgs{},
			ProbeSelector:                   &monitoringv1.PrometheusSpecProbeSelectorArgs{},
			ProbeNamespaceSelector:          &monitoringv1.PrometheusSpecProbeNamespaceSelectorArgs{},
			ScrapeConfigSelector:            &monitoringv1.PrometheusSpecScrapeConfigSelectorArgs{},
			ScrapeConfigNamespaceSelector:   &monitoringv1.PrometheusSpecScrapeConfigNamespaceSelectorArgs{},
			NodeSelector:                    pulumi.StringMap{"kubernetes.io/os": pulumi.String("linux")},
			PodMetadata: &monitoringv1.PrometheusSpecPodMetadataArgs{
				Labels: podLabels,
			},
			EnableFeatures: pulumi.StringArray{},
			ExternalLabels: pulumi.StringMap{},
		},
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	serviceLabels := maps.Clone(labels)
	serviceLabels["istio.io/use-waypoint"] = args.WaypointName

	service, err := corev1.NewService(ctx, fmt.Sprintf("%s-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fmt.Sprintf("prometheus-%s", instanceName)),
			Namespace: args.Namespace,
			Labels:    serviceLabels,
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector:        pulumi.StringMap{"prometheus": pulumi.String(instanceName)},
			SessionAffinity: pulumi.String("ClientIP"),
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{Name: pulumi.String("web"), Port: pulumi.Int(9090), TargetPort: pulumi.String("web")},
				&corev1.ServicePortArgs{Name: pulumi.String("reloader-web"), Port: pulumi.Int(8080), TargetPort: pulumi.String("reloader-web")},
			},
		},
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	return prom, service, nil
}
