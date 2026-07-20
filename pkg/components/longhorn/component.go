// Package longhorn provides a Pulumi component for deploying Longhorn distributed storage.
//
// Longhorn is a lightweight, reliable, and distributed block storage system for Kubernetes.
// This component deploys Longhorn via Helm chart with Istio ambient mesh integration.
//
// The component:
//   - Deploys Longhorn via Helm (all defaults) into a caller-supplied namespace
//   - Exports the default StorageClass for use by other components
//   - Exposes the Longhorn UI over Tailscale, mirroring
//     pkg/deploy/applications/private.go's exposure pattern
//
// The namespace itself (including the Istio ambient-mode label) is created
// centrally by pkg/deploy/namespaces.go, not by this component.
//
// Example usage:
//
//	storage, err := longhorn.NewLonghorn(ctx, "longhorn", &longhorn.LonghornArgs{
//	    Version:                    versions.Longhorn,
//	    Namespace:                  namespaces.Get(deploy.LonghornSystemNamespace).Metadata.Name(),
//	    TailscaleOperatorNamespace: namespaces.Get(deploy.TailscaleNamespace).Metadata.Name(),
//	    TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
//	    CloudflareZoneID:           zoneID,
//	    CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
//	    CloudflareProvider:         providers.Cloudflare,
//	}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{istioMesh, tsOperator}))
package longhorn

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/istio/waypoint"
	"github.com/liamawhite/homelab/pkg/components/tailscale"
	"github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	storagev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/storage/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	helmRepository   = "https://charts.longhorn.io"
	storageClassName = "longhorn"

	// uiServiceName/uiServicePort/uiPodLabel describe the Service and pods
	// the Longhorn Helm chart itself creates for its UI (confirmed via
	// `helm template longhorn/longhorn`) - this component doesn't own or
	// create them, only references them by name to route them through a
	// waypoint and onto Tailscale.
	uiServiceName = "longhorn-frontend"
	uiServicePort = 80
	uiPodLabel    = "longhorn-ui"

	// uiHostname is the Tailscale-side hostname prefix for the Longhorn UI,
	// matching the legacy TypeScript component's "storage" naming.
	uiHostname = "storage"
)

// Longhorn represents the Longhorn storage system component
type Longhorn struct {
	pulumi.ResourceState

	Namespace           pulumi.StringOutput
	DefaultStorageClass pulumi.StringOutput
	UIHostname          pulumi.StringOutput

	redirect ingress.RedirectRoute
}

// TailscaleRedirect returns the Longhorn UI's Cloudflare-redirect data - see
// applications.Private.TailscaleRedirect for why this can't be applied
// independently and instead has to be collected centrally (pkg/deploy/redirects.go).
func (l *Longhorn) TailscaleRedirect() ingress.RedirectRoute {
	return l.redirect
}

// LonghornArgs contains the configuration for Longhorn
type LonghornArgs struct {
	// Version is the Longhorn Helm chart version to deploy
	Version string
	// Namespace is longhorn-system's name, created centrally by
	// pkg/deploy/namespaces.go (with the Istio ambient-mode label) and
	// passed in here - this component does not create it.
	Namespace pulumi.StringInput

	// TailscaleOperatorNamespace is where pkg/components/tailscale's
	// operator (and its dynamically created per-Ingress proxy pods) run -
	// passed through to ingress.NewIngress so it can restrict its
	// AuthorizationPolicy bypass to that namespace's identities.
	TailscaleOperatorNamespace pulumi.StringInput
	// TailscaleMagicDNSSuffix is infraCfg.Tailscale.MagicDNSSuffix - your
	// tailnet's real MagicDNS suffix, used to build the UI's redirect target
	// URL.
	TailscaleMagicDNSSuffix pulumi.StringInput

	// CloudflareZoneID is the Cloudflare zone the UI's redirect DNS record
	// belongs to - precomputed once in pkg/deploy and shared across every
	// caller (see pkg/deploy/zone.go).
	CloudflareZoneID pulumi.StringInput
	// CloudflareBaseDomain is infraCfg.Cloudflare.Tunnel.Domain.
	CloudflareBaseDomain pulumi.StringInput
	// CloudflareProvider is the Cloudflare provider to create the redirect
	// DNS record with.
	CloudflareProvider *cloudflare.Provider
}

// NewLonghorn creates a new Longhorn component with distributed storage
func NewLonghorn(
	ctx *pulumi.Context,
	name string,
	args *LonghornArgs,
	opts ...pulumi.ResourceOption,
) (*Longhorn, error) {
	// Validate required args
	if args.Version == "" {
		return nil, fmt.Errorf("longhorn version is required")
	}

	longhorn := &Longhorn{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:longhorn", name, longhorn, opts...)
	if err != nil {
		return nil, err
	}

	// All child resources should be parented to this component
	localOpts := append(opts, pulumi.Parent(longhorn))

	longhorn.Namespace = args.Namespace.ToStringOutput()

	// Network policy - see network.go. Applied before the chart so
	// Longhorn's pods never even briefly come up without it.
	if err := newNetworkPolicy(ctx, fmt.Sprintf("%s-network", name), args.Namespace, localOpts...); err != nil {
		return nil, err
	}

	// 1. Deploy Longhorn Helm chart with default values
	chart, err := helmv4.NewChart(ctx, fmt.Sprintf("%s-chart", name), &helmv4.ChartArgs{
		Namespace: args.Namespace,
		Chart:     pulumi.String("longhorn"),
		Version:   pulumi.String(args.Version),
		RepositoryOpts: &helmv4.RepositoryOptsArgs{
			Repo: pulumi.String(helmRepository),
		},
		Values: pulumi.Map{}, // No custom values - all defaults
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 2. Retrieve the default StorageClass created by Helm
	storageClass, err := storagev1.GetStorageClass(ctx,
		fmt.Sprintf("%s-default-sc", name),
		pulumi.ID(storageClassName),
		&storagev1.StorageClassState{},
		append(localOpts, pulumi.DependsOn([]pulumi.Resource{chart}))...,
	)
	if err != nil {
		return nil, err
	}
	longhorn.DefaultStorageClass = storageClass.Metadata.Name().Elem()

	// 3. Dedicated waypoint for the UI's Service, opting into
	// pkg/components/tailscale's waypoint-access policy since the UI is
	// reachable through Tailscale, same as applications.Private. TargetLabels
	// matches the frontend pods' actual "app: longhorn-ui" label so the
	// waypoint component wires up the matching Cilium egress/ingress CCNP
	// pair automatically.
	wp, err := waypoint.NewWaypoint(ctx, fmt.Sprintf("%s-ui-waypoint", name), &waypoint.WaypointArgs{
		Namespace: args.Namespace,
		Labels: pulumi.StringMap{
			tailscale.WaypointAccessLabelKey: pulumi.String(tailscale.WaypointAccessLabelValue),
		},
		TargetLabels: pulumi.StringMap{"app": pulumi.String(uiPodLabel)},
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{chart}))...)
	if err != nil {
		return nil, err
	}

	// 4. Route the UI's Service through that waypoint. The Helm chart owns
	// this Service (it's not created by this component like every other
	// app's Service in this repo), so a server-side-apply patch adds just
	// the istio.io/use-waypoint label rather than this component taking over
	// the whole object.
	svcPatch, err := corev1.NewServicePatch(ctx, fmt.Sprintf("%s-ui-service-patch", name), &corev1.ServicePatchArgs{
		Metadata: &metav1.ObjectMetaPatchArgs{
			Name:      pulumi.String(uiServiceName),
			Namespace: args.Namespace,
			Labels: pulumi.StringMap{
				"istio.io/use-waypoint": wp.Name,
			},
		},
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{chart, wp}))...)
	if err != nil {
		return nil, err
	}

	// 5. Put the UI's Service on Tailscale: the k8s Ingress the operator
	// reconciles into a tailnet-joined proxy pod, the AuthorizationPolicy
	// bypass that traffic needs to reach the Service through its waypoint,
	// and the Cloudflare-side redirect bookkeeping - see
	// applications.Private for the app-shaped equivalent of this same
	// pattern.
	tsIngress, err := ingress.NewIngress(ctx, fmt.Sprintf("%s-ui", name), &ingress.IngressArgs{
		Namespace:            args.Namespace,
		ServiceName:          pulumi.String(uiServiceName),
		ServicePort:          uiServicePort,
		Hostname:             uiHostname,
		OperatorNamespace:    args.TailscaleOperatorNamespace,
		MagicDNSSuffix:       args.TailscaleMagicDNSSuffix,
		CloudflareZoneID:     args.CloudflareZoneID,
		CloudflareBaseDomain: args.CloudflareBaseDomain,
		CloudflareProvider:   args.CloudflareProvider,
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{svcPatch, wp}))...)
	if err != nil {
		return nil, err
	}
	longhorn.UIHostname = tsIngress.Hostname
	longhorn.redirect = tsIngress.Redirect

	// 6. Register component outputs
	if err := ctx.RegisterResourceOutputs(longhorn, pulumi.Map{
		"namespace":           longhorn.Namespace,
		"defaultStorageClass": longhorn.DefaultStorageClass,
		"uiHostname":          longhorn.UIHostname,
	}); err != nil {
		return nil, err
	}

	return longhorn, nil
}
