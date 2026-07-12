// Package applications holds actual app deployments that route through the
// shared Istio Gateway (pkg/components/istio/gateway), as opposed to
// pkg/components which holds reusable infrastructure primitives.
package applications

import (
	"fmt"

	domain "github.com/liamawhite/homelab/pkg/components/cloudflare/domain"
	tunnel "github.com/liamawhite/homelab/pkg/components/cloudflare/tunnel"
	route "github.com/liamawhite/homelab/pkg/components/istio/route"
	waypoint "github.com/liamawhite/homelab/pkg/components/istio/waypoint"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// echoImage is a tiny fixed-response HTTP server. Gateway API's
	// HTTPRoute has no native "direct response" filter (unlike Istio's own
	// VirtualService.DirectResponse), so every route needs a real
	// backendRef - this stands in for a static response body.
	echoImage = "hashicorp/http-echo:1.0.0"
	echoPort  = 5678

	responseText = "good response"

	// Subdomain is this app's hostname prefix - reachable at
	// "<Subdomain>.<Domain>". Exported so pkg/deploy/domains.go can
	// register the matching public DNS record without duplicating the
	// literal.
	Subdomain = "homelab"
)

// Home represents a minimal "homelab.<domain>" health-check app: a fixed
// 200 response, routed through the shared Gateway.
type Home struct {
	pulumi.ResourceState

	Namespace   pulumi.StringOutput
	ServiceName pulumi.StringOutput
}

// TunnelRoute returns this app's own Cloudflare Tunnel ingress rule -
// pkg/components/cloudflare/tunnel.Tunnel has no way to accept routes
// registered independently (Cloudflare's API models a tunnel's ingress list
// as a single object, one write replaces it all), so this only hands back
// data for whatever central place collects every app's route into that one
// list. Not yet called from pkg/deploy/deploy.go - Home is created after
// Tunnel there today, so wiring this in requires reordering that pass first.
func (h *Home) TunnelRoute() tunnel.TunnelRoute {
	return tunnel.TunnelRoute{
		Subdomain:        Subdomain,
		ServiceName:      h.ServiceName,
		ServiceNamespace: h.Namespace,
		ServicePort:      80,
	}
}

// HomeArgs contains the configuration for Home.
type HomeArgs struct {
	// Namespace is the namespace this app's backend runs in. Uses the
	// pre-existing "default" namespace for now (not created by us - no
	// conflict risk) rather than adding a new managed namespace for a
	// single demo app.
	Namespace pulumi.StringInput

	// Domains are the public hostnames this app is reachable at - the
	// Domain resource(s) pkg/deploy/domains.go registered for this app
	// (keyed by the Subdomain constant below), passed straight through to
	// the Route.
	Domains []*domain.Domain

	// GatewayName/GatewayNamespace identify the shared Gateway to attach
	// to (pkg/components/istio/gateway.Gateway.Name / .Namespace).
	GatewayName      pulumi.StringInput
	GatewayNamespace pulumi.StringInput
}

// NewHome deploys the home health-check app and its route.
func NewHome(ctx *pulumi.Context, name string, args *HomeArgs, opts ...pulumi.ResourceOption) (*Home, error) {
	home := &Home{}

	err := ctx.RegisterComponentResource("homelab:applications:home", name, home, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(home))

	labels := pulumi.StringMap{"app": pulumi.String(name)}

	// 1. Dedicated ServiceAccount for this app - every app gets its own
	// rather than running as its namespace's shared "default" account, so
	// RBAC can be scoped per-app later without a retrofit.
	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 2. Deploy the echo backend
	deployment, err := appsv1.NewDeployment(ctx, fmt.Sprintf("%s-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
			Labels:    labels,
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Selector: &metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{Labels: labels},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("http-echo"),
							Image: pulumi.String(echoImage),
							Args: pulumi.StringArray{
								pulumi.Sprintf("-text=%s", responseText),
							},
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{ContainerPort: pulumi.Int(echoPort)},
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("50m"),
									"memory": pulumi.String("32Mi"),
								},
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("10m"),
									"memory": pulumi.String("16Mi"),
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

	// 3. Dedicated waypoint for this app's Service - waypoints in this repo
	// are per-service, not shared across a namespace, so each app's
	// AuthorizationPolicy/RequestAuthentication can be scoped and evolved
	// independently rather than funneling through one shared enforcement
	// point.
	wp, err := waypoint.NewWaypoint(ctx, fmt.Sprintf("%s-waypoint", name), &waypoint.WaypointArgs{
		Namespace: args.Namespace,
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 4. Expose it via a Service, routed through the waypoint above
	// (istio.io/use-waypoint - the label ambient uses to assign traffic to
	// a waypoint; supported on Service/Pod/Namespace, not ServiceAccount).
	service, err := corev1.NewService(ctx, fmt.Sprintf("%s-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
			Labels: pulumi.StringMap{
				"istio.io/use-waypoint": wp.Name,
			},
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: labels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{
					Port:       pulumi.Int(80),
					TargetPort: pulumi.Int(echoPort),
				},
			},
		},
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{deployment}))...)
	if err != nil {
		return nil, err
	}

	// 5. Route its registered domain(s) to it via the shared Gateway
	_, err = route.NewRoute(ctx, name, &route.RouteArgs{
		Namespace:          args.Namespace,
		Domains:            args.Domains,
		GatewayName:        args.GatewayName,
		GatewayNamespace:   args.GatewayNamespace,
		BackendServiceName: service.Metadata.Name().Elem(),
		BackendServicePort: 80,
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{service}))...)
	if err != nil {
		return nil, err
	}

	home.Namespace = args.Namespace.ToStringOutput()
	home.ServiceName = service.Metadata.Name().Elem()

	// Register outputs
	if err := ctx.RegisterResourceOutputs(home, pulumi.Map{
		"namespace":   home.Namespace,
		"serviceName": home.ServiceName,
	}); err != nil {
		return nil, err
	}

	return home, nil
}
