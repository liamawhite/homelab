// Package grafana deploys Grafana, provisioned entirely as code (a
// Prometheus datasource and two dashboards - see dashboards.go) and exposed
// over Tailscale, following pkg/components/longhorn's UI-exposure
// conventions: a dedicated waypoint, a Service labeled onto it, and
// pkg/components/tailscale/ingress.NewIngress.
//
// Two deliberate choices, both explicit user decisions rather than defaults
// carried over unexamined from the legacy TypeScript stack:
//
//   - Anonymous-admin auth (matches legacy): no login screen, anonymous
//     users get the Admin role. Acceptable because the actual access
//     control is Tailscale-only reachability, the same trust model as
//     Longhorn's UI.
//   - No PVC (unlike legacy, which had one): Grafana's SQLite DB lives in
//     an EmptyDir. Dashboards/datasources are fully provisioned as code via
//     the ConfigMaps below, so a pod restart only loses ad-hoc UI edits,
//     annotations, and alert-silence state - an acceptable homelab
//     tradeoff for one fewer Longhorn volume to manage.
//
// Prometheus's and Alertmanager's own UIs are also Tailscale-exposed
// ("prom"/"alerts" hostnames), directly by pkg/components/prometheus using
// this exact same waypoint+ingress pattern - see that package's component.go.
package grafana

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/dns"
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
	image        = "grafana/grafana"
	uiHostname   = "grafana"
	uiPodLabel   = "grafana"
	servicePort  = 3000
	configMapKey = "grafana-config"
)

const grafanaINI = `[server]
root_url = https://%[1]s

[auth.anonymous]
enabled = true
org_role = Admin

[security]
admin_user = admin
disable_initial_admin_creation = true

[users]
allow_sign_up = false
`

const dashboardsYML = `apiVersion: 1
providers:
  - name: default
    orgId: 1
    folder: ""
    type: file
    disableDeletion: false
    allowUiUpdates: true
    updateIntervalSeconds: 10
    options:
      path: /var/lib/grafana/dashboards
`

// Grafana represents the deployed Grafana UI.
type Grafana struct {
	pulumi.ResourceState

	Namespace pulumi.StringOutput
	Hostname  pulumi.StringOutput

	redirect ingress.RedirectRoute
}

// TailscaleRedirect returns Grafana's Cloudflare-redirect data - see
// applications.Private.TailscaleRedirect for why this can't be applied
// independently and instead has to be collected centrally
// (pkg/deploy/redirects.go).
func (g *Grafana) TailscaleRedirect() ingress.RedirectRoute {
	return g.redirect
}

// GrafanaArgs contains the configuration for Grafana.
type GrafanaArgs struct {
	// Version is the Grafana image tag to deploy (no "v" prefix, unlike
	// most other versions.go constants - matches upstream's own tagging).
	Version string
	// Namespace is "monitoring", created centrally by
	// pkg/deploy/namespaces.go and passed in here - this component does not
	// create it.
	Namespace pulumi.StringInput

	// PrometheusServiceName is the Prometheus Service's name
	// (prometheus.NewPrometheus's caller knows this - "prometheus-k8s") -
	// the datasource ConfigMap points at
	// http://<PrometheusServiceName>.<Namespace>:9090.
	PrometheusServiceName pulumi.StringInput

	// TailscaleOperatorNamespace is where pkg/components/tailscale's
	// operator (and its dynamically created per-Ingress proxy pods) run -
	// passed through to ingress.NewIngress so it can restrict its
	// AuthorizationPolicy bypass to that namespace's identities.
	TailscaleOperatorNamespace pulumi.StringInput
	// TailscaleMagicDNSSuffix is infraCfg.Tailscale.MagicDNSSuffix - your
	// tailnet's real MagicDNS suffix, used to build the UI's redirect
	// target URL.
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

// NewGrafana deploys Grafana and its Tailscale ingress.
func NewGrafana(ctx *pulumi.Context, name string, args *GrafanaArgs, opts ...pulumi.ResourceOption) (*Grafana, error) {
	if args.Version == "" {
		return nil, fmt.Errorf("grafana version is required")
	}

	g := &Grafana{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:grafana", name, g, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(g))

	g.Namespace = args.Namespace.ToStringOutput()

	// Network policy - see network.go. Applied before the Deployment so
	// Grafana's pod never even briefly comes up without it.
	if err := newNetworkPolicy(ctx, fmt.Sprintf("%s-network", name), args.Namespace, localOpts...); err != nil {
		return nil, err
	}

	labels := pulumi.StringMap{"app": pulumi.String(uiPodLabel)}

	datasourcesYML := pulumi.Sprintf(`apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://%s.%s:9090
    isDefault: true
    jsonData:
      timeInterval: 30s
`, args.PrometheusServiceName, args.Namespace)

	configMap, err := corev1.NewConfigMap(ctx, fmt.Sprintf("%s-config", name), &corev1.ConfigMapArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(configMapKey),
			Namespace: args.Namespace,
		},
		Data: pulumi.StringMap{
			"grafana.ini":     pulumi.Sprintf(grafanaINI, pulumi.Sprintf("%s.%s", pulumi.String(uiHostname), args.TailscaleMagicDNSSuffix)),
			"datasources.yml": datasourcesYML,
			"dashboards.yml":  pulumi.String(dashboardsYML),
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	dashboardsConfigMap, err := corev1.NewConfigMap(ctx, fmt.Sprintf("%s-dashboards", name), &corev1.ConfigMapArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("grafana-dashboards"),
			Namespace: args.Namespace,
		},
		Data: pulumi.StringMap{
			"node-metrics.json": pulumi.String(nodeMetricsDashboard),
			"pod-metrics.json":  pulumi.String(podMetricsDashboard),
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	deployment, err := appsv1.NewDeployment(ctx, fmt.Sprintf("%s-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("grafana"),
			Namespace: args.Namespace,
			Labels:    labels,
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Selector: &metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: pulumi.StringMap{
						"app":              pulumi.String(uiPodLabel),
						dns.AccessLabelKey: pulumi.String(dns.AccessLabelValue),
					},
				},
				Spec: &corev1.PodSpecArgs{
					SecurityContext: &corev1.PodSecurityContextArgs{
						FsGroup:      pulumi.Int(472),
						RunAsUser:    pulumi.Int(472),
						RunAsGroup:   pulumi.Int(472),
						RunAsNonRoot: pulumi.Bool(true),
					},
					Volumes: corev1.VolumeArray{
						&corev1.VolumeArgs{
							Name:      pulumi.String("config"),
							ConfigMap: &corev1.ConfigMapVolumeSourceArgs{Name: configMap.Metadata.Name().Elem()},
						},
						&corev1.VolumeArgs{
							Name:      pulumi.String("dashboards"),
							ConfigMap: &corev1.ConfigMapVolumeSourceArgs{Name: dashboardsConfigMap.Metadata.Name().Elem()},
						},
						&corev1.VolumeArgs{
							Name:     pulumi.String("data"),
							EmptyDir: &corev1.EmptyDirVolumeSourceArgs{},
						},
					},
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("grafana"),
							Image: pulumi.Sprintf("%s:%s", image, args.Version),
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{Name: pulumi.String("web"), ContainerPort: pulumi.Int(servicePort)},
							},
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{Name: pulumi.String("GF_SECURITY_ADMIN_USER"), Value: pulumi.String("admin")},
							},
							VolumeMounts: corev1.VolumeMountArray{
								&corev1.VolumeMountArgs{Name: pulumi.String("config"), SubPath: pulumi.String("grafana.ini"), MountPath: pulumi.String("/etc/grafana/grafana.ini")},
								&corev1.VolumeMountArgs{Name: pulumi.String("config"), SubPath: pulumi.String("datasources.yml"), MountPath: pulumi.String("/etc/grafana/provisioning/datasources/datasources.yml")},
								&corev1.VolumeMountArgs{Name: pulumi.String("config"), SubPath: pulumi.String("dashboards.yml"), MountPath: pulumi.String("/etc/grafana/provisioning/dashboards/dashboards.yml")},
								&corev1.VolumeMountArgs{Name: pulumi.String("dashboards"), MountPath: pulumi.String("/var/lib/grafana/dashboards")},
								&corev1.VolumeMountArgs{Name: pulumi.String("data"), MountPath: pulumi.String("/var/lib/grafana")},
							},
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet:             &corev1.HTTPGetActionArgs{Path: pulumi.String("/api/health"), Port: pulumi.String("web")},
								InitialDelaySeconds: pulumi.Int(10),
								PeriodSeconds:       pulumi.Int(10),
							},
							ReadinessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{Path: pulumi.String("/api/health"), Port: pulumi.String("web")},
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{"cpu": pulumi.String("100m"), "memory": pulumi.String("128Mi")},
								Limits:   pulumi.StringMap{"cpu": pulumi.String("500m"), "memory": pulumi.String("512Mi")},
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

	// Dedicated waypoint for Grafana's Service, opting into
	// pkg/components/tailscale's waypoint-access policy since the UI is
	// reachable through Tailscale, same as Longhorn's UI.
	wp, err := waypoint.NewWaypoint(ctx, fmt.Sprintf("%s-waypoint", name), &waypoint.WaypointArgs{
		Namespace: args.Namespace,
		Labels: pulumi.StringMap{
			tailscale.WaypointAccessLabelKey: pulumi.String(tailscale.WaypointAccessLabelValue),
		},
		TargetLabels: pulumi.StringMap{"app": pulumi.String(uiPodLabel)},
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{deployment}))...)
	if err != nil {
		return nil, err
	}

	// Grafana's Service - we own this (unlike Longhorn's Helm-owned one), so
	// the istio.io/use-waypoint label is set directly at creation, no
	// ServicePatch needed (same pattern as applications.Private).
	service, err := corev1.NewService(ctx, fmt.Sprintf("%s-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("grafana"),
			Namespace: args.Namespace,
			Labels: pulumi.StringMap{
				"istio.io/use-waypoint": wp.Name,
			},
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: labels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{Name: pulumi.String("web"), Port: pulumi.Int(servicePort), TargetPort: pulumi.String("web")},
			},
		},
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{deployment}))...)
	if err != nil {
		return nil, err
	}

	// Put Grafana's Service on Tailscale - see pkg/components/longhorn's
	// identical step for the full reasoning.
	tsIngress, err := ingress.NewIngress(ctx, fmt.Sprintf("%s-ui", name), &ingress.IngressArgs{
		Namespace:            args.Namespace,
		ServiceName:          service.Metadata.Name().Elem(),
		ServicePort:          servicePort,
		Hostname:             uiHostname,
		OperatorNamespace:    args.TailscaleOperatorNamespace,
		MagicDNSSuffix:       args.TailscaleMagicDNSSuffix,
		CloudflareZoneID:     args.CloudflareZoneID,
		CloudflareBaseDomain: args.CloudflareBaseDomain,
		CloudflareProvider:   args.CloudflareProvider,
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{service, wp}))...)
	if err != nil {
		return nil, err
	}
	g.Hostname = tsIngress.Hostname
	g.redirect = tsIngress.Redirect

	if err := ctx.RegisterResourceOutputs(g, pulumi.Map{
		"namespace": g.Namespace,
		"hostname":  g.Hostname,
	}); err != nil {
		return nil, err
	}

	return g, nil
}
