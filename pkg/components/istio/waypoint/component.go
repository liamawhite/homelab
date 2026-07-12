// Package waypoint provides a Pulumi component for a single Istio ambient
// waypoint proxy, scoped to one Service. Waypoints in this repo are
// per-service, not shared across a namespace - each app creates and owns
// its own (see pkg/deploy/applications/home.go), so each service's
// AuthorizationPolicy/RequestAuthentication can be scoped and evolved
// independently rather than funneling through one shared enforcement point.
package waypoint

import (
	gatewayv1 "github.com/liamawhite/homelab/pkg/crds/gatewayapi/crds/kubernetes/gateway/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

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
}

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
		},
	}, localOpts...)
	if err != nil {
		return nil, err
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
