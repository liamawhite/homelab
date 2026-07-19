// Package applications holds actual app deployments, each fronted by its
// own Istio ambient waypoint (pkg/components/istio/waypoint) rather than a
// shared ingress Gateway, as opposed to pkg/components which holds reusable
// infrastructure primitives.
package applications

import (
	"fmt"

	waypoint "github.com/liamawhite/homelab/pkg/components/istio/waypoint"
	"github.com/liamawhite/homelab/pkg/components/tailscale"
	"github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// echoImage is a tiny fixed-response HTTP server. Gateway API's
	// HTTPRoute has no native "direct response" filter (unlike Istio's own
	// VirtualService.DirectResponse), so every route needs a real
	// backendRef - this stands in for a static response body. Shared with
	// public.go (same package - Go allows exactly one definition, not a
	// new abstraction).
	echoImage = "hashicorp/http-echo:1.0.0"
	echoPort  = 5678

	responseText = "good response"
)

// Private represents the Tailscale-only half of the health-check demo: a
// fixed 200 response, reachable directly at
// "<name>.<your-tailnet's-MagicDNS-suffix>", plus a Cloudflare-side
// redirect bookmark (see pkg/components/tailscale/ingress). See Public
// (public.go) for the Cloudflare-Tunnel-only counterpart - the two are
// split by exposure mechanism rather than one app wired to both, sharing
// HealthNamespace.
type Private struct {
	pulumi.ResourceState

	Namespace   pulumi.StringOutput
	ServiceName pulumi.StringOutput

	redirect ingress.RedirectRoute
}

// TailscaleRedirect returns this app's Cloudflare-redirect data -
// pkg/components/tailscale/ingress.NewIngress has no way to apply this
// independently (Cloudflare's Rulesets API models a zone+phase's whole rule
// list as a single object), so this only hands back data for whatever
// central place collects every app's route into that one Ruleset (see
// pkg/deploy/redirects.go).
func (p *Private) TailscaleRedirect() ingress.RedirectRoute {
	return p.redirect
}

// PrivateArgs contains the configuration for Private.
type PrivateArgs struct {
	// Namespace is the namespace this app's backend runs in - created
	// centrally by pkg/deploy/namespaces.go (HealthNamespace) and passed in
	// here, ambient-enrolled like istio-system/cloudflare/tailscale. This
	// component does not create it.
	Namespace pulumi.StringInput

	// TailscaleOperatorNamespace is where pkg/components/tailscale's
	// operator (and its dynamically created per-Ingress proxy pods) run -
	// passed through to ingress.NewIngress so it can restrict its
	// AuthorizationPolicy bypass to that namespace's identities.
	TailscaleOperatorNamespace pulumi.StringInput
	// TailscaleMagicDNSSuffix is infraCfg.Tailscale.MagicDNSSuffix - your
	// tailnet's real MagicDNS suffix, used to build this app's redirect
	// target URL.
	TailscaleMagicDNSSuffix pulumi.StringInput

	// CloudflareZoneID is the Cloudflare zone this app's redirect DNS
	// record belongs to - precomputed once in pkg/deploy (see
	// pkg/deploy/zone.go) and shared across every caller.
	CloudflareZoneID pulumi.StringInput
	// CloudflareBaseDomain is infraCfg.Cloudflare.Tunnel.Domain.
	CloudflareBaseDomain pulumi.StringInput
	// CloudflareProvider is the Cloudflare provider to create the redirect
	// DNS record with.
	CloudflareProvider *cloudflare.Provider
}

// NewPrivate deploys the private health-check app and its Tailscale ingress.
func NewPrivate(ctx *pulumi.Context, name string, args *PrivateArgs, opts ...pulumi.ResourceOption) (*Private, error) {
	private := &Private{}

	err := ctx.RegisterComponentResource("homelab:applications:private", name, private, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(private))

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
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/"),
									Port: pulumi.Int(echoPort),
								},
								InitialDelaySeconds: pulumi.Int(5),
								PeriodSeconds:       pulumi.Int(10),
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
	// AuthorizationPolicy can be scoped and evolved independently rather
	// than funneling through one shared enforcement point. Opts into
	// pkg/components/tailscale's waypoint-access policy since Private is
	// reachable through Tailscale (see step 5 below). TargetLabels has the
	// waypoint component wire up its own network policy to this app's
	// backend.
	wp, err := waypoint.NewWaypoint(ctx, fmt.Sprintf("%s-waypoint", name), &waypoint.WaypointArgs{
		Namespace: args.Namespace,
		Labels: pulumi.StringMap{
			tailscale.WaypointAccessLabelKey: pulumi.String(tailscale.WaypointAccessLabelValue),
		},
		TargetLabels: labels,
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

	// 5. Put this Service on Tailscale: the k8s Ingress the operator
	// reconciles into a tailnet-joined proxy pod, the AuthorizationPolicy
	// bypass that traffic needs to reach this Service through its waypoint
	// (no Cloudflare-Access-style policy here at all - auth is fully
	// delegated to tailnet membership/ACLs), and the Cloudflare-side
	// redirect bookkeeping.
	tsIngress, err := ingress.NewIngress(ctx, name, &ingress.IngressArgs{
		Namespace:            args.Namespace,
		ServiceName:          service.Metadata.Name().Elem(),
		ServicePort:          80,
		Hostname:             name,
		OperatorNamespace:    args.TailscaleOperatorNamespace,
		MagicDNSSuffix:       args.TailscaleMagicDNSSuffix,
		CloudflareZoneID:     args.CloudflareZoneID,
		CloudflareBaseDomain: args.CloudflareBaseDomain,
		CloudflareProvider:   args.CloudflareProvider,
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{service, wp}))...)
	if err != nil {
		return nil, err
	}

	private.Namespace = args.Namespace.ToStringOutput()
	private.ServiceName = service.Metadata.Name().Elem()
	private.redirect = tsIngress.Redirect

	// Register outputs
	if err := ctx.RegisterResourceOutputs(private, pulumi.Map{
		"namespace":   private.Namespace,
		"serviceName": private.ServiceName,
	}); err != nil {
		return nil, err
	}

	return private, nil
}
