package prometheus

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	monitoringv1 "github.com/liamawhite/homelab/pkg/crds/prometheus/crds/kubernetes/monitoring/v1"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const kubeStateMetricsImage = "registry.k8s.io/kube-state-metrics/kube-state-metrics"

// newKubeStateMetrics deploys kube-state-metrics (turns Kubernetes object
// state - Deployments, PVCs, HPAs, ... - into metrics) and its
// ServiceMonitor. A normal ambient pod in "monitoring", so it carries
// apiserver.AccessLabelKey directly (unlike node-exporter's hostNetwork
// pods).
func newKubeStateMetrics(ctx *pulumi.Context, name string, namespace pulumi.StringInput, version string, opts ...pulumi.ResourceOption) error {
	labels := pulumi.StringMap{
		"app.kubernetes.io/name":      pulumi.String("kube-state-metrics"),
		"app.kubernetes.io/component": pulumi.String("exporter"),
	}

	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("kube-state-metrics"),
			Namespace: namespace,
		},
		AutomountServiceAccountToken: pulumi.Bool(false),
	}, opts...)
	if err != nil {
		return err
	}

	// Broad read-only access to the object kinds kube-state-metrics turns
	// into metrics, mirroring upstream's own recommended RBAC (and
	// _migrateme's kube-state-metrics component).
	clusterRole, err := rbacv1.NewClusterRole(ctx, fmt.Sprintf("%s-cr", name), &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("kube-state-metrics"),
		},
		Rules: rbacv1.PolicyRuleArray{
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{
					pulumi.String("configmaps"), pulumi.String("secrets"), pulumi.String("nodes"), pulumi.String("pods"),
					pulumi.String("services"), pulumi.String("serviceaccounts"), pulumi.String("resourcequotas"),
					pulumi.String("replicationcontrollers"), pulumi.String("limitranges"),
					pulumi.String("persistentvolumeclaims"), pulumi.String("persistentvolumes"),
					pulumi.String("namespaces"), pulumi.String("endpoints"),
				},
				Verbs: pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("apps")},
				Resources: pulumi.StringArray{pulumi.String("deployments"), pulumi.String("daemonsets"), pulumi.String("statefulsets"), pulumi.String("replicasets")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("batch")},
				Resources: pulumi.StringArray{pulumi.String("cronjobs"), pulumi.String("jobs")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("autoscaling")},
				Resources: pulumi.StringArray{pulumi.String("horizontalpodautoscalers")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("policy")},
				Resources: pulumi.StringArray{pulumi.String("poddisruptionbudgets")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("certificates.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("certificatesigningrequests")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("discovery.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("endpointslices")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("storage.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("storageclasses"), pulumi.String("volumeattachments")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("admissionregistration.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("mutatingwebhookconfigurations"), pulumi.String("validatingwebhookconfigurations")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("networking.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("networkpolicies"), pulumi.String("ingressclasses"), pulumi.String("ingresses")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("coordination.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("leases")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("rbac.authorization.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("clusterrolebindings"), pulumi.String("clusterroles"), pulumi.String("rolebindings"), pulumi.String("roles")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("authentication.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("tokenreviews")},
				Verbs:     pulumi.StringArray{pulumi.String("create")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("authorization.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("subjectaccessreviews")},
				Verbs:     pulumi.StringArray{pulumi.String("create")},
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	_, err = rbacv1.NewClusterRoleBinding(ctx, fmt.Sprintf("%s-crb", name), &rbacv1.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("kube-state-metrics"),
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
				Namespace: namespace,
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	podLabels := pulumi.StringMap{
		"app.kubernetes.io/name":      pulumi.String("kube-state-metrics"),
		"app.kubernetes.io/component": pulumi.String("exporter"),
		apiserver.AccessLabelKey:      pulumi.String(apiserver.AccessLabelValue),
	}

	_, err = appsv1.NewDeployment(ctx, fmt.Sprintf("%s-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("kube-state-metrics"),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Selector: &metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{Labels: podLabels},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName:           serviceAccount.Metadata.Name().Elem(),
					AutomountServiceAccountToken: pulumi.Bool(true),
					SecurityContext: &corev1.PodSecurityContextArgs{
						RunAsNonRoot: pulumi.Bool(true),
						RunAsUser:    pulumi.Int(65534),
						RunAsGroup:   pulumi.Int(65534),
						FsGroup:      pulumi.Int(65534),
					},
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("kube-state-metrics"),
							Image: pulumi.Sprintf("%s:v%s", kubeStateMetricsImage, version),
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{Name: pulumi.String("http-metrics"), ContainerPort: pulumi.Int(8080)},
								&corev1.ContainerPortArgs{Name: pulumi.String("telemetry"), ContainerPort: pulumi.Int(8081)},
							},
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{Path: pulumi.String("/healthz"), Port: pulumi.Int(8080)},
							},
							ReadinessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{Path: pulumi.String("/"), Port: pulumi.Int(8081)},
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{"cpu": pulumi.String("10m"), "memory": pulumi.String("32Mi")},
								Limits:   pulumi.StringMap{"cpu": pulumi.String("250m"), "memory": pulumi.String("256Mi")},
							},
							SecurityContext: &corev1.SecurityContextArgs{
								AllowPrivilegeEscalation: pulumi.Bool(false),
								ReadOnlyRootFilesystem:   pulumi.Bool(true),
								RunAsNonRoot:             pulumi.Bool(true),
								RunAsUser:                pulumi.Int(65534),
								Capabilities: &corev1.CapabilitiesArgs{
									Drop: pulumi.StringArray{pulumi.String("ALL")},
								},
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

	_, err = corev1.NewService(ctx, fmt.Sprintf("%s-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("kube-state-metrics"),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &corev1.ServiceSpecArgs{
			ClusterIP: pulumi.String("None"),
			Selector:  labels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{Name: pulumi.String("http-metrics"), Port: pulumi.Int(8080), TargetPort: pulumi.String("http-metrics")},
				&corev1.ServicePortArgs{Name: pulumi.String("telemetry"), Port: pulumi.Int(8081), TargetPort: pulumi.String("telemetry")},
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	_, err = monitoringv1.NewServiceMonitor(ctx, fmt.Sprintf("%s-servicemonitor", name), &monitoringv1.ServiceMonitorArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("kube-state-metrics"),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &monitoringv1.ServiceMonitorSpecArgs{
			Selector: &monitoringv1.ServiceMonitorSpecSelectorArgs{MatchLabels: labels},
			Endpoints: monitoringv1.ServiceMonitorSpecEndpointsArray{
				&monitoringv1.ServiceMonitorSpecEndpointsArgs{
					Port:          pulumi.String("http-metrics"),
					Interval:      pulumi.String("30s"),
					ScrapeTimeout: pulumi.String("30s"),
				},
				&monitoringv1.ServiceMonitorSpecEndpointsArgs{
					Port:          pulumi.String("telemetry"),
					Interval:      pulumi.String("30s"),
					ScrapeTimeout: pulumi.String("30s"),
				},
			},
		},
	}, opts...)
	return err
}
