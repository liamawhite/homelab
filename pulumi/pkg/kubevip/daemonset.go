package kubevip

import (
	"fmt"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// createDaemonSet creates the kube-vip DaemonSet
func createDaemonSet(ctx *pulumi.Context, name string, args *KubeVipArgs, namespace pulumi.StringInput, serviceAccountName pulumi.StringInput, opts ...pulumi.ResourceOption) (*appsv1.DaemonSet, error) {
	image := fmt.Sprintf("ghcr.io/kube-vip/kube-vip:%s", args.Version)

	labels := pulumi.StringMap{
		"app.kubernetes.io/name":    pulumi.String("kube-vip"),
		"app.kubernetes.io/version": pulumi.String(args.Version),
	}

	daemonSet, err := appsv1.NewDaemonSet(ctx, name+"-ds", &appsv1.DaemonSetArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("kube-vip"),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &appsv1.DaemonSetSpecArgs{
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app.kubernetes.io/name": pulumi.String("kube-vip"),
				},
			},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: labels,
				},
				Spec: &corev1.PodSpecArgs{
					// Host network required for VIP binding
					HostNetwork: pulumi.Bool(true),

					// Use kube-vip service account
					ServiceAccountName: serviceAccountName,

					// Only run on control plane nodes
					NodeSelector: pulumi.StringMap{
						"node-role.kubernetes.io/control-plane": pulumi.String("true"),
					},

					// Tolerate control plane taints
					Tolerations: corev1.TolerationArray{
						&corev1.TolerationArgs{
							Key:      pulumi.String("node-role.kubernetes.io/control-plane"),
							Operator: pulumi.String("Exists"),
							Effect:   pulumi.String("NoSchedule"),
						},
						&corev1.TolerationArgs{
							Key:      pulumi.String("node-role.kubernetes.io/master"),
							Operator: pulumi.String("Exists"),
							Effect:   pulumi.String("NoSchedule"),
						},
					},

					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:            pulumi.String("kube-vip"),
							Image:           pulumi.String(image),
							ImagePullPolicy: pulumi.String("IfNotPresent"),
							Args: pulumi.StringArray{
								pulumi.String("manager"),
							},

							// Environment variables for kube-vip configuration
							Env: corev1.EnvVarArray{
								// Use ARP for VIP advertisement
								&corev1.EnvVarArgs{
									Name:  pulumi.String("vip_arp"),
									Value: pulumi.String("true"),
								},
								// API server port
								&corev1.EnvVarArgs{
									Name:  pulumi.String("port"),
									Value: pulumi.String("6443"),
								},
								// Network interface (hardcoded to eth0)
								&corev1.EnvVarArgs{
									Name:  pulumi.String("vip_interface"),
									Value: pulumi.String("eth0"),
								},
								// CIDR for single IP
								&corev1.EnvVarArgs{
									Name:  pulumi.String("vip_cidr"),
									Value: pulumi.String("32"),
								},
								// Enable control plane mode
								&corev1.EnvVarArgs{
									Name:  pulumi.String("cp_enable"),
									Value: pulumi.String("true"),
								},
								// Namespace for control plane
								&corev1.EnvVarArgs{
									Name:  pulumi.String("cp_namespace"),
									Value: pulumi.String("kube-system"),
								},
								// Disable DDNS
								&corev1.EnvVarArgs{
									Name:  pulumi.String("vip_ddns"),
									Value: pulumi.String("false"),
								},
								// CRITICAL: Disable service load balancing (MetalLB handles this)
								&corev1.EnvVarArgs{
									Name:  pulumi.String("svc_enable"),
									Value: pulumi.String("false"),
								},
								// Enable leader election
								&corev1.EnvVarArgs{
									Name:  pulumi.String("vip_leaderelection"),
									Value: pulumi.String("true"),
								},
								// Lease duration (seconds)
								&corev1.EnvVarArgs{
									Name:  pulumi.String("vip_leaseduration"),
									Value: pulumi.String("5"),
								},
								// Renew deadline (seconds)
								&corev1.EnvVarArgs{
									Name:  pulumi.String("vip_renewdeadline"),
									Value: pulumi.String("3"),
								},
								// Retry period (seconds)
								&corev1.EnvVarArgs{
									Name:  pulumi.String("vip_retryperiod"),
									Value: pulumi.String("1"),
								},
								// VIP address
								&corev1.EnvVarArgs{
									Name:  pulumi.String("address"),
									Value: pulumi.String(args.VIP),
								},
								// Prometheus metrics server
								&corev1.EnvVarArgs{
									Name:  pulumi.String("prometheus_server"),
									Value: pulumi.String(":2112"),
								},
							},

							// Security context with required capabilities
							SecurityContext: &corev1.SecurityContextArgs{
								Capabilities: &corev1.CapabilitiesArgs{
									Add: pulumi.StringArray{
										pulumi.String("NET_ADMIN"),
										pulumi.String("NET_RAW"),
									},
								},
							},

							// Resource limits (Raspberry Pi optimized)
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("50m"),
									"memory": pulumi.String("64Mi"),
								},
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("100m"),
									"memory": pulumi.String("128Mi"),
								},
							},
						},
					},
				},
			},
		},
	}, opts...)

	if err != nil {
		return nil, err
	}

	return daemonSet, nil
}
