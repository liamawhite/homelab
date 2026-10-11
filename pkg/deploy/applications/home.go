package applications

import (
	"fmt"

	waypoint "github.com/liamawhite/homelab/pkg/components/istio/waypoint"
	"github.com/liamawhite/homelab/pkg/components/tailscale"
	"github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	"github.com/liamawhite/homelab/pkg/versions"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	homePort = 8080
)

// Home represents the home app: a single Go binary (embedded React
// frontend, no API, no storage - just cards linking out to every other app
// in this repo) reachable only over Tailscale - same exposure pattern as
// Private/Workouts, minus the PVC neither this app needs.
type Home struct {
	pulumi.ResourceState

	Namespace   pulumi.StringOutput
	ServiceName pulumi.StringOutput

	redirect ingress.RedirectRoute
}

// TailscaleRedirect returns this app's Cloudflare-redirect data - see
// Private.TailscaleRedirect's doc comment for why this only hands back data
// rather than applying anything itself.
func (h *Home) TailscaleRedirect() ingress.RedirectRoute {
	return h.redirect
}

// HomeArgs contains the configuration for Home.
type HomeArgs struct {
	// Namespace is created centrally by pkg/deploy/namespaces.go
	// (HomeNamespace) and passed in here - this component does not create
	// it.
	Namespace pulumi.StringInput

	// TailscaleOperatorNamespace/TailscaleMagicDNSSuffix/CloudflareZoneID/
	// CloudflareBaseDomain/CloudflareProvider are threaded straight through
	// to ingress.NewIngress - see PrivateArgs' doc comments for each.
	TailscaleOperatorNamespace pulumi.StringInput
	TailscaleMagicDNSSuffix    pulumi.StringInput
	CloudflareZoneID           pulumi.StringInput
	CloudflareBaseDomain       pulumi.StringInput
	CloudflareProvider         *cloudflare.Provider

	// Retained for call-site compatibility; publishing runs in GitHub Actions.
	GHCRUsername string
	GHCRToken    string
}

// NewHome deploys Dashboard using the existing Home resources.
func NewHome(ctx *pulumi.Context, name string, args *HomeArgs, opts ...pulumi.ResourceOption) (*Home, error) {
	home := &Home{}

	err := ctx.RegisterComponentResource("homelab:applications:home", name, home, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(home))

	labels := pulumi.StringMap{"app": pulumi.String(name)}

	// 1. Dedicated ServiceAccount for this app.
	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 2. Deploy the backend.
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
							Name:  pulumi.String(name),
							Image: pulumi.String(versions.DashboardImage),
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{ContainerPort: pulumi.Int(homePort)},
							},
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/"),
									Port: pulumi.Int(homePort),
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

	// 3. Dedicated waypoint for this app's Service, opted into Tailscale's
	// access policy - same as Private/Workouts.
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

	// 4. Expose it via a Service, routed through the waypoint above.
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
					TargetPort: pulumi.Int(homePort),
				},
			},
		},
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{deployment}))...)
	if err != nil {
		return nil, err
	}

	// 5. Put this Service on Tailscale - same as Private/Workouts.
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

	home.Namespace = args.Namespace.ToStringOutput()
	home.ServiceName = service.Metadata.Name().Elem()
	home.redirect = tsIngress.Redirect

	if err := ctx.RegisterResourceOutputs(home, pulumi.Map{
		"namespace":   home.Namespace,
		"serviceName": home.ServiceName,
	}); err != nil {
		return nil, err
	}

	return home, nil
}
