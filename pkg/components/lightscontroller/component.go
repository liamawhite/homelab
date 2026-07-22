// Package lightscontroller deploys the lights-controller (see
// applications/lights-controller): a Deployment that syncs live Philips
// Hue light status into Light custom resources (see pkg/crds/lights).
//
// The namespace itself (including the Istio ambient-mode label) is created
// centrally by pkg/deploy/namespaces.go, not by this component.
package lightscontroller

import (
	"encoding/json"
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	"github.com/liamawhite/homelab/pkg/config"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const serviceAccountName = "lights-controller"

// LightsController represents the deployed lights-controller.
type LightsController struct {
	pulumi.ResourceState

	Namespace pulumi.StringOutput
}

// LightsControllerArgs contains the configuration for LightsController.
type LightsControllerArgs struct {
	// Namespace is created centrally by pkg/deploy/namespaces.go and
	// passed in here - this component does not create it.
	Namespace pulumi.StringInput
	// Bridges is infraCfg.Lights.Hue.Bridges - every paired bridge's id
	// and application key, marshaled into the Secret the controller reads
	// at startup.
	Bridges []config.HueBridgeConfig
	// PollInterval is how often the controller polls bridges, e.g. "60s".
	PollInterval pulumi.StringInput
	// DryRun controls whether the Light reconciler enacts spec changes
	// against the bridge (false) or only logs drift (true). Passed
	// explicitly rather than left to the binary's own default so flipping
	// a live deployment into real-enactment mode is a deliberate,
	// reviewable change here rather than a side effect of a new image.
	DryRun pulumi.BoolInput
	// Image is the shared lights-controller/hub-controller image
	// (built once in applications/lights.go and passed to both components).
	Image pulumi.StringInput
}

// NewLightsController creates the lights-controller Deployment, its RBAC,
// the Secret carrying paired bridge credentials, and the network policy
// letting it reach the API server and the Hue bridge(s) on the LAN.
func NewLightsController(ctx *pulumi.Context, name string, args *LightsControllerArgs, opts ...pulumi.ResourceOption) (*LightsController, error) {
	lc := &LightsController{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:lights-controller", name, lc, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(lc))

	lc.Namespace = args.Namespace.ToStringOutput()

	// 1. Dedicated ServiceAccount - every app in this repo gets its own
	// rather than running as its namespace's shared "default" account.
	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(serviceAccountName),
			Namespace: args.Namespace,
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 2. ClusterRole/ClusterRoleBinding for the Light CRD itself - cluster
	// scoped, so this can't be a namespaced Role.
	clusterRole, err := rbacv1.NewClusterRole(ctx, fmt.Sprintf("%s-cr", name), &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("lights-controller"),
		},
		Rules: rbacv1.PolicyRuleArray{
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("lights")},
				Verbs: pulumi.StringArray{
					pulumi.String("get"), pulumi.String("list"), pulumi.String("watch"),
					pulumi.String("create"), pulumi.String("update"), pulumi.String("patch"), pulumi.String("delete"),
				},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("lights/status")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("update"), pulumi.String("patch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("switches")},
				Verbs: pulumi.StringArray{
					pulumi.String("get"), pulumi.String("list"), pulumi.String("watch"),
					pulumi.String("create"), pulumi.String("update"), pulumi.String("patch"), pulumi.String("delete"),
				},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("switches/status")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("update"), pulumi.String("patch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("groups")},
				Verbs: pulumi.StringArray{
					pulumi.String("get"), pulumi.String("list"), pulumi.String("watch"),
					pulumi.String("create"), pulumi.String("update"), pulumi.String("patch"), pulumi.String("delete"),
				},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("groups/status")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("update"), pulumi.String("patch")},
			},
			// Read-only: hub-controller (pkg/components/hubcontroller) owns
			// writing HueBridge - this controller only reads status.ip from
			// it, never writes one.
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("huebridges")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("list"), pulumi.String("watch")},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	_, err = rbacv1.NewClusterRoleBinding(ctx, fmt.Sprintf("%s-crb", name), &rbacv1.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("lights-controller"),
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
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 3. Namespaced Role/RoleBinding for leader-election Leases - these
	// are namespaced even though Light is cluster-scoped, so this is
	// intentionally separate from the ClusterRole above rather than
	// lumping both into one. Also covers Events: controller-runtime's
	// leader-election machinery records a "became leader" Event on the
	// Lease it acquires - confirmed live this is otherwise rejected
	// ("events is forbidden"), non-fatal (leader election itself still
	// succeeds) but real RBAC noise worth granting properly.
	leaseRole, err := rbacv1.NewRole(ctx, fmt.Sprintf("%s-lease-role", name), &rbacv1.RoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("lights-controller-leases"),
			Namespace: args.Namespace,
		},
		Rules: rbacv1.PolicyRuleArray{
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("coordination.k8s.io")},
				Resources: pulumi.StringArray{pulumi.String("leases")},
				Verbs: pulumi.StringArray{
					pulumi.String("get"), pulumi.String("list"), pulumi.String("watch"),
					pulumi.String("create"), pulumi.String("update"),
				},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{pulumi.String("events")},
				Verbs:     pulumi.StringArray{pulumi.String("create"), pulumi.String("patch")},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	_, err = rbacv1.NewRoleBinding(ctx, fmt.Sprintf("%s-lease-rb", name), &rbacv1.RoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("lights-controller-leases"),
			Namespace: args.Namespace,
		},
		RoleRef: &rbacv1.RoleRefArgs{
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			Kind:     pulumi.String("Role"),
			Name:     leaseRole.Metadata.Name().Elem(),
		},
		Subjects: rbacv1.SubjectArray{
			&rbacv1.SubjectArgs{
				Kind:      pulumi.String("ServiceAccount"),
				Name:      serviceAccount.Metadata.Name().Elem(),
				Namespace: args.Namespace,
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 4. Secret carrying every paired bridge's id/appKey, mounted as a
	// file rather than per-key env vars: the bridge list has arbitrary
	// cardinality (doesn't map to fixed env var names), this matches the
	// existing precedent for secret-shaped config (see
	// pkg/components/cloudflare/tunnel's credentials.json Secret), and it
	// avoids leaking key material into `kubectl describe pod`'s env dump.
	bridgesJSON, err := json.Marshal(args.Bridges)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bridges: %w", err)
	}

	secret, err := corev1.NewSecret(ctx, fmt.Sprintf("%s-secret", name), &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("hue-bridges"),
			Namespace: args.Namespace,
		},
		Type: pulumi.String("Opaque"),
		StringData: pulumi.StringMap{
			"bridges.json": pulumi.String(string(bridgesJSON)),
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 5. Deployment. Single replica - leader election (see the lease
	// RBAC above) makes more than one safe later, but nothing needs it
	// yet. Image is the shared lights-controller/hub-controller image
	// (see applications/lights.go) - Command picks which of its two binaries
	// this Deployment actually runs.
	const volumeName = "hue-bridges"
	const mountPath = "/etc/lights-controller"

	_, err = appsv1.NewDeployment(ctx, fmt.Sprintf("%s-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("lights-controller"),
			Namespace: args.Namespace,
			Labels: pulumi.StringMap{
				"app": pulumi.String("lights-controller"),
			},
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app": pulumi.String("lights-controller"),
				},
			},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: pulumi.StringMap{
						"app": pulumi.String("lights-controller"),
						// Needs the K8s API to manage Light CRs, read
						// HueBridge status, and its leader-election Lease.
						apiserver.AccessLabelKey: pulumi.String(apiserver.AccessLabelValue),
						// Needs the Hue bridge(s) on the LAN to fetch light
						// data (at the IP hub-controller resolved) - see
						// network.go. No DNS label: this controller no
						// longer does any hostname-based discovery itself
						// (see pkg/components/hubcontroller).
						AccessLabelKey: pulumi.String(AccessLabelValue),
					},
				},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:    pulumi.String("lights-controller"),
							Image:   args.Image,
							Command: pulumi.StringArray{pulumi.String("/lights-controller")},
							Args: pulumi.StringArray{
								pulumi.Sprintf("--bridges-file=%s/bridges.json", mountPath),
								pulumi.Sprintf("--poll-interval=%s", args.PollInterval),
								pulumi.Sprintf("--dry-run=%v", args.DryRun),
							},
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{
									Name: pulumi.String("POD_NAMESPACE"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										FieldRef: &corev1.ObjectFieldSelectorArgs{
											FieldPath: pulumi.String("metadata.namespace"),
										},
									},
								},
							},
							VolumeMounts: corev1.VolumeMountArray{
								&corev1.VolumeMountArgs{
									Name:      pulumi.String(volumeName),
									MountPath: pulumi.String(mountPath),
									ReadOnly:  pulumi.Bool(true),
								},
							},
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/healthz"),
									Port: pulumi.Int(8081),
								},
								InitialDelaySeconds: pulumi.Int(5),
								PeriodSeconds:       pulumi.Int(10),
							},
							ReadinessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/readyz"),
									Port: pulumi.Int(8081),
								},
								InitialDelaySeconds: pulumi.Int(5),
								PeriodSeconds:       pulumi.Int(10),
							},
							// 128Mi OOM-killed this on first deploy - controller-runtime's
							// manager (client-go caches, leader election, health
							// server) has more baseline memory overhead than a bare
							// "small Go binary" estimate accounts for.
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("20m"),
									"memory": pulumi.String("64Mi"),
								},
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("200m"),
									"memory": pulumi.String("256Mi"),
								},
							},
						},
					},
					Volumes: corev1.VolumeArray{
						&corev1.VolumeArgs{
							Name: pulumi.String(volumeName),
							Secret: &corev1.SecretVolumeSourceArgs{
								SecretName: secret.Metadata.Name().Elem(),
							},
						},
					},
				},
			},
		},
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{secret}))...)
	if err != nil {
		return nil, err
	}

	// 6. Network policy - see network.go.
	if err := newNetworkPolicy(ctx, name, localOpts...); err != nil {
		return nil, err
	}

	if err := ctx.RegisterResourceOutputs(lc, pulumi.Map{
		"namespace": lc.Namespace,
	}); err != nil {
		return nil, err
	}

	return lc, nil
}
