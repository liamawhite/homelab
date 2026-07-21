package prometheus

import (
	"fmt"

	monitoringv1 "github.com/liamawhite/homelab/pkg/crds/prometheus/crds/kubernetes/monitoring/v1"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const nodeExporterImage = "quay.io/prometheus/node-exporter"

// newNodeExporter deploys node-exporter as a DaemonSet (one pod per node,
// HostNetwork/HostPID so it can read the host's own /proc and /sys) and its
// ServiceMonitor.
//
// Deliberately carries NO apiserver.AccessLabelKey (or any other Cilium
// access label): this cluster never enabled Cilium's host-firewall feature
// (same precedent as pkg/components/hubcontroller/kubevip), so hostNetwork
// pods aren't subject to the pod-level default-deny baseline at all - the
// label would be a harmless no-op here, not a load-bearing grant.
func newNodeExporter(ctx *pulumi.Context, name string, namespace pulumi.StringInput, nodeExporterVersion, kubeRBACProxyVersion string, opts ...pulumi.ResourceOption) error {
	labels := pulumi.StringMap{
		"app.kubernetes.io/name":      pulumi.String("node-exporter"),
		"app.kubernetes.io/component": pulumi.String("exporter"),
	}

	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("node-exporter"),
			Namespace: namespace,
		},
		AutomountServiceAccountToken: pulumi.Bool(false),
	}, opts...)
	if err != nil {
		return err
	}

	// kube-rbac-proxy's own TokenReview/SubjectAccessReview calls - the same
	// pair every kube-rbac-proxy sidecar in this stack needs.
	clusterRole, err := rbacv1.NewClusterRole(ctx, fmt.Sprintf("%s-cr", name), &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("node-exporter"),
		},
		Rules: rbacv1.PolicyRuleArray{
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
			Name: pulumi.String("node-exporter"),
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

	_, err = appsv1.NewDaemonSet(ctx, fmt.Sprintf("%s-daemonset", name), &appsv1.DaemonSetArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("node-exporter"),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &appsv1.DaemonSetSpecArgs{
			Selector: &metav1.LabelSelectorArgs{MatchLabels: labels},
			UpdateStrategy: &appsv1.DaemonSetUpdateStrategyArgs{
				Type: pulumi.String("RollingUpdate"),
				RollingUpdate: &appsv1.RollingUpdateDaemonSetArgs{
					MaxUnavailable: pulumi.Int(1),
				},
			},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{Labels: labels},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName:           serviceAccount.Metadata.Name().Elem(),
					AutomountServiceAccountToken: pulumi.Bool(true),
					HostNetwork:                  pulumi.Bool(true),
					HostPID:                      pulumi.Bool(true),
					PriorityClassName:            pulumi.String("system-cluster-critical"),
					NodeSelector:                 pulumi.StringMap{"kubernetes.io/os": pulumi.String("linux")},
					Tolerations: corev1.TolerationArray{
						&corev1.TolerationArgs{Operator: pulumi.String("Exists")},
					},
					SecurityContext: &corev1.PodSecurityContextArgs{
						RunAsNonRoot: pulumi.Bool(true),
						RunAsUser:    pulumi.Int(65534),
						RunAsGroup:   pulumi.Int(65534),
					},
					Volumes: corev1.VolumeArray{
						&corev1.VolumeArgs{
							Name:     pulumi.String("sys"),
							HostPath: &corev1.HostPathVolumeSourceArgs{Path: pulumi.String("/sys")},
						},
						&corev1.VolumeArgs{
							Name:     pulumi.String("root"),
							HostPath: &corev1.HostPathVolumeSourceArgs{Path: pulumi.String("/")},
						},
					},
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("node-exporter"),
							Image: pulumi.Sprintf("%s:v%s", nodeExporterImage, nodeExporterVersion),
							Args: pulumi.StringArray{
								pulumi.String("--web.listen-address=127.0.0.1:9101"),
								pulumi.String("--path.sysfs=/host/sys"),
								pulumi.String("--path.rootfs=/host/root"),
								pulumi.String("--path.procfs=/host/root/proc"),
								pulumi.String("--path.udev.data=/host/root/run/udev/data"),
								pulumi.String("--no-collector.wifi"),
								pulumi.String("--no-collector.hwmon"),
								pulumi.String("--no-collector.btrfs"),
								pulumi.String("--collector.filesystem.mount-points-exclude=^/(dev|proc|sys|run/k3s/containerd/.+|var/lib/kubelet/pods/.+)($|/)"),
								pulumi.String("--collector.netclass.ignored-devices=^(veth.*|[a-f0-9]{15})$"),
								pulumi.String("--collector.netdev.device-exclude=^(veth.*|[a-f0-9]{15})$"),
							},
							VolumeMounts: corev1.VolumeMountArray{
								&corev1.VolumeMountArgs{Name: pulumi.String("sys"), MountPath: pulumi.String("/host/sys"), MountPropagation: pulumi.String("HostToContainer"), ReadOnly: pulumi.Bool(true)},
								&corev1.VolumeMountArgs{Name: pulumi.String("root"), MountPath: pulumi.String("/host/root"), MountPropagation: pulumi.String("HostToContainer"), ReadOnly: pulumi.Bool(true)},
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{"cpu": pulumi.String("10m"), "memory": pulumi.String("32Mi")},
								Limits:   pulumi.StringMap{"cpu": pulumi.String("100m"), "memory": pulumi.String("64Mi")},
							},
							SecurityContext: &corev1.SecurityContextArgs{
								AllowPrivilegeEscalation: pulumi.Bool(false),
								ReadOnlyRootFilesystem:   pulumi.Bool(true),
								Capabilities: &corev1.CapabilitiesArgs{
									Add:  pulumi.StringArray{pulumi.String("SYS_TIME")},
									Drop: pulumi.StringArray{pulumi.String("ALL")},
								},
							},
						},
						&corev1.ContainerArgs{
							Name:  pulumi.String("kube-rbac-proxy"),
							Image: pulumi.Sprintf("%s:v%s", kubeRBACProxyImage, kubeRBACProxyVersion),
							Args: pulumi.StringArray{
								pulumi.String("--secure-listen-address=[$(IP)]:9100"),
								pulumi.Sprintf("--tls-cipher-suites=%s", kubeRBACProxyCiphers),
								pulumi.String("--upstream=http://127.0.0.1:9101/"),
							},
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{
									Name: pulumi.String("IP"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										FieldRef: &corev1.ObjectFieldSelectorArgs{FieldPath: pulumi.String("status.podIP")},
									},
								},
							},
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{Name: pulumi.String("https"), ContainerPort: pulumi.Int(9100), HostPort: pulumi.Int(9100)},
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{"cpu": pulumi.String("10m"), "memory": pulumi.String("20Mi")},
								Limits:   pulumi.StringMap{"cpu": pulumi.String("20m"), "memory": pulumi.String("40Mi")},
							},
							SecurityContext: &corev1.SecurityContextArgs{
								AllowPrivilegeEscalation: pulumi.Bool(false),
								ReadOnlyRootFilesystem:   pulumi.Bool(true),
								RunAsNonRoot:             pulumi.Bool(true),
								RunAsUser:                pulumi.Int(65532),
								RunAsGroup:               pulumi.Int(65532),
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
			Name:      pulumi.String("node-exporter"),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &corev1.ServiceSpecArgs{
			ClusterIP: pulumi.String("None"),
			Selector:  labels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{Name: pulumi.String("https"), Port: pulumi.Int(9100), TargetPort: pulumi.String("https")},
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	_, err = monitoringv1.NewServiceMonitor(ctx, fmt.Sprintf("%s-servicemonitor", name), &monitoringv1.ServiceMonitorArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("node-exporter"),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &monitoringv1.ServiceMonitorSpecArgs{
			JobLabel: pulumi.String("app.kubernetes.io/name"),
			Selector: &monitoringv1.ServiceMonitorSpecSelectorArgs{MatchLabels: labels},
			Endpoints: monitoringv1.ServiceMonitorSpecEndpointsArray{
				&monitoringv1.ServiceMonitorSpecEndpointsArgs{
					Port:            pulumi.String("https"),
					Scheme:          pulumi.String("https"),
					Interval:        pulumi.String("15s"),
					BearerTokenFile: pulumi.String("/var/run/secrets/kubernetes.io/serviceaccount/token"),
					TlsConfig: &monitoringv1.ServiceMonitorSpecEndpointsTlsConfigArgs{
						InsecureSkipVerify: pulumi.Bool(true),
					},
					Relabelings: monitoringv1.ServiceMonitorSpecEndpointsRelabelingsArray{
						&monitoringv1.ServiceMonitorSpecEndpointsRelabelingsArgs{
							Action:       pulumi.String("replace"),
							Regex:        pulumi.String("(.*)"),
							Replacement:  pulumi.String("$1"),
							SourceLabels: pulumi.StringArray{pulumi.String("__meta_kubernetes_pod_node_name")},
							TargetLabel:  pulumi.String("instance"),
						},
					},
				},
			},
		},
	}, opts...)
	return err
}
