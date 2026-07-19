// Package waypoint provides a Pulumi component for a single Istio ambient
// waypoint proxy, scoped to one Service. Waypoints in this repo are
// per-service, not shared across a namespace - each app creates and owns
// its own (see pkg/deploy/applications/home.go), so each service's
// AuthorizationPolicy/RequestAuthentication can be scoped and evolved
// independently rather than funneling through one shared enforcement point.
package waypoint

import (
	"fmt"
	"maps"

	"github.com/liamawhite/homelab/pkg/components/cilium"
	"github.com/liamawhite/homelab/pkg/components/dns"
	"github.com/liamawhite/homelab/pkg/components/istio"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	gatewayv1 "github.com/liamawhite/homelab/pkg/crds/gatewayapi/crds/kubernetes/gateway/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// mergeLabels returns a new pulumi.StringMap containing base plus every
// entry of extra (extra wins on key collision).
func mergeLabels(base, extra pulumi.StringMap) pulumi.StringMap {
	merged := maps.Clone(base)
	maps.Copy(merged, extra)
	return merged
}

// Waypoint represents a single Istio ambient waypoint proxy.
type Waypoint struct {
	pulumi.ResourceState

	Name      pulumi.StringOutput
	Namespace pulumi.StringOutput
}

// WaypointArgs contains the configuration for a Waypoint.
type WaypointArgs struct {
	// Namespace is where the waypoint proxy itself runs - not created/owned
	// by this component, same convention as everywhere else in this repo.
	Namespace pulumi.StringInput
	// Labels are merged into the waypoint proxy's own pod labels, in
	// addition to the istiod/DNS access this component always adds -
	// e.g. a label opting into pkg/components/cloudflare/tunnel's
	// waypoint-access policy, for apps actually exposed through that
	// tunnel. Left to the caller since not every waypoint needs it.
	Labels pulumi.StringMap
	// TargetLabels are the pod labels (e.g. {"app": "home"}) of the app
	// backend this waypoint fronts, in the same Namespace. If set, this
	// component creates the waypoint-to-app CiliumClusterwideNetworkPolicy
	// pair - egress from this specific waypoint to that specific app, and
	// the app's matching ingress counterpart (default-deny blocks both
	// directions independently) - scoped to this one app, not reusable
	// across others, unlike the cloudflared-to-waypoint policies in
	// pkg/components/cloudflare/tunnel. Left empty for a waypoint with no
	// backend to wire up yet.
	TargetLabels pulumi.StringMap
}

// ztunnelInboundPort is ztunnel's fixed HBONE listener port on every
// ambient-enrolled pod - NOT the app's own container port. A waypoint's
// connect_originate dials the app pod's real IP but always on this port
// (ztunnel terminates HBONE/mTLS here and only then forwards plaintext to
// the app's real port over loopback inside that pod's own netns - Cilium
// never sees that final hop as a separate flow). The egress/ingress CCNPs
// below have to allow this port, not the app's TargetPort - confirmed live
// via Hubble showing Cilium DROPPING waypoint->app SYNs on the app's real
// port with a policy verdict, while cloudflared->waypoint traffic (which
// already correctly targets this same port) went through fine - see issue
// investigation in cloudflare/tunnel's matching allow-ingress policy.
const ztunnelInboundPort = 15008

// NewWaypoint creates a single Istio ambient waypoint proxy. Traffic is
// routed through it by labeling the target (a Service, Pod, or Namespace)
// with istio.io/use-waypoint: <this Waypoint's Name> - this component only
// creates the waypoint itself, not that label (the caller's job, since only
// the caller knows what it wants routed through it).
func NewWaypoint(ctx *pulumi.Context, name string, args *WaypointArgs, opts ...pulumi.ResourceOption) (*Waypoint, error) {
	wp := &Waypoint{}

	err := ctx.RegisterComponentResource("homelab:istio:waypoint", name, wp, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(wp))

	// Base labels every waypoint needs (istiod/DNS access), plus whatever
	// the caller asked for on top.
	infraLabels := pulumi.StringMap{
		istio.AccessLabelKey: pulumi.String(istio.AccessLabelValue),
		dns.AccessLabelKey:   pulumi.String(dns.AccessLabelValue),
	}
	maps.Copy(infraLabels, args.Labels)

	// istiod auto-creates the "istio-waypoint" GatewayClass (controllerName
	// istio.io/waypoint-controller) - referenced by name here rather than
	// owned, same "don't own the class" pattern used for GatewayClass
	// "istio" before the shared ingress Gateway was removed.
	_, err = gatewayv1.NewGateway(ctx, name, &gatewayv1.GatewayArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
			Labels: pulumi.StringMap{
				// Explicitly set - istio.io docs don't confirm what this
				// defaults to if omitted, so don't rely on one.
				"istio.io/waypoint-for": pulumi.String("service"),
			},
		},
		Spec: &gatewayv1.GatewaySpecArgs{
			GatewayClassName: pulumi.String("istio-waypoint"),
			Listeners: gatewayv1.GatewaySpecListenersArray{
				&gatewayv1.GatewaySpecListenersArgs{
					Name:     pulumi.String("mesh"),
					Port:     pulumi.Int(15008),
					Protocol: pulumi.String("HBONE"),
				},
			},
			// The waypoint proxy istiod provisions from this Gateway is an
			// XDS client - needs istiod access under Cilium's default-deny
			// egress baseline, plus DNS to resolve istiod's hostname
			// (istiod.istio-system.svc) in the first place - the istiod
			// grant alone only covers the connection once that hostname
			// is already resolved.
			Infrastructure: &gatewayv1.GatewaySpecInfrastructureArgs{
				Labels: infraLabels,
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	if args.TargetLabels != nil {
		// Egress first, then its ingress counterpart - default-deny
		// blocks both directions independently (confirmed live with
		// istiod's own missing ingress policy, see pkg/components/istio).
		// Scoped to this specific waypoint's istiod-assigned gateway-name
		// identity and this specific app's own pod labels, not a shared
		// opt-in label - unlike pkg/components/cloudflare/tunnel's
		// cloudflared-to-waypoint policies, which are reusable across
		// every app's waypoint, this leg is a 1:1 relationship intrinsic
		// to this one waypoint/app pair.
		gatewayNameMatch := pulumi.StringMap{
			cilium.K8sNamespaceLabel: args.Namespace,
			GatewayNameLabel:         pulumi.String(name),
		}
		hbonePort := fmt.Sprintf("%d", ztunnelInboundPort)

		_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-to-app", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String(fmt.Sprintf("allow-egress-%s-to-app", name)),
			},
			Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
				EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
					MatchLabels: gatewayNameMatch,
				},
				Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
					&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
						ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
							&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{
								MatchLabels: mergeLabels(pulumi.StringMap{cilium.K8sNamespaceLabel: args.Namespace}, args.TargetLabels),
							},
						},
						ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
							&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
								Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
									&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String(hbonePort), Protocol: pulumi.String("TCP")},
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

		_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-from-waypoint", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String(fmt.Sprintf("allow-ingress-%s-from-waypoint", name)),
			},
			Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
				EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
					MatchLabels: mergeLabels(pulumi.StringMap{cilium.K8sNamespaceLabel: args.Namespace}, args.TargetLabels),
				},
				Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
					&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
						FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
							&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{
								MatchLabels: gatewayNameMatch,
							},
						},
						ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArray{
							&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArgs{
								Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArray{
									&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String(hbonePort), Protocol: pulumi.String("TCP")},
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
	}

	wp.Name = pulumi.String(name).ToStringOutput()
	wp.Namespace = args.Namespace.ToStringOutput()

	// Register outputs
	if err := ctx.RegisterResourceOutputs(wp, pulumi.Map{
		"name":      wp.Name,
		"namespace": wp.Namespace,
	}); err != nil {
		return nil, err
	}

	return wp, nil
}
