package lumenetescontroller

import (
	"fmt"

	waypoint "github.com/liamawhite/homelab/pkg/components/istio/waypoint"
	"github.com/liamawhite/homelab/pkg/components/tailscale"
	"github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// uiPort is where lumenetes-controller's embedded read-only web UI (see
// cmd/lumenetes-controller/main.go's manager.Server Runnable) listens.
const uiPort = 8082

// uiHostname is the Tailscale/Cloudflare-redirect subdomain the UI is
// reachable at: lights.<tailnet>.
const uiHostname = "lights"

// uiExposureArgs is what newUIExposure needs to put lumenetes-controller's
// embedded web UI on Tailscale - same shape as every other
// waypoint+Service+ingress.NewIngress exposure in this repo (see
// pkg/deploy/applications/workouts.go).
type uiExposureArgs struct {
	Namespace                  pulumi.StringInput
	TailscaleOperatorNamespace pulumi.StringInput
	TailscaleMagicDNSSuffix    pulumi.StringInput
	CloudflareZoneID           pulumi.StringInput
	CloudflareBaseDomain       pulumi.StringInput
	CloudflareProvider         *cloudflare.Provider
}

// newUIExposure puts lumenetes-controller's embedded web UI on Tailscale at
// uiHostname. Unlike every other app this repo exposes this way, there's no
// dedicated Deployment/ServiceAccount here - this Service simply selects the
// existing lumenetes-controller pods (see component.go's Deployment) on
// their extra "ui" container port, since the UI is served by that same
// binary/container rather than a separate one.
func newUIExposure(ctx *pulumi.Context, name string, args *uiExposureArgs, opts ...pulumi.ResourceOption) (ingress.RedirectRoute, error) {
	targetLabels := pulumi.StringMap{"app": pulumi.String("lumenetes-controller")}

	wp, err := waypoint.NewWaypoint(ctx, fmt.Sprintf("%s-ui-waypoint", name), &waypoint.WaypointArgs{
		Namespace: args.Namespace,
		Labels: pulumi.StringMap{
			tailscale.WaypointAccessLabelKey: pulumi.String(tailscale.WaypointAccessLabelValue),
		},
		TargetLabels: targetLabels,
	}, opts...)
	if err != nil {
		return ingress.RedirectRoute{}, err
	}

	service, err := corev1.NewService(ctx, fmt.Sprintf("%s-ui-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fmt.Sprintf("%s-ui", name)),
			Namespace: args.Namespace,
			Labels: pulumi.StringMap{
				"istio.io/use-waypoint": wp.Name,
			},
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: targetLabels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{
					Port:       pulumi.Int(80),
					TargetPort: pulumi.Int(uiPort),
				},
			},
		},
	}, opts...)
	if err != nil {
		return ingress.RedirectRoute{}, err
	}

	tsIngress, err := ingress.NewIngress(ctx, fmt.Sprintf("%s-ui", name), &ingress.IngressArgs{
		Namespace:            args.Namespace,
		ServiceName:          service.Metadata.Name().Elem(),
		ServicePort:          80,
		Hostname:             uiHostname,
		OperatorNamespace:    args.TailscaleOperatorNamespace,
		MagicDNSSuffix:       args.TailscaleMagicDNSSuffix,
		CloudflareZoneID:     args.CloudflareZoneID,
		CloudflareBaseDomain: args.CloudflareBaseDomain,
		CloudflareProvider:   args.CloudflareProvider,
	}, append(append([]pulumi.ResourceOption{}, opts...), pulumi.DependsOn([]pulumi.Resource{service, wp}))...)
	if err != nil {
		return ingress.RedirectRoute{}, err
	}

	return tsIngress.Redirect, nil
}
