// Package prometheus deploys the cluster's metrics-collection plane:
// prometheus-operator, the Prometheus custom resource it reconciles, and
// the exporters that feed it (node-exporter, kube-state-metrics, and a
// ServiceMonitor for the kubelet's built-in cAdvisor) - all hand-rolled Go
// resources (not a Helm chart), following pkg/components/longhorn's
// conventions: the namespace is created centrally by
// pkg/deploy/namespaces.go and passed in, not created here.
//
// This is ported from the old TypeScript stack under
// _migrateme/components/kubernetes/{prometheus-operator,prometheus,
// node-exporter,kube-state-metrics,cadvisor}, with one deliberate
// improvement and two deliberate gaps carried forward as-is:
//
//   - Data retention strategy (the improvement): legacy set only a
//     time-based `retention: 30d` with no size cap at all - a real gap,
//     since time-only retention can let a cardinality spike fill the PVC to
//     100% between GC cycles and crash-loop Prometheus. See instance.go's
//     doc comment for the dual time+size retention this version sets
//     instead.
//   - No PrometheusRule exists anywhere in this stack (no alerting rules),
//     matching legacy, which never created one either.
//   - No self-scrape ServiceMonitor for Prometheus's own /metrics exists
//     either, also matching legacy.
//
// Grafana (pkg/components/grafana) is a separate package/component, wired
// to this one's Service by hostname alone (prometheus-<name>.<namespace>) -
// it has a materially different concern (a Tailscale-exposed, user-facing
// UI) than this "backend metrics plumbing" package.
//
// Both Prometheus's own UI and Alertmanager's UI ARE Tailscale-exposed
// (unlike the initial cut of this package, which deferred Prometheus's own
// exposure) - "prom" and "alerts" hostnames, same waypoint+ingress pattern
// as Grafana and Longhorn's UI. Alertmanager itself is deliberately bare -
// see alertmanager.go's doc comment for why it has no notification
// receivers wired.
package prometheus

import (
	"github.com/liamawhite/homelab/pkg/components/istio/waypoint"
	"github.com/liamawhite/homelab/pkg/components/tailscale"
	"github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Prometheus represents the deployed metrics-collection plane.
type Prometheus struct {
	pulumi.ResourceState

	Namespace pulumi.StringOutput

	redirects []ingress.RedirectRoute
}

// TailscaleRedirects returns the Cloudflare-redirect data for both
// Prometheus's own UI and Alertmanager's UI - see
// applications.Private.TailscaleRedirect for why these can't be applied
// independently and instead have to be collected centrally
// (pkg/deploy/redirects.go).
func (p *Prometheus) TailscaleRedirects() []ingress.RedirectRoute {
	return p.redirects
}

// PrometheusArgs contains the configuration for Prometheus.
type PrometheusArgs struct {
	// Namespace is created centrally by pkg/deploy/namespaces.go
	// (MonitoringNamespace) and passed in here - this component does not
	// create it.
	Namespace pulumi.StringInput

	// OperatorVersion, Version (Prometheus itself), NodeExporterVersion,
	// KubeStateMetricsVersion, AlertmanagerVersion, and KubeRBACProxyVersion
	// are the pkg/versions/versions.go constants for each image this
	// package deploys.
	OperatorVersion         string
	Version                 string
	NodeExporterVersion     string
	KubeStateMetricsVersion string
	AlertmanagerVersion     string
	KubeRBACProxyVersion    string

	// StorageClassName is the cluster's default StorageClass (Longhorn's
	// DefaultStorageClass output) - Prometheus's PVC is provisioned against
	// it.
	StorageClassName pulumi.StringInput
	// StorageSize is the Prometheus PVC's requested size, e.g. "20Gi".
	StorageSize string
	// Retention is Prometheus's time-based retention ceiling, e.g. "14d".
	Retention string
	// RetentionSize is Prometheus's size-based retention ceiling, e.g.
	// "18GB" - see instance.go's doc comment for why both are set together.
	RetentionSize string

	// TailscaleOperatorNamespace is where pkg/components/tailscale's
	// operator (and its dynamically created per-Ingress proxy pods) run -
	// passed through to ingress.NewIngress so it can restrict its
	// AuthorizationPolicy bypass to that namespace's identities.
	TailscaleOperatorNamespace pulumi.StringInput
	// TailscaleMagicDNSSuffix is infraCfg.Tailscale.MagicDNSSuffix - your
	// tailnet's real MagicDNS suffix, used to build the redirect target
	// URLs.
	TailscaleMagicDNSSuffix pulumi.StringInput

	// CloudflareZoneID is the Cloudflare zone the redirect DNS records
	// belong to - precomputed once in pkg/deploy and shared across every
	// caller (see pkg/deploy/zone.go).
	CloudflareZoneID pulumi.StringInput
	// CloudflareBaseDomain is infraCfg.Cloudflare.Tunnel.Domain.
	CloudflareBaseDomain pulumi.StringInput
	// CloudflareProvider is the Cloudflare provider to create the redirect
	// DNS records with.
	CloudflareProvider *cloudflare.Provider
}

// NewPrometheus deploys prometheus-operator, the Prometheus CR, this
// stack's exporters, a bare Alertmanager, and exposes both Prometheus's and
// Alertmanager's UIs over Tailscale.
func NewPrometheus(ctx *pulumi.Context, name string, args *PrometheusArgs, opts ...pulumi.ResourceOption) (*Prometheus, error) {
	p := &Prometheus{}

	err := ctx.RegisterComponentResource("homelab:kubernetes:prometheus", name, p, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(p))

	p.Namespace = args.Namespace.ToStringOutput()

	// Network policy - see network.go. Applied first so no workload below
	// ever comes up even briefly without it.
	if err := newNetworkPolicy(ctx, name+"-network", args.Namespace, localOpts...); err != nil {
		return nil, err
	}

	operator, err := newOperator(ctx, name, args.Namespace, args.OperatorVersion, args.KubeRBACProxyVersion, localOpts...)
	if err != nil {
		return nil, err
	}

	// Dedicated waypoints for Prometheus's and Alertmanager's own Services -
	// each Service gets its own waypoint in this repo's convention, not a
	// shared one (see pkg/components/istio/waypoint's package doc). Created
	// before their Services so each Service can set its
	// istio.io/use-waypoint label directly at creation (we own both
	// Services, unlike Longhorn's Helm-owned one).
	promWaypoint, err := waypoint.NewWaypoint(ctx, name+"-waypoint", &waypoint.WaypointArgs{
		Namespace: args.Namespace,
		Labels: pulumi.StringMap{
			tailscale.WaypointAccessLabelKey: pulumi.String(tailscale.WaypointAccessLabelValue),
		},
		TargetLabels: pulumi.StringMap{"app.kubernetes.io/name": pulumi.String(PrometheusPodLabel)},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	alertmanagerWaypoint, err := waypoint.NewWaypoint(ctx, name+"-alertmanager-waypoint", &waypoint.WaypointArgs{
		Namespace: args.Namespace,
		Labels: pulumi.StringMap{
			tailscale.WaypointAccessLabelKey: pulumi.String(tailscale.WaypointAccessLabelValue),
		},
		TargetLabels: pulumi.StringMap{"app.kubernetes.io/name": pulumi.String(AlertmanagerPodLabel)},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// The Prometheus CR itself waits on the operator's own Deployment - its
	// controller has to be running to reconcile the CR into a StatefulSet.
	instOpts := append(localOpts, pulumi.DependsOn([]pulumi.Resource{operator}))
	_, promService, err := newInstance(ctx, name, instanceArgs{
		Namespace:        args.Namespace,
		Version:          args.Version,
		StorageClassName: args.StorageClassName,
		StorageSize:      args.StorageSize,
		Retention:        args.Retention,
		RetentionSize:    args.RetentionSize,
		WaypointName:     promWaypoint.Name,
	}, instOpts...)
	if err != nil {
		return nil, err
	}

	_, alertmanagerService, err := newAlertmanager(ctx, name+"-alertmanager", args.Namespace, args.AlertmanagerVersion, alertmanagerWaypoint.Name, instOpts...)
	if err != nil {
		return nil, err
	}

	// node-exporter/kube-state-metrics/cadvisor's ServiceMonitor all depend
	// only on the operator (its CRD-reconciling controller has to exist to
	// accept a ServiceMonitor at all) - not on the Prometheus CR itself,
	// mirroring legacy's own stated dependency ordering.
	if err := newNodeExporter(ctx, name+"-node-exporter", args.Namespace, args.NodeExporterVersion, args.KubeRBACProxyVersion, instOpts...); err != nil {
		return nil, err
	}

	if err := newKubeStateMetrics(ctx, name+"-kube-state-metrics", args.Namespace, args.KubeStateMetricsVersion, instOpts...); err != nil {
		return nil, err
	}

	if err := newCadvisorServiceMonitor(ctx, name+"-cadvisor", instOpts...); err != nil {
		return nil, err
	}

	// Put both UIs on Tailscale - see pkg/components/longhorn's identical
	// step for the full reasoning.
	promIngress, err := ingress.NewIngress(ctx, name+"-ui", &ingress.IngressArgs{
		Namespace:            args.Namespace,
		ServiceName:          pulumi.String(ServiceName),
		ServicePort:          9090,
		Hostname:             "prom",
		OperatorNamespace:    args.TailscaleOperatorNamespace,
		MagicDNSSuffix:       args.TailscaleMagicDNSSuffix,
		CloudflareZoneID:     args.CloudflareZoneID,
		CloudflareBaseDomain: args.CloudflareBaseDomain,
		CloudflareProvider:   args.CloudflareProvider,
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{promService, promWaypoint}))...)
	if err != nil {
		return nil, err
	}

	alertmanagerIngress, err := ingress.NewIngress(ctx, name+"-alertmanager-ui", &ingress.IngressArgs{
		Namespace:            args.Namespace,
		ServiceName:          pulumi.String(AlertmanagerServiceName),
		ServicePort:          9093,
		Hostname:             "alerts",
		OperatorNamespace:    args.TailscaleOperatorNamespace,
		MagicDNSSuffix:       args.TailscaleMagicDNSSuffix,
		CloudflareZoneID:     args.CloudflareZoneID,
		CloudflareBaseDomain: args.CloudflareBaseDomain,
		CloudflareProvider:   args.CloudflareProvider,
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{alertmanagerService, alertmanagerWaypoint}))...)
	if err != nil {
		return nil, err
	}

	p.redirects = []ingress.RedirectRoute{promIngress.Redirect, alertmanagerIngress.Redirect}

	if err := ctx.RegisterResourceOutputs(p, pulumi.Map{
		"namespace": p.Namespace,
	}); err != nil {
		return nil, err
	}

	return p, nil
}
