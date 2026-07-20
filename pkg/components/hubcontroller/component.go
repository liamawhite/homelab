// Package hubcontroller deploys hub-controller (see
// applications/lights-controller/cmd/hub-controller): a Deployment that
// discovers Philips Hue bridges via SSDP and publishes their current IP
// as HueBridge custom resources (see pkg/crds/lights), for
// pkg/components/lightscontroller to read instead of discovering bridges
// itself.
//
// Runs with HostNetwork: true - SSDP's multicast group-join can't work
// from a normal pod under this cluster's CNI (confirmed live via `cilium
// monitor --type drop`: an "Unknown L4 protocol" drop on the IGMP
// membership report, beneath the level any NetworkPolicy can act on).
// hostNetwork moves this pod's networking into the node's own network
// namespace, bypassing the CNI overlay entirely.
//
// Deliberately NO CiliumClusterwideNetworkPolicy here, unlike every other
// component in this repo: confirmed via a real precedent already running
// in this cluster - kube-vip (pkg/components/kubevip) also runs
// HostNetwork: true, also needs apiserver access, and has zero
// network-policy wiring anywhere in its package, because this cluster
// never enabled Cilium's host-firewall feature. hostNetwork pods simply
// aren't subject to the pod-level default-deny baseline at all. This is a
// real, deliberate tradeoff (this pod's network access is effectively
// unrestricted, unlike every other pod in the cluster) - the reason this
// component exists at all, rather than just making lights-controller
// itself hostNetwork, is to confine that tradeoff to the smallest
// possible surface.
//
// The namespace itself (including the Istio ambient-mode label, which -
// per the same reasoning above - doesn't actually apply to this
// component's hostNetwork pod either) is created centrally by
// pkg/deploy/namespaces.go, not by this component.
package hubcontroller

import (
	"fmt"
	"strings"

	"github.com/liamawhite/homelab/pkg/config"
	lightsv1alpha1 "github.com/liamawhite/homelab/pkg/crds/lights/crds/kubernetes/lights/v1alpha1"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// hueBridgeResourceName maps a Hue bridge ID (uppercase hex, as stored in
// infra.yaml) to the HueBridge CR name that identifies it - lowercased,
// since Kubernetes object names must be a lowercase RFC 1123 subdomain.
// Must match applications/lights-controller/internal/bridges.ResourceName
// exactly: this package creates each HueBridge CR under that name, and
// hub-controller/lights-controller look it up by the same name.
func hueBridgeResourceName(bridgeID string) string {
	return strings.ToLower(bridgeID)
}

const serviceAccountName = "hub-controller"

// HubController represents the deployed hub-controller.
type HubController struct {
	pulumi.ResourceState

	Namespace pulumi.StringOutput
}

// HubControllerArgs contains the configuration for HubController.
type HubControllerArgs struct {
	// Namespace is created centrally by pkg/deploy/namespaces.go and
	// passed in here - this component does not create it.
	Namespace pulumi.StringInput
	// Bridges is infraCfg.Lights.Hue.Bridges - every paired bridge's id,
	// used only to create one HueBridge CR per bridge below (appKey is
	// unused here - only lightscontroller needs it, to authenticate to a
	// bridge's data API).
	Bridges []config.HueBridgeConfig
	// PollInterval is how often the controller runs an SSDP discovery
	// round, e.g. "60s".
	PollInterval pulumi.StringInput
	// Image is the shared lights-controller/hub-controller image
	// (built once in pkg/deploy/image.go and passed to both components).
	Image pulumi.StringInput
}

// NewHubController creates the hub-controller Deployment, its RBAC, and
// the Secret carrying paired bridge IDs.
func NewHubController(ctx *pulumi.Context, name string, args *HubControllerArgs, opts ...pulumi.ResourceOption) (*HubController, error) {
	hc := &HubController{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:hub-controller", name, hc, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(hc))

	hc.Namespace = args.Namespace.ToStringOutput()

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

	// 2. ClusterRole/ClusterRoleBinding for the HueBridge CRD - cluster
	// scoped, so this can't be a namespaced Role. Read-only on the base
	// object: Pulumi (this very component, see step 5 below) owns
	// creating/deleting HueBridge CRs declaratively from infra.yaml, so
	// the controller only ever syncs status onto an object that's
	// already there.
	clusterRole, err := rbacv1.NewClusterRole(ctx, fmt.Sprintf("%s-cr", name), &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("hub-controller"),
		},
		Rules: rbacv1.PolicyRuleArray{
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("huebridges")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("list"), pulumi.String("watch")},
			},
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("lights.homelab.internal")},
				Resources: pulumi.StringArray{pulumi.String("huebridges/status")},
				Verbs:     pulumi.StringArray{pulumi.String("get"), pulumi.String("update"), pulumi.String("patch")},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	_, err = rbacv1.NewClusterRoleBinding(ctx, fmt.Sprintf("%s-crb", name), &rbacv1.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("hub-controller"),
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

	// 3. Namespaced Role/RoleBinding for leader-election Leases + Events -
	// same pattern (and same "events is forbidden" lesson) as
	// pkg/components/lightscontroller.
	leaseRole, err := rbacv1.NewRole(ctx, fmt.Sprintf("%s-lease-role", name), &rbacv1.RoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("hub-controller-leases"),
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
			Name:      pulumi.String("hub-controller-leases"),
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

	// 4. One HueBridge CR per configured bridge, created here rather than
	// by the controller itself: infra.yaml's lights.hue.bridges is the
	// declarative source of truth for which bridges exist, the same way
	// every other resource in this repo is Pulumi-owned - the controller
	// (see applications/lights-controller/internal/hubcontroller) only
	// syncs status (ip/reachable/etc.) onto these, it never creates or
	// deletes a HueBridge itself. Spec is empty (see
	// applications/lights-controller/api/v1alpha1/huebridge_types.go) -
	// this object exists purely to carry a name and a status.
	for _, bridge := range args.Bridges {
		_, err := lightsv1alpha1.NewHueBridge(ctx, fmt.Sprintf("%s-huebridge-%s", name, hueBridgeResourceName(bridge.ID)), &lightsv1alpha1.HueBridgeArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String(hueBridgeResourceName(bridge.ID)),
			},
		}, localOpts...)
		if err != nil {
			return nil, err
		}
	}

	// 5. Deployment. HostNetwork: true is the entire point of this
	// component - see the package doc comment. DNSPolicy must be set
	// explicitly: Kubernetes silently downgrades a hostNetwork pod's
	// effective DNS policy to the node's own /etc/resolv.conf unless
	// ClusterFirstWithHostNet is set, and this controller needs reliable
	// DNS (todo: only actually true if/when nupnp is ever added back as a
	// fallback - SSDP itself needs no DNS - kept anyway since it's the
	// correct default for a hostNetwork pod regardless). Single replica -
	// leader election (see the lease RBAC above) makes more than one safe
	// later, but nothing needs it yet. Image is the shared
	// lights-controller/hub-controller image (see pkg/deploy/image.go) -
	// Command picks which of its two binaries this Deployment runs. No
	// mounted config: which bridges to sync comes from listing HueBridge
	// CRs directly (step 4 above), not a bridges.json file.
	_, err = appsv1.NewDeployment(ctx, fmt.Sprintf("%s-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("hub-controller"),
			Namespace: args.Namespace,
			Labels: pulumi.StringMap{
				"app": pulumi.String("hub-controller"),
			},
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			// HostNetwork means this pod binds ports (8080/8081) directly
			// on the node - the default RollingUpdate strategy brings up
			// the new pod before killing the old one, which collides on
			// those host ports when (as here) both replicas land on the
			// same node. Recreate kills the old pod first.
			Strategy: &appsv1.DeploymentStrategyArgs{
				Type: pulumi.String("Recreate"),
			},
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app": pulumi.String("hub-controller"),
				},
			},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: pulumi.StringMap{
						"app": pulumi.String("hub-controller"),
					},
				},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
					HostNetwork:        pulumi.Bool(true),
					DnsPolicy:          pulumi.String("ClusterFirstWithHostNet"),
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:    pulumi.String("hub-controller"),
							Image:   args.Image,
							Command: pulumi.StringArray{pulumi.String("/hub-controller")},
							Args: pulumi.StringArray{
								pulumi.Sprintf("--poll-interval=%s", args.PollInterval),
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
							// Same starting point as lightscontroller's -
							// see that component's identical comment about
							// 128Mi OOM-killing it on first deploy.
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
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	if err := ctx.RegisterResourceOutputs(hc, pulumi.Map{
		"namespace": hc.Namespace,
	}); err != nil {
		return nil, err
	}

	return hc, nil
}
