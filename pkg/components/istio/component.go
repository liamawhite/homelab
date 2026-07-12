package istio

import (
	"fmt"

	securityv1 "github.com/liamawhite/homelab/pkg/crds/istio/crds/kubernetes/security/v1"
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Istio represents the Istio service mesh control plane (istiod, CNI,
// ztunnel) plus the mesh-wide security baseline (waypoint-class default-deny
// AuthorizationPolicy, default STRICT PeerAuthentication) - every mesh this
// component deploys comes with that baseline, rather than callers needing to
// remember to wire it up separately. Ingress is handled separately by
// pkg/components/istio/gateway via Gateway API, which Istio auto-deploys a
// proxy for once this control plane and a Gateway object referencing its
// auto-created "istio" GatewayClass both exist.
type Istio struct {
	pulumi.ResourceState

	Namespace pulumi.StringOutput // echoes IstioArgs.Namespace, not owned by this component
}

// IstioArgs contains the configuration for Istio
type IstioArgs struct {
	// Version is the Istio version to deploy (e.g., "1.30.2")
	Version string
	// Namespace is istio-system's name, created centrally by
	// pkg/deploy/namespaces.go and passed in here - this component does not
	// create it.
	Namespace pulumi.StringInput
}

// NewIstio creates a new Istio component with ambient mesh profile
func NewIstio(ctx *pulumi.Context, name string, args *IstioArgs, opts ...pulumi.ResourceOption) (*Istio, error) {
	istio := &Istio{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:istio", name, istio, opts...)
	if err != nil {
		return nil, err
	}

	// All child resources should be parented to this component
	localOpts := append(opts, pulumi.Parent(istio))

	istio.Namespace = args.Namespace.ToStringOutput()

	// Helm repository configuration
	repositoryOpts := &helmv4.RepositoryOptsArgs{
		Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
	}

	// 1. Install istiod (control plane)
	_, err = helmv4.NewChart(ctx, fmt.Sprintf("%s-istiod", name), &helmv4.ChartArgs{
		Namespace:      args.Namespace,
		Chart:          pulumi.String("istiod"),
		Version:        pulumi.String(args.Version),
		RepositoryOpts: repositoryOpts,
		Values: pulumi.Map{
			"profile": pulumi.String("ambient"),
			"pilot": pulumi.Map{
				"resources": pulumi.Map{
					"limits": pulumi.Map{
						"cpu":    pulumi.String("200m"),
						"memory": pulumi.String("128Mi"),
					},
					"requests": pulumi.Map{
						"cpu":    pulumi.String("20m"),
						"memory": pulumi.String("64Mi"),
					},
				},
			},
			"meshConfig": pulumi.Map{
				"accessLogFile": pulumi.String("/dev/stdout"),
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 2. Install CNI plugin (K3s-specific paths)
	_, err = helmv4.NewChart(ctx, fmt.Sprintf("%s-cni", name), &helmv4.ChartArgs{
		Namespace:      args.Namespace,
		Chart:          pulumi.String("cni"),
		Version:        pulumi.String(args.Version),
		RepositoryOpts: repositoryOpts,
		Values: pulumi.Map{
			"profile": pulumi.String("ambient"),
			"global": pulumi.Map{
				"platform": pulumi.String("k3s"),
			},
			// K3s-specific CNI paths
			// https://github.com/k3s-io/k3s/issues/11076
			"cni": pulumi.Map{
				"cniConfDir": pulumi.String("/var/lib/rancher/k3s/agent/etc/cni/net.d"),
				"cniBinDir":  pulumi.String("/var/lib/rancher/k3s/data/cni"),
				"resources": pulumi.Map{
					"limits": pulumi.Map{
						"cpu":    pulumi.String("100m"),
						"memory": pulumi.String("64Mi"),
					},
					"requests": pulumi.Map{
						"cpu":    pulumi.String("10m"),
						"memory": pulumi.String("32Mi"),
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 3. Install ztunnel (zero-trust tunnel for ambient mesh)
	_, err = helmv4.NewChart(ctx, fmt.Sprintf("%s-ztunnel", name), &helmv4.ChartArgs{
		Namespace:      args.Namespace,
		Chart:          pulumi.String("ztunnel"),
		Version:        pulumi.String(args.Version),
		RepositoryOpts: repositoryOpts,
		Values: pulumi.Map{
			"resources": pulumi.Map{
				"limits": pulumi.Map{
					"cpu":    pulumi.String("200m"),
					"memory": pulumi.String("128Mi"),
				},
				"requests": pulumi.Map{
					"cpu":    pulumi.String("20m"),
					"memory": pulumi.String("96Mi"),
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 4. Mesh-wide default STRICT mTLS: only has teeth for workloads
	// ztunnel actually captures - it's the caller's istio.io/dataplane-mode:
	// ambient namespace label (pkg/deploy/namespaces.go) that puts a
	// workload's traffic through ztunnel in the first place; this policy
	// just refuses to accept plaintext from workloads that are. Same
	// move-with-alias reasoning as default-deny above.
	_, err = securityv1.NewPeerAuthentication(ctx, fmt.Sprintf("%s-default-mtls", name), &securityv1.PeerAuthenticationArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("default"),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &securityv1.PeerAuthenticationSpecArgs{
			Mtls: &securityv1.PeerAuthenticationSpecMtlsArgs{
				Mode: pulumi.String("STRICT"),
			},
		},
	}, append(localOpts, pulumi.Aliases([]pulumi.Alias{
		{Name: pulumi.String("mesh-default-mtls"), NoParent: pulumi.Bool(true)},
	}))...)
	if err != nil {
		return nil, err
	}

	// 5. Waypoint-class default-deny: targets the istio-waypoint GatewayClass
	// itself via targetRefs (selector-based policies are ignored by
	// waypoints entirely - "selector policies will be ignored", istio.io),
	// so this applies to every waypoint in the cluster (present and future -
	// e.g. pkg/deploy/applications/home.go's own waypoint). Empty spec/rules
	// means implicit deny-all, requiring an explicit targetRefs-scoped
	// ALLOW (e.g. targeting a specific Service) to open anything up. This is
	// now the only mesh-wide default-deny - workloads outside any waypoint
	// (e.g. cloudflared) rely on their own specific ALLOW policies rather
	// than a blanket mesh-wide deny.
	_, err = securityv1.NewAuthorizationPolicy(ctx, fmt.Sprintf("%s-waypoint-default-deny", name), &securityv1.AuthorizationPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("waypoint-default-deny"),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &securityv1.AuthorizationPolicySpecArgs{
			TargetRefs: securityv1.AuthorizationPolicySpecTargetRefsArray{
				&securityv1.AuthorizationPolicySpecTargetRefsArgs{
					Group: pulumi.String("gateway.networking.k8s.io"),
					Kind:  pulumi.String("GatewayClass"),
					Name:  pulumi.String("istio-waypoint"),
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// Register outputs
	if err := ctx.RegisterResourceOutputs(istio, pulumi.Map{
		"namespace": istio.Namespace,
	}); err != nil {
		return nil, err
	}

	return istio, nil
}
