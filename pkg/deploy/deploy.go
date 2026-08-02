// Package deploy holds the Pulumi program that deploys the homelab's
// Pulumi-managed infrastructure. It's a plain library (not `package main`)
// so the CLI can run it fully inline via the Pulumi Automation API,
// supplying a kubeconfig it has resolved itself instead of the program
// reading one from a fixed path, with no on-disk Pulumi project required.
package deploy

import (
	"github.com/liamawhite/homelab/pkg/components/apiserver"
	"github.com/liamawhite/homelab/pkg/components/cilium"
	accessjwt "github.com/liamawhite/homelab/pkg/components/cloudflare/accessjwt"
	cfauth "github.com/liamawhite/homelab/pkg/components/cloudflare/auth"
	cftunnel "github.com/liamawhite/homelab/pkg/components/cloudflare/tunnel"
	"github.com/liamawhite/homelab/pkg/components/dns"
	"github.com/liamawhite/homelab/pkg/components/grafana"
	"github.com/liamawhite/homelab/pkg/components/istio"
	"github.com/liamawhite/homelab/pkg/components/kubevip"
	"github.com/liamawhite/homelab/pkg/components/longhorn"
	"github.com/liamawhite/homelab/pkg/components/prometheus"
	"github.com/liamawhite/homelab/pkg/components/tailscale"
	tsacl "github.com/liamawhite/homelab/pkg/components/tailscale/acl"
	tsingress "github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	infraconfig "github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/deploy/applications"
	"github.com/liamawhite/homelab/pkg/versions"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Program returns the Pulumi program that deploys kube-vip, the Istio
// control plane, and every app - each fronted by its own ambient waypoint
// rather than a shared ingress Gateway - against the cluster reachable via
// kubeconfig. The total time this program is allowed to run is bounded by
// the caller's context deadline (see cli/cmd/pulumi's "--timeout" flag),
// not by anything set here - see that flag's doc comment for why a
// per-resource timeout isn't the right tool for that.
func Program(kubeconfig string, infraCfg *infraconfig.InfraConfig) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		providers, err := NewProviders(ctx, kubeconfig, infraCfg)
		if err != nil {
			return err
		}

		// Cilium establishes the base pod network - everything below
		// depends on it, so it has to be the first real workload created.
		ciliumComp, err := cilium.NewCilium(ctx, "cilium", &cilium.CiliumArgs{
			Version: versions.Cilium,
		}, pulumi.Provider(providers.Kubernetes))
		if err != nil {
			return err
		}

		// DNS and the API server both have to keep working under Cilium's
		// default-deny baseline, so both depend directly on Cilium
		// (specifically, the CiliumClusterwideNetworkPolicy CRD its Helm
		// chart installs). apiserver has to come first: DNS's own
		// ClusterDNS component patches CoreDNS's Deployment to grant it
		// apiserver access (its "kubernetes" plugin watches the K8s API
		// directly) and waits for that rollout to succeed, which can't
		// happen until the apiserver-access CiliumClusterwideNetworkPolicy
		// already exists.
		apiserverComp, err := apiserver.NewClusterAPIServer(ctx, "cluster-apiserver",
			pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{ciliumComp}),
		)
		if err != nil {
			return err
		}

		_, err = dns.NewClusterDNS(ctx, "cluster-dns",
			pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{ciliumComp, apiserverComp}),
		)
		if err != nil {
			return err
		}

		namespaces, err := createNamespaces(ctx, pulumi.Provider(providers.Kubernetes))
		if err != nil {
			return err
		}
		istioSystemNS := namespaces.Get(IstioSystemNamespace)
		longhornNS := namespaces.Get(LonghornSystemNamespace)
		cloudflareNS := namespaces.Get(CloudflareNamespace)
		tailscaleNS := namespaces.Get(TailscaleNamespace)
		healthNS := namespaces.Get(HealthNamespace)
		lumenetesNS := namespaces.Get(LumenetesNamespace)
		monitoringNS := namespaces.Get(MonitoringNamespace)
		workoutsNS := namespaces.Get(WorkoutsNamespace)
		shoppingNS := namespaces.Get(ShoppingNamespace)
		tripsNS := namespaces.Get(TripsNamespace)
		remindersNS := namespaces.Get(RemindersNamespace)
		financesNS := namespaces.Get(FinancesNamespace)
		homeNS := namespaces.Get(HomeNamespace)

		crds, err := installCRDs(ctx, IstioSystemNamespace,
			pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{istioSystemNS}),
		)
		if err != nil {
			return err
		}

		_, err = kubevip.NewKubeVip(ctx, "kube-vip", &kubevip.KubeVipArgs{
			VIP:     infraCfg.Cluster.VIP,
			Version: versions.KubeVip,
		}, pulumi.Provider(providers.Kubernetes))
		if err != nil {
			return err
		}

		mesh, err := istio.NewIstio(ctx, "istio", &istio.IstioArgs{
			Version:             versions.Istio,
			Namespace:           istioSystemNS.Metadata.Name().Elem(),
			PrometheusNamespace: monitoringNS.Metadata.Name().Elem(),
		}, pulumi.Provider(providers.Kubernetes),
			// crds.Prometheus: monitoring.go's PodMonitor objects need that
			// CRD to already exist - istiod/ztunnel/waypoint scraping is
			// wired up here even though Prometheus itself isn't deployed
			// until later, since these are just static label-selector
			// definitions, not live references to Prometheus's own pod.
			pulumi.DependsOn([]pulumi.Resource{crds.Istio, crds.GatewayAPI, crds.Prometheus, istioSystemNS, monitoringNS, ciliumComp}),
		)
		if err != nil {
			return err
		}

		// Gate everything behind Cloudflare Access. Created before Public
		// since Public's AccessJWT policy needs this application's AUD to
		// validate Access-issued JWTs against.
		access, err := cfauth.NewAccess(ctx, "homelab-access", &cfauth.AccessArgs{
			AccountID:       pulumi.String(infraCfg.Cloudflare.AccountID),
			Domain:          pulumi.Sprintf("*.%s", infraCfg.Cloudflare.Tunnel.Domain),
			AllowedEmails:   infraCfg.Cloudflare.Access.AllowedEmails,
			SessionDuration: pulumi.String("24h"),
			TeamDomain:      infraCfg.Cloudflare.Access.TeamDomain,
		}, pulumi.Provider(providers.Cloudflare))
		if err != nil {
			return err
		}

		// The tailnet's ACL policy - must exist before the operator/proxies
		// can register themselves under tag:k8s-operator/tag:k8s.
		tsAcl, err := tsacl.NewAcl(ctx, "tailscale-acl", &tsacl.AclArgs{
			Provider: providers.Tailscale,
		})
		if err != nil {
			return err
		}

		// Puts every Tailscale-fronted app's Service on the tailnet - see
		// pkg/components/tailscale/ingress for the per-app half of this.
		tsOperator, err := tailscale.NewOperator(ctx, "tailscale-operator", &tailscale.OperatorArgs{
			Namespace:         tailscaleNS.Metadata.Name().Elem(),
			Version:           versions.Tailscale,
			OAuthClientID:     pulumi.String(infraCfg.Tailscale.OAuthClientID),
			OAuthClientSecret: pulumi.String(infraCfg.Tailscale.OAuthClientSecret),
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{ciliumComp, tailscaleNS, tsAcl}),
		)
		if err != nil {
			return err
		}

		// Resolved once and shared by every Cloudflare DNS/Ruleset resource
		// below (pkg/components/tailscale/ingress.NewIngress and
		// createTailscaleRedirects) - createDomains has its own separate,
		// pre-existing lookup.
		zoneID := lookupZoneID(ctx,
			pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			pulumi.String(infraCfg.Cloudflare.AccountID),
			providers.Cloudflare,
		)

		public, err := applications.NewPublic(ctx, "public", &applications.PublicArgs{
			Namespace: healthNS.Metadata.Name().Elem(),
			Cloudflare: &accessjwt.Config{
				Access:          access,
				TunnelNamespace: cloudflareNS.Metadata.Name().Elem(),
			},
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, healthNS}),
		)
		if err != nil {
			return err
		}

		// Cloudflare Tunnel carries traffic from the edge straight to each
		// app's own Service (routed through that app's waypoint from
		// there) - no inbound firewall ports needed on the cluster's
		// network.
		tunnel, err := cftunnel.NewTunnel(ctx, "cloudflare-tunnel", &cftunnel.TunnelArgs{
			Domain:    pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			Namespace: cloudflareNS.Metadata.Name().Elem(),
			// Just the Cloudflare-dashboard display name for the tunnel
			// object - changing this string forces a full tunnel
			// replacement (new ID, secret, CNAME, cascading into the DNS
			// record). Leave it alone even though "gateway" no longer
			// describes anything else in this repo.
			TunnelName:          "homelab-gateway",
			Routes:              []cftunnel.TunnelRoute{public.TunnelRoute()},
			CloudflareAccountID: pulumi.String(infraCfg.Cloudflare.AccountID),
			CloudflareProvider:  providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{cloudflareNS, public, ciliumComp}),
		)
		if err != nil {
			return err
		}

		// Publish only the hostnames real apps above actually route - not a
		// blanket wildcard - as CNAMEs pointing at the tunnel.
		_, err = createDomains(ctx,
			pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			tunnel.TunnelCNAME,
			pulumi.String(infraCfg.Cloudflare.AccountID),
			providers.Cloudflare,
			pulumi.DependsOn([]pulumi.Resource{tunnel}),
		)
		if err != nil {
			return err
		}

		// Tailscale-only counterpart to Public - fully independent of
		// tunnel/public, no ordering relationship between them.
		_, err = applications.NewPrivate(ctx, "private", &applications.PrivateArgs{
			Namespace:                  healthNS.Metadata.Name().Elem(),
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, healthNS, tsOperator}),
		)
		if err != nil {
			return err
		}

		// Home: static React frontend, no API, no storage - just cards
		// linking out to every other app in this repo. Tailscale-only, same
		// exposure pattern as Private, placed here since (like Private) it
		// needs no storage and can go up independently of Longhorn.
		home, err := applications.NewHome(ctx, "home", &applications.HomeArgs{
			Namespace:                  homeNS.Metadata.Name().Elem(),
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
			GHCRUsername:               infraCfg.GHCR.Username,
			GHCRToken:                  infraCfg.GHCR.Token,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, homeNS, tsOperator}),
		)
		if err != nil {
			return err
		}

		// Longhorn provides the cluster's distributed block storage backend,
		// including a Tailscale-exposed UI (same exposure pattern as
		// Private) - fully independent of every app above, just placed here
		// since it needs both tsOperator and zoneID.
		storage, err := longhorn.NewLonghorn(ctx, "longhorn", &longhorn.LonghornArgs{
			Version:                    versions.Longhorn,
			Namespace:                  longhornNS.Metadata.Name().Elem(),
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, longhornNS, tsOperator}),
		)
		if err != nil {
			return err
		}

		// Workouts: SQLite-backed app on a Longhorn PVC, Tailscale-only -
		// needs storage.DefaultStorageClass, so has to come after Longhorn.
		_, err = applications.NewWorkouts(ctx, "workouts", &applications.WorkoutsArgs{
			Namespace:                  workoutsNS.Metadata.Name().Elem(),
			StorageClassName:           storage.DefaultStorageClass,
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
			GHCRUsername:               infraCfg.GHCR.Username,
			GHCRToken:                  infraCfg.GHCR.Token,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, workoutsNS, tsOperator, storage}),
		)
		if err != nil {
			return err
		}

		// Shopping: same shape as Workouts - SQLite-backed app on a Longhorn
		// PVC, Tailscale-only - needs storage.DefaultStorageClass, so has to
		// come after Longhorn.
		_, err = applications.NewShopping(ctx, "shopping", &applications.ShoppingArgs{
			Namespace:                  shoppingNS.Metadata.Name().Elem(),
			StorageClassName:           storage.DefaultStorageClass,
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
			GHCRUsername:               infraCfg.GHCR.Username,
			GHCRToken:                  infraCfg.GHCR.Token,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, shoppingNS, tsOperator, storage}),
		)
		if err != nil {
			return err
		}

		// Trips: same shape as Workouts/Shopping - SQLite-backed app on a
		// Longhorn PVC, Tailscale-only - needs storage.DefaultStorageClass,
		// so has to come after Longhorn.
		_, err = applications.NewTrips(ctx, "trips", &applications.TripsArgs{
			Namespace:                  tripsNS.Metadata.Name().Elem(),
			StorageClassName:           storage.DefaultStorageClass,
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
			GHCRUsername:               infraCfg.GHCR.Username,
			GHCRToken:                  infraCfg.GHCR.Token,
			FlightAPIKey:               pulumi.String(infraCfg.FlightData.APIKey),
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, tripsNS, tsOperator, storage}),
		)
		if err != nil {
			return err
		}

		// Reminders: same shape as Workouts/Shopping/Trips - SQLite-backed
		// app on a Longhorn PVC, Tailscale-only - needs
		// storage.DefaultStorageClass, so has to come after Longhorn.
		_, err = applications.NewReminders(ctx, "reminders", &applications.RemindersArgs{
			Namespace:                  remindersNS.Metadata.Name().Elem(),
			StorageClassName:           storage.DefaultStorageClass,
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
			GHCRUsername:               infraCfg.GHCR.Username,
			GHCRToken:                  infraCfg.GHCR.Token,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, remindersNS, tsOperator, storage}),
		)
		if err != nil {
			return err
		}

		// Finances: same shape as Workouts/Shopping/Trips/Reminders -
		// SQLite-backed app on a Longhorn PVC, Tailscale-only - needs
		// storage.DefaultStorageClass, so has to come after Longhorn.
		_, err = applications.NewFinances(ctx, "finances", &applications.FinancesArgs{
			Namespace:                  financesNS.Metadata.Name().Elem(),
			StorageClassName:           storage.DefaultStorageClass,
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
			GHCRUsername:               infraCfg.GHCR.Username,
			GHCRToken:                  infraCfg.GHCR.Token,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, financesNS, tsOperator, storage}),
		)
		if err != nil {
			return err
		}

		// Metrics-collection plane: prometheus-operator, the Prometheus CR,
		// and its exporters (node-exporter, kube-state-metrics, cadvisor's
		// ServiceMonitor) - hand-rolled Go resources rather than a Helm
		// chart (see pkg/components/prometheus's package doc comment for
		// why). Depends on storage.DefaultStorageClass for its PVC, so has
		// to come after Longhorn.
		monitoring, err := prometheus.NewPrometheus(ctx, "monitoring", &prometheus.PrometheusArgs{
			Namespace:                 monitoringNS.Metadata.Name().Elem(),
			GrafanaServiceAccountName: grafana.ServiceAccountName,
			OperatorVersion:           versions.PrometheusOperator,
			Version:                   versions.Prometheus,
			NodeExporterVersion:       versions.NodeExporter,
			KubeStateMetricsVersion:   versions.KubeStateMetrics,
			AlertmanagerVersion:       versions.Alertmanager,
			KubeRBACProxyVersion:      versions.KubeRBACProxy,
			StorageClassName:          storage.DefaultStorageClass,
			// Data retention strategy: a size cap alongside the time-based
			// one, so Prometheus proactively compacts away old blocks the
			// moment either limit is hit rather than risking a disk-full
			// crash if ingestion ever grows faster than expected - see
			// instance.go's doc comment for the full reasoning.
			StorageSize:                "20Gi",
			Retention:                  "14d",
			RetentionSize:              "18GB",
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.Prometheus, crds.GatewayAPI, crds.Istio, mesh, ciliumComp, apiserverComp, monitoringNS, storage, tsOperator}),
		)
		if err != nil {
			return err
		}

		// Tailscale-exposed Grafana UI, wired to the Prometheus Service
		// above - same exposure pattern as Longhorn's UI and Private.
		_, err = grafana.NewGrafana(ctx, "grafana", &grafana.GrafanaArgs{
			Version:                    versions.Grafana,
			Namespace:                  monitoringNS.Metadata.Name().Elem(),
			PrometheusServiceName:      pulumi.String(prometheus.ServiceName),
			PrometheusWaypointName:     monitoring.UIWaypointName,
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, crds.Istio, mesh, ciliumComp, monitoringNS, tsOperator, monitoring}),
		)
		if err != nil {
			return err
		}

		// Builds the shared lumenetes-controller/hub-controller image and
		// deploys both against it (see pkg/deploy/applications/lumenetes.go
		// for the full reasoning, including hub-controller's
		// HostNetwork: true and why there's no DependsOn between the two
		// components - they only interact at runtime via the Kubernetes
		// API, not at deploy time).
		_, err = applications.NewLumenetes(ctx, &applications.LumenetesArgs{
			Namespace:       lumenetesNS.Metadata.Name().Elem(),
			Bridges:         infraCfg.Lumenetes.Hue.Bridges,
			Location:        infraCfg.Lumenetes.Location,
			GHCRUsername:    infraCfg.GHCR.Username,
			GHCRToken:       infraCfg.GHCR.Token,
			HubPollInterval: pulumi.String("60s"),
			// Now a drift safety net behind the real-time eventstream path,
			// not the primary sync mechanism.
			LightsPollInterval: pulumi.String("30s"),
			// Dry-run: the reconciler only logs Light.Spec drift instead of
			// pushing it to the physical bridge. Scoped entirely to
			// lumenetescontroller.Reconciler - doesn't affect hub-controller,
			// Switch, or Group at all.
			DryRun:                     pulumi.Bool(true),
			PrometheusNamespace:        monitoringNS.Metadata.Name().Elem(),
			TailscaleOperatorNamespace: tailscaleNS.Metadata.Name().Elem(),
			TailscaleMagicDNSSuffix:    pulumi.String(infraCfg.Tailscale.MagicDNSSuffix),
			CloudflareZoneID:           zoneID,
			CloudflareBaseDomain:       pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareProvider:         providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes),
			// crds.Prometheus/monitoringNS: lumenetescontroller's own
			// PodMonitor+CCNP metrics wiring needs both - same reasoning as
			// mesh's (istio.NewIstio) identical DependsOn addition above.
			// tsOperator: its embedded web UI's Tailscale ingress needs the
			// operator installed - same reasoning as workouts/shopping.
			pulumi.DependsOn([]pulumi.Resource{crds.Lumenetes, crds.Prometheus, lumenetesNS, monitoringNS, ciliumComp, apiserverComp, tsOperator}),
		)
		if err != nil {
			return err
		}

		// Cloudflare-side redirect bookmark - deliberately only home.
		// liamwhite.fyi -> home.<tailnet>: the zone's plan caps the
		// http_request_dynamic_redirect phase at 10 rules, and we were
		// sitting right at that cap with one rule per Tailscale-fronted
		// app. Every other app (including prom/alerts, which were never
		// in this list) stays reachable via its own Tailscale MagicDNS
		// hostname instead of a liamwhite.fyi bookmark.
		_, err = createTailscaleRedirects(ctx,
			zoneID,
			pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			providers.Cloudflare,
			[]tsingress.RedirectRoute{home.TailscaleRedirect()},
			pulumi.DependsOn([]pulumi.Resource{home}),
		)
		if err != nil {
			return err
		}

		return nil
	}
}
