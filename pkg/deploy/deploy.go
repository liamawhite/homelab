// Package deploy holds the Pulumi program that deploys the homelab's
// Pulumi-managed infrastructure. It's a plain library (not `package main`)
// so the CLI can run it fully inline via the Pulumi Automation API,
// supplying a kubeconfig it has resolved itself instead of the program
// reading one from a fixed path, with no on-disk Pulumi project required.
package deploy

import (
	cfauth "github.com/liamawhite/homelab/pkg/components/cloudflare/auth"
	cfdomain "github.com/liamawhite/homelab/pkg/components/cloudflare/domain"
	cftunnel "github.com/liamawhite/homelab/pkg/components/cloudflare/tunnel"
	"github.com/liamawhite/homelab/pkg/components/istio"
	istiogateway "github.com/liamawhite/homelab/pkg/components/istio/gateway"
	"github.com/liamawhite/homelab/pkg/components/kubevip"
	infraconfig "github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/deploy/applications"
	"github.com/liamawhite/homelab/pkg/versions"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Program returns the Pulumi program that deploys kube-vip, the Istio
// control plane, and the shared ingress Gateway against the cluster
// reachable via kubeconfig.
func Program(kubeconfig string, infraCfg *infraconfig.InfraConfig) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		providers, err := NewProviders(ctx, kubeconfig, infraCfg)
		if err != nil {
			return err
		}

		namespaces, err := createNamespaces(ctx, pulumi.Provider(providers.Kubernetes))
		if err != nil {
			return err
		}
		istioSystemNS := namespaces.Get(IstioSystemNamespace)
		cloudflareNS := namespaces.Get(CloudflareNamespace)

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
			pulumi.DependsOn([]pulumi.Resource{crds.Istio, crds.GatewayAPI, istioSystemNS}),
		)
		if err != nil {
			return err
		}

		// Gate everything routed via the tunnel (once it exists) behind
		// Cloudflare Access. Created before the Gateway since the Gateway's
		// AuthorizationPolicy needs this application's AUD to validate
		// Access-issued JWTs against.
		access, err := cfauth.NewAccess(ctx, "homelab-access", &cfauth.AccessArgs{
			AccountID:       pulumi.String(infraCfg.Cloudflare.AccountID),
			Domain:          pulumi.Sprintf("*.%s", infraCfg.Cloudflare.Tunnel.Domain),
			AllowedEmails:   infraCfg.Cloudflare.Access.AllowedEmails,
			SessionDuration: pulumi.String("24h"),
		}, pulumi.Provider(providers.Cloudflare))
		if err != nil {
			return err
		}

		gw, err := istiogateway.NewGateway(ctx, "shared-gateway", &istiogateway.GatewayArgs{
			Namespace:            istioSystemNS.Metadata.Name().Elem(),
			Domain:               pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			CloudflareTeamDomain: pulumi.String(infraCfg.Cloudflare.Access.TeamDomain),
			CloudflareAccessAUD:  access.AUD,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, mesh, istioSystemNS}),
		)
		if err != nil {
			return err
		}

		// Cloudflare Tunnel carries traffic from the edge to the shared
		// Gateway's auto-provisioned Service - no inbound firewall ports
		// needed on the cluster's network.
		tunnel, err := cftunnel.NewTunnel(ctx, "cloudflare-tunnel", &cftunnel.TunnelArgs{
			Domain:     pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			Namespace:  cloudflareNS.Metadata.Name().Elem(),
			TunnelName: "homelab-gateway",
			// Still routed at the shared Gateway's Service, same behavior as
			// before the Routes API existed - switching this to
			// home.TunnelRoute() (routing straight to home's Service through
			// its waypoint) is a follow-up step, not yet done.
			Routes: []cftunnel.TunnelRoute{
				{
					Subdomain:        "*",
					ServiceName:      gw.ServiceName,
					ServiceNamespace: gw.ServiceNamespace,
					ServicePort:      80,
				},
			},
			CloudflareAccountID: pulumi.String(infraCfg.Cloudflare.AccountID),
			CloudflareProvider:  providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{cloudflareNS, gw}),
		)
		if err != nil {
			return err
		}

		// Publish only the hostnames real apps below actually route - not a
		// blanket wildcard - as CNAMEs pointing at the tunnel.
		domains, err := createDomains(ctx,
			pulumi.String(infraCfg.Cloudflare.Tunnel.Domain),
			tunnel.TunnelCNAME,
			pulumi.String(infraCfg.Cloudflare.AccountID),
			providers.Cloudflare,
			pulumi.DependsOn([]pulumi.Resource{tunnel}),
		)
		if err != nil {
			return err
		}

		_, err = applications.NewHome(ctx, "home", &applications.HomeArgs{
			Namespace:        pulumi.String("default"),
			Domains:          []*cfdomain.Domain{domains.Get("home")},
			GatewayName:      gw.Name,
			GatewayNamespace: gw.Namespace,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{crds.GatewayAPI, gw}),
		)
		if err != nil {
			return err
		}

		return nil
	}
}
