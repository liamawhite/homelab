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
	"github.com/liamawhite/homelab/pkg/components/istio"
	"github.com/liamawhite/homelab/pkg/components/kubevip"
	"github.com/liamawhite/homelab/pkg/components/longhorn"
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
			Version:   versions.Istio,
			Namespace: istioSystemNS.Metadata.Name().Elem(),
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.Istio, crds.GatewayAPI, istioSystemNS, ciliumComp}),
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
		private, err := applications.NewPrivate(ctx, "private", &applications.PrivateArgs{
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

		// Cloudflare-side redirect bookmarks for every Tailscale-fronted
		// app - see pkg/deploy/redirects.go for why this has to be
		// collected centrally rather than each app creating its own.
		_, err = createTailscaleRedirects(ctx,
			zoneID,
			pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			providers.Cloudflare,
			[]tsingress.RedirectRoute{private.TailscaleRedirect(), storage.TailscaleRedirect()},
			pulumi.DependsOn([]pulumi.Resource{private, storage}),
		)
		if err != nil {
			return err
		}

		return nil
	}
}
