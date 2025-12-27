package main

import (
	cftunnel "github.com/liamawhite/homelab/pulumi/pkg/cloudflare/tunnel"
	"github.com/liamawhite/homelab/pulumi/pkg/istio"
	"github.com/liamawhite/homelab/pulumi/pkg/kubevip"
	"github.com/liamawhite/homelab/pulumi/pkg/longhorn"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// 1. Load and validate configuration
		cfg, err := LoadConfig(ctx)
		if err != nil {
			return err
		}

		// 2. Create infrastructure providers
		providers, err := NewProviders(ctx, cfg)
		if err != nil {
			return err
		}

		// 3. Deploy kube-vip
		_, err = kubevip.NewKubeVip(ctx, "kube-vip", &kubevip.KubeVipArgs{
			VIP:     cfg.VIP,
			Version: cfg.KubeVip.Version,
		}, pulumi.Provider(providers.Kubernetes))
		if err != nil {
			return err
		}

		// 4. Deploy Istio (includes ingress gateway, main gateway, and health check)
		istioMesh, err := istio.NewIstio(ctx, "istio", &istio.IstioArgs{
			Version:         cfg.Istio.Version,
			GatewayDomain:   cfg.Cloudflare.Tunnel.Domain,
			HealthSubdomain: pulumi.String("health"),
		}, pulumi.Provider(providers.Kubernetes))
		if err != nil {
			return err
		}

		// 5. Deploy Longhorn storage system
		_, err = longhorn.NewLonghorn(ctx, "longhorn", &longhorn.LonghornArgs{
			Version: cfg.Longhorn.Version,
		}, pulumi.Provider(providers.Kubernetes),
			pulumi.DependsOn([]pulumi.Resource{istioMesh}),
		)
		if err != nil {
			return err
		}

		// 6. Create Cloudflare Tunnel
		_, err = cftunnel.NewTunnel(ctx, "gateway-tunnel", &cftunnel.TunnelArgs{
			Domain:              cfg.Cloudflare.Tunnel.Domain,
			Subdomain:           "*",
			TunnelName:          "homelab-gateway",
			GatewayNamespace:    istioMesh.Namespace,
			GatewayService:      istioMesh.GatewayService,
			CloudflareAccountID: pulumi.String(cfg.Cloudflare.AccountID),
			CloudflareProvider:  providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes), pulumi.Providers(providers.Cloudflare))
		if err != nil {
			return err
		}

		return nil
	})
}
