// Package tailscale provides a Pulumi component for the Tailscale
// Kubernetes Operator - each app opts in by creating its own Ingress
// (ingressClassName: tailscale, see pkg/components/tailscale/ingress),
// getting a dynamically created proxy pod that joins the tailnet and
// forwards traffic in.
package tailscale

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/apiserver"
	"github.com/liamawhite/homelab/pkg/components/dns"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	tsv1alpha1 "github.com/liamawhite/homelab/pkg/crds/tailscale/crds/kubernetes/tailscale/v1alpha1"
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Operator represents the Tailscale Kubernetes Operator installation.
type Operator struct {
	pulumi.ResourceState

	Namespace pulumi.StringOutput
}

// OperatorArgs contains the configuration for Operator.
type OperatorArgs struct {
	// Namespace is where the operator (and its dynamically created
	// per-Ingress proxy pods) run - created centrally by
	// pkg/deploy/namespaces.go and passed in here, ambient-enrolled like
	// cloudflare/home - this component does not create it.
	Namespace pulumi.StringInput
	// Version is the tailscale-operator chart version to deploy (e.g.
	// versions.Tailscale).
	Version string
	// OAuthClientID and OAuthClientSecret authenticate the operator to the
	// tailnet - see infra.yaml's tailscale.oauthClientId/oauthClientSecret.
	// The operator mints its own short-lived auth keys from these rather
	// than using one static long-lived authkey.
	OAuthClientID     pulumi.StringInput
	OAuthClientSecret pulumi.StringInput
}

// NewOperator installs the Tailscale Kubernetes Operator.
func NewOperator(ctx *pulumi.Context, name string, args *OperatorArgs, opts ...pulumi.ResourceOption) (*Operator, error) {
	op := &Operator{}

	err := ctx.RegisterComponentResource("homelab:tailscale:operator", name, op, opts...)
	if err != nil {
		return nil, err
	}

	resourceOpts := append(opts, pulumi.Parent(op))

	// 1. Install the tailscale-operator Helm chart. Its own installCRDs
	// value (true by default, left unset here) installs this component's
	// CRDs (ProxyClass, Connector, ...) as part of this same release - see
	// pkg/crds/tailscale's doc comment for why there's no separate central
	// install step here, unlike Istio/Gateway API.
	chart, err := helmv4.NewChart(ctx, name, &helmv4.ChartArgs{
		Namespace: args.Namespace,
		Chart:     pulumi.String("tailscale-operator"),
		Version:   pulumi.String(args.Version),
		RepositoryOpts: &helmv4.RepositoryOptsArgs{
			Repo: pulumi.String("https://pkgs.tailscale.com/helmcharts"),
		},
		Values: pulumi.Map{
			"oauth": pulumi.Map{
				"clientId":     args.OAuthClientID,
				"clientSecret": args.OAuthClientSecret,
			},
			"operatorConfig": pulumi.Map{
				// Grants the operator's own control pod DNS access,
				// Tailscale control-plane/DERP/WireGuard access (see
				// allow-egress-tailscale below), and Kubernetes API server
				// access under Cilium's default-deny baseline - the
				// operator is a real controller that watches Ingress/
				// Secret/etc. resources and persists its own tailscaled
				// state in a k8s Secret; without apiserver access it can't
				// even start (confirmed live: "connection reset by peer"
				// reaching kubernetes.default.svc, then a fatal "creating
				// kube store" crash loop).
				"podLabels": pulumi.Map{
					dns.AccessLabelKey:       pulumi.String(dns.AccessLabelValue),
					apiserver.AccessLabelKey: pulumi.String(apiserver.AccessLabelValue),
					AccessLabelKey:           pulumi.String(AccessLabelValue),
				},
				// Matches the chart's own default - set explicitly (not
				// left implicit) so it can't silently drift from what
				// pkg/components/tailscale/acl declares as tagOwners for
				// this tag.
				"defaultTags": pulumi.StringArray{pulumi.String(OperatorTag)},
				"resources": pulumi.Map{
					"limits": pulumi.Map{
						"cpu":    pulumi.String("100m"),
						"memory": pulumi.String("128Mi"),
					},
					"requests": pulumi.Map{
						"cpu":    pulumi.String("20m"),
						"memory": pulumi.String("64Mi"),
					},
				},
			},
			"proxyConfig": pulumi.Map{
				// Every dynamically created per-Ingress proxy pod picks up
				// this ProxyClass by default (step 2 below) - without it,
				// those pods would have no way to get
				// AccessLabelKey/dns.AccessLabelKey under Cilium's
				// default-deny baseline.
				"defaultProxyClass": pulumi.String(DefaultProxyClassName),
				// Matches the chart's own default - set explicitly for the
				// same reason as operatorConfig.defaultTags above. Unlike
				// that field (a string array), the chart's values.yaml
				// takes a single string here.
				"defaultTags": pulumi.String(ProxyTag),
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// 2. ProxyClass applied by default to every proxy pod the operator
	// creates (step 1's proxyConfig.defaultProxyClass) - grants those pods
	// the same DNS/apiserver/Tailscale access the operator's own pod gets,
	// under Cilium's default-deny baseline. Proxy pods persist their own
	// tailscaled state in a k8s Secret the same way the operator does, so
	// they need apiserver access too. Deliberately does NOT set
	// istio.io/dataplane-mode: none - these pods must stay ambient-enrolled
	// so ztunnel captures their egress into an app's waypoint, exactly like
	// cloudflared (see pkg/components/cloudflare/tunnel).
	_, err = tsv1alpha1.NewProxyClass(ctx, fmt.Sprintf("%s-proxyclass", name), &tsv1alpha1.ProxyClassArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(DefaultProxyClassName),
		},
		Spec: &tsv1alpha1.ProxyClassSpecArgs{
			StatefulSet: &tsv1alpha1.ProxyClassSpecStatefulSetArgs{
				Pod: &tsv1alpha1.ProxyClassSpecStatefulSetPodArgs{
					Labels: pulumi.StringMap{
						dns.AccessLabelKey:       pulumi.String(dns.AccessLabelValue),
						apiserver.AccessLabelKey: pulumi.String(apiserver.AccessLabelValue),
						AccessLabelKey:           pulumi.String(AccessLabelValue),
					},
				},
			},
		},
	}, append(resourceOpts, pulumi.DependsOn([]pulumi.Resource{chart}))...)
	if err != nil {
		return nil, err
	}

	// 3. Only pods carrying AccessLabelKey/AccessLabelValue (the operator
	// itself and every proxy pod it creates) can reach Tailscale's control
	// plane/DERP/WireGuard - egress access is opt-in per workload, not
	// blanket, same as every other network policy in this repo. Tailscale's
	// edge IPs aren't expressible as a fixed CIDR, hence ToEntities "world"
	// restricted by port, same reasoning as
	// pkg/components/cloudflare/tunnel's matching policy. Port 443/TCP is
	// required (control plane + DERP-over-TCP fallback - Tailscale
	// deliberately designs this path to always work alone); ports
	// 41641/UDP (default WireGuard direct-connection port) and 3478/UDP
	// (STUN, for discovering direct-connection paths) are best-effort only
	// - narrower than Tailscale's own recommended blanket UDP allow, a
	// deliberate trade-off matching this repo's existing discipline
	// (cloudflared's own policy is similarly narrow) - if a peer uses a
	// different direct-connection port, traffic still works via
	// DERP-relay-over-443, just with relay latency instead of a direct
	// connection.
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-tailscale", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-tailscale"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					AccessLabelKey: pulumi.String(AccessLabelValue),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("world")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("443"), Protocol: pulumi.String("TCP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("41641"), Protocol: pulumi.String("UDP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("3478"), Protocol: pulumi.String("UDP")},
							},
						},
					},
				},
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// 4. Only pods carrying AccessLabelKey/AccessLabelValue (i.e. the
	// operator and its proxy pods) can reach waypoints that opt in via
	// WaypointAccessLabelKey/Value - needed for a Tailscale proxy pod to
	// actually deliver a tailnet request to an app's Service once ambient
	// routes it through that Service's waypoint, on the waypoint's HBONE
	// mesh listener (port 15008). Mirrors
	// pkg/components/cloudflare/tunnel's matching policy pair exactly; the
	// waypoint-to-app leg is specific to each app's own pods and lives with
	// that app instead (see pkg/components/istio/waypoint).
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-waypoints", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-tailscale-waypoints"),
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
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{
							MatchLabels: pulumi.StringMap{
								WaypointAccessLabelKey: pulumi.String(WaypointAccessLabelValue),
							},
						},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("15008"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// 5. Ingress counterpart to #4 - default-deny blocks ingress
	// independently of egress, so a waypoint that opted in via
	// WaypointAccessLabelKey also needs to actually accept the connection
	// (same lesson as istiod's own missing ingress policy, see
	// pkg/components/istio).
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-waypoints", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-tailscale-waypoints"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					WaypointAccessLabelKey: pulumi.String(WaypointAccessLabelValue),
				},
			},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{
							MatchLabels: pulumi.StringMap{
								AccessLabelKey: pulumi.String(AccessLabelValue),
							},
						},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String("15008"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	op.Namespace = args.Namespace.ToStringOutput()

	if err := ctx.RegisterResourceOutputs(op, pulumi.Map{
		"namespace": op.Namespace,
	}); err != nil {
		return nil, err
	}

	return op, nil
}
