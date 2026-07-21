package prometheus

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	operatorName         = "prometheus-operator"
	kubeRBACProxyImage   = "quay.io/brancz/kube-rbac-proxy"
	operatorImage        = "quay.io/prometheus-operator/prometheus-operator"
	configReloaderImage  = "quay.io/prometheus-operator/prometheus-config-reloader"
	kubeRBACProxyCiphers = "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305"
)

// newOperator deploys prometheus-operator: the controller that reconciles
// Prometheus/ServiceMonitor/PrometheusRule/... CRs (see pkg/crds/prometheus)
// into an actual StatefulSet + generated config. It watches the K8s API
// directly (its controller-runtime cache), so its pod carries
// apiserver.AccessLabelKey.
func newOperator(ctx *pulumi.Context, name string, namespace pulumi.StringInput, operatorVersion, kubeRBACProxyVersion string, opts ...pulumi.ResourceOption) (*appsv1.Deployment, error) {
	labels := pulumi.StringMap{
		"app.kubernetes.io/name":      pulumi.String(operatorName),
		"app.kubernetes.io/component": pulumi.String("controller"),
	}

	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-operator-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(operatorName),
			Namespace: namespace,
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	// Broad access to every monitoring.coreos.com CRD (this is the
	// controller that reconciles all of them), plus the handful of core/apps
	// resources it manages on their behalf (StatefulSets it generates from a
	// Prometheus/Alertmanager/ThanosRuler CR, the Secret it renders scrape
	// config into, the Service the "kubelet" flag below has it maintain,
	// ...). Mirrors _migrateme/components/kubernetes/prometheus-operator's
	// ClusterRole verbatim.
	clusterRole, err := rbacv1.NewClusterRole(ctx, fmt.Sprintf("%s-operator-cr", name), &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(operatorName),
		},
		Rules: rbacv1.PolicyRuleArray{
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("monitoring.coreos.com")},
				Resources: pulumi.StringArray{
					pulumi.String("alertmanagers"), pulumi.String("alertmanagers/finalizers"), pulumi.String("alertmanagers/status"),
					pulumi.String("alertmanagerconfigs"),
					pulumi.String("prometheuses"), pulumi.String("prometheuses/finalizers"), pulumi.String("prometheuses/status"),
					pulumi.String("prometheusagents"), pulumi.String("prometheusagents/finalizers"), pulumi.String("prometheusagents/status"),
					pulumi.String("thanosrulers"), pulumi.String("thanosrulers/finalizers"),
					pulumi.String("scrapeconfigs"),
					pulumi.String("servicemonitors"), pulumi.String("servicemonitors/status"),
					pulumi.String("podmonitors"),
					pulumi.String("probes"),
					pulumi.String("prometheusrules"),
				},
				Verbs: pulumi.StringArray{pulumi.String("*")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("apps")},
				Resources: pulumi.StringArray{pulumi.String("statefulsets")},
				Verbs:     pulumi.StringArray{pulumi.String("*")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("configmaps"), pulumi.String("secrets")},
				Verbs:     pulumi.StringArray{pulumi.String("*")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("pods")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("delete")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("services"), pulumi.String("services/finalizers")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("create"), pulumi.String("update"), pulumi.String("delete")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("nodes")},
				Verbs:     pulumi.StringArray{pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("namespaces")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("events")},
				Verbs:     pulumi.StringArray{pulumi.String("patch"), pulumi.String("create")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("networking.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("ingresses")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("storage.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("storageclasses")},
				Verbs:     pulumi.StringArray{pulumi.String("get")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("endpoints")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("create"), pulumi.String("update"), pulumi.String("delete")},
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
		return nil, err
	}

	_, err = rbacv1.NewClusterRoleBinding(ctx, fmt.Sprintf("%s-operator-crb", name), &rbacv1.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(operatorName),
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
		return nil, err
	}

	_, err = corev1.NewService(ctx, fmt.Sprintf("%s-operator-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(operatorName),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &corev1.ServiceSpecArgs{
			ClusterIP: pulumi.String("None"),
			Selector:  labels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{Name: pulumi.String("https"), Port: pulumi.Int(8443), TargetPort: pulumi.String("https")},
			},
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	podLabels := pulumi.StringMap{
		"app.kubernetes.io/name":      pulumi.String(operatorName),
		"app.kubernetes.io/component": pulumi.String("controller"),
		apiserver.AccessLabelKey:      pulumi.String(apiserver.AccessLabelValue),
	}

	deployment, err := appsv1.NewDeployment(ctx, fmt.Sprintf("%s-operator-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(operatorName),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Selector: &metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{Labels: podLabels},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
					NodeSelector:       pulumi.StringMap{"kubernetes.io/os": pulumi.String("linux")},
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String(operatorName),
							Image: pulumi.Sprintf("%s:v%s", operatorImage, operatorVersion),
							Args: pulumi.StringArray{
								pulumi.String("--kubelet-service=kube-system/kubelet"),
								pulumi.Sprintf("--prometheus-config-reloader=%s:v%s", configReloaderImage, operatorVersion),
								pulumi.String("--kubelet-endpoints=true"),
								pulumi.String("--kubelet-endpointslice=false"),
							},
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{Name: pulumi.String("GOGC"), Value: pulumi.String("30")},
							},
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{Name: pulumi.String("http"), ContainerPort: pulumi.Int(8080)},
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{"cpu": pulumi.String("10m"), "memory": pulumi.String("32Mi")},
								Limits:   pulumi.StringMap{"cpu": pulumi.String("100m"), "memory": pulumi.String("64Mi")},
							},
							SecurityContext: &corev1.SecurityContextArgs{
								AllowPrivilegeEscalation: pulumi.Bool(false),
								ReadOnlyRootFilesystem:   pulumi.Bool(true),
								Capabilities: &corev1.CapabilitiesArgs{
									Drop: pulumi.StringArray{pulumi.String("ALL")},
								},
							},
						},
						&corev1.ContainerArgs{
							Name:  pulumi.String("kube-rbac-proxy"),
							Image: pulumi.Sprintf("%s:v%s", kubeRBACProxyImage, kubeRBACProxyVersion),
							Args: pulumi.StringArray{
								pulumi.String("--secure-listen-address=:8443"),
								pulumi.Sprintf("--tls-cipher-suites=%s", kubeRBACProxyCiphers),
								pulumi.String("--upstream=http://127.0.0.1:8080/"),
							},
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{Name: pulumi.String("https"), ContainerPort: pulumi.Int(8443)},
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
					SecurityContext: &corev1.PodSecurityContextArgs{
						RunAsNonRoot: pulumi.Bool(true),
						RunAsUser:    pulumi.Int(65534),
						RunAsGroup:   pulumi.Int(65534),
						SeccompProfile: &corev1.SeccompProfileArgs{
							Type: pulumi.String("RuntimeDefault"),
						},
					},
				},
			},
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}
