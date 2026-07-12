// Package route provides a reusable Pulumi component for a Gateway API
// HTTPRoute attached to an existing Gateway (typically the shared Gateway
// from pkg/components/istio/gateway), routing one or more hostnames to a
// single backend Service.
//
// Out of scope for now (add later if needed): per-route TLS, header/query
// matching, weighted multi-backend traffic-splitting, request filters
// (redirects/rewrites).
package route

import (
	domain "github.com/liamawhite/homelab/pkg/components/cloudflare/domain"
	gatewayv1 "github.com/liamawhite/homelab/pkg/crds/gatewayapi/crds/kubernetes/gateway/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Route represents a single HTTPRoute attached to an existing Gateway.
type Route struct {
	pulumi.ResourceState

	Name      pulumi.StringOutput
	Namespace pulumi.StringOutput
}

// RouteArgs contains the configuration for a Route.
type RouteArgs struct {
	// Namespace is the HTTPRoute object's own namespace, typically the
	// backend Service's own namespace.
	Namespace pulumi.StringInput
	// Domains this route matches - each Domain's registered Hostname
	// becomes an entry in the HTTPRoute's Hostnames list. Passing the
	// actual Domain resources (rather than raw hostname strings) makes the
	// DNS record and this route's Host matching the same Pulumi-tracked
	// value instead of two independently built strings that only happen to
	// match by convention.
	Domains []*domain.Domain

	// GatewayName/GatewayNamespace identify the shared Gateway to attach
	// to (pkg/components/istio/gateway.Gateway.Name / .Namespace).
	GatewayName      pulumi.StringInput
	GatewayNamespace pulumi.StringInput

	// BackendServiceName/BackendServicePort identify the single backend
	// Service this route forwards all matched traffic to.
	BackendServiceName pulumi.StringInput
	BackendServicePort int

	// PathPrefix optionally restricts matching to a path prefix (e.g.
	// "/api"). Defaults to "/" (match everything) if empty.
	PathPrefix string
}

// NewRoute creates a new HTTPRoute attached to an existing Gateway.
func NewRoute(ctx *pulumi.Context, name string, args *RouteArgs, opts ...pulumi.ResourceOption) (*Route, error) {
	route := &Route{}

	err := ctx.RegisterComponentResource("homelab:istio:route", name, route, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(route))

	pathPrefix := args.PathPrefix
	if pathPrefix == "" {
		pathPrefix = "/"
	}

	hostnames := make(pulumi.StringArray, len(args.Domains))
	for i, d := range args.Domains {
		hostnames[i] = d.Hostname
	}

	// 1. Create the HTTPRoute
	_, err = gatewayv1.NewHTTPRoute(ctx, name, &gatewayv1.HTTPRouteArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &gatewayv1.HTTPRouteSpecArgs{
			Hostnames: hostnames,
			ParentRefs: gatewayv1.HTTPRouteSpecParentRefsArray{
				&gatewayv1.HTTPRouteSpecParentRefsArgs{
					Name:      args.GatewayName.ToStringPtrOutput(),
					Namespace: args.GatewayNamespace.ToStringPtrOutput(),
				},
			},
			Rules: gatewayv1.HTTPRouteSpecRulesArray{
				&gatewayv1.HTTPRouteSpecRulesArgs{
					Matches: gatewayv1.HTTPRouteSpecRulesMatchesArray{
						&gatewayv1.HTTPRouteSpecRulesMatchesArgs{
							Path: &gatewayv1.HTTPRouteSpecRulesMatchesPathArgs{
								Type:  pulumi.String("PathPrefix"),
								Value: pulumi.String(pathPrefix),
							},
						},
					},
					BackendRefs: gatewayv1.HTTPRouteSpecRulesBackendRefsArray{
						&gatewayv1.HTTPRouteSpecRulesBackendRefsArgs{
							Name: args.BackendServiceName.ToStringPtrOutput(),
							Port: pulumi.Int(args.BackendServicePort),
						},
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	route.Name = pulumi.String(name).ToStringOutput()
	route.Namespace = args.Namespace.ToStringOutput()

	// Register outputs
	if err := ctx.RegisterResourceOutputs(route, pulumi.Map{
		"name":      route.Name,
		"namespace": route.Namespace,
	}); err != nil {
		return nil, err
	}

	return route, nil
}
