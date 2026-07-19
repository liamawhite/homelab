package istio

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	"github.com/liamawhite/homelab/pkg/components/cilium"
	"github.com/liamawhite/homelab/pkg/components/dns"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
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
			// istiod watches the K8s API directly (Namespaces, CRDs,
			// Secrets, leader-election leases, etc.) - needs apiserver
			// access under Cilium's default-deny egress baseline. It also
			// fetches JWKS for any RequestAuthentication with a remote
			// jwksUri itself, at config-push time (see
			// pkg/components/cloudflare/accessjwt's own egress policy) -
			// needs DNS to resolve that JWKS host's name in the first
			// place, confirmed live: istiod had apiserver access and its
			// own JWKS-fetch egress grant, but no DNS access at all, so
			// it could never resolve the hostname to attempt the fetch.
			"podLabels": pulumi.StringMap{
				apiserver.AccessLabelKey: pulumi.String(apiserver.AccessLabelValue),
				dns.AccessLabelKey:       pulumi.String(dns.AccessLabelValue),
			},
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
			// istio-cni pushes config/status via istiod's XDS and watches
			// Pods directly for ambient redirection - needs istiod and
			// apiserver access under Cilium's default-deny egress
			// baseline, plus DNS to resolve istiod's hostname
			// (istiod.istio-system.svc) in the first place - the istiod
			// grant alone only covers the connection once that hostname
			// is already resolved.
			"podLabels": pulumi.StringMap{
				AccessLabelKey:           pulumi.String(AccessLabelValue),
				apiserver.AccessLabelKey: pulumi.String(apiserver.AccessLabelValue),
				dns.AccessLabelKey:       pulumi.String(dns.AccessLabelValue),
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
			// ztunnel is an istiod XDS consumer (per this component's own
			// doc comment above) - needs istiod access under Cilium's
			// default-deny egress baseline, plus DNS to resolve istiod's
			// hostname (istiod.istio-system.svc) in the first place - the
			// istiod grant alone only covers the connection once that
			// hostname is already resolved.
			"podLabels": pulumi.StringMap{
				AccessLabelKey:     pulumi.String(AccessLabelValue),
				dns.AccessLabelKey: pulumi.String(dns.AccessLabelValue),
			},
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

	// 6. Only pods carrying AccessLabelKey/AccessLabelValue can reach
	// istiod's XDS ports - istiod access is opt-in per workload, not
	// blanket, so every consumer (ztunnel, istio-cni, waypoints, gateways)
	// is explicit about depending on it. Requires the Cilium
	// CiliumClusterwideNetworkPolicy CRD to already exist - callers must
	// pass pulumi.DependsOn on the Cilium installation (see
	// pkg/components/cilium.NewCilium).
	istiodMatchLabels := pulumi.StringMap{
		cilium.K8sNamespaceLabel: args.Namespace,
		"app":                    pulumi.String("istiod"),
	}
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-istiod", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-istiod"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					AccessLabelKey: pulumi.String(AccessLabelValue),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{MatchLabels: istiodMatchLabels},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("15012"), Protocol: pulumi.String("TCP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("15010"), Protocol: pulumi.String("TCP")},
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

	// istiod also needs an INGRESS allow to match - the egress policy above
	// only lets a labeled client's traffic leave; "default-deny" blocks all
	// ingress cluster-wide too, so without this istiod's pod silently drops
	// the connection before it ever reaches the process (no server-side log
	// entry at all - confirmed live, this exact gap left ztunnel/istio-cni
	// stuck retrying a TCP connect indefinitely). Mirrors allow-ingress-dns.
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-istiod", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-istiod"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: istiodMatchLabels,
			},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String("15012"), Protocol: pulumi.String("TCP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String("15010"), Protocol: pulumi.String("TCP")},
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

	// istiod fetches JWKS for any RequestAuthentication with a remote
	// jwksUri itself, at config-push time, embedding the keys statically
	// into what it pushes to the enforcing waypoint rather than that
	// waypoint fetching them live (confirmed live: the waypoint's own
	// Envoy had no remote JWKS cluster at all, and istiod's own logs
	// showed the actual fetch attempts/failures). A property of istiod
	// itself, not anything specific to Cloudflare Access
	// (pkg/components/cloudflare/accessjwt) or any other JWT issuer, so
	// it's a fixed baseline policy here rather than something each
	// RequestAuthentication caller creates. JWKS hosts aren't a fixed
	// CIDR, hence ToEntities "world" restricted by port, same reasoning
	// as allow-egress-coredns-upstream (pkg/components/dns).
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-jwks", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-jwks"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: istiodMatchLabels,
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("world")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("443"), Protocol: pulumi.String("TCP")},
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

	// Register outputs
	if err := ctx.RegisterResourceOutputs(istio, pulumi.Map{
		"namespace": istio.Namespace,
	}); err != nil {
		return nil, err
	}

	return istio, nil
}
