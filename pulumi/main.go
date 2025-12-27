package main

import (
	cftunnel "github.com/liamawhite/homelab/pulumi/pkg/cloudflare/tunnel"
	"github.com/liamawhite/homelab/pulumi/pkg/gateway"
	"github.com/liamawhite/homelab/pulumi/pkg/istio"
	"github.com/liamawhite/homelab/pulumi/pkg/kubevip"
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

		// 5. Deploy Istio
		istioMesh, err := istio.NewIstio(ctx, "istio", &istio.IstioArgs{
			Version: cfg.Istio.Version,
		}, pulumi.Provider(providers.Kubernetes))
		if err != nil {
			return err
		}

		// 6. Create main Istio Gateway (depends on Istio)
		mainGateway, err := gateway.NewGateway(ctx, "main-gateway", &gateway.GatewayArgs{
			Namespace: istioMesh.Namespace,
		}, pulumi.Provider(providers.Kubernetes))
		if err != nil {
			return err
		}

		// 7. Create health check endpoint
		err = gateway.NewHealthCheck(ctx, istioMesh.Namespace, "main-gateway", "health.liamwhite.fyi", pulumi.Provider(providers.Kubernetes), pulumi.DependsOn([]pulumi.Resource{mainGateway}))
		if err != nil {
			return err
		}

		// 8. Create Cloudflare Tunnel (depends on Istio ingress gateway)
		_, err = cftunnel.NewTunnel(ctx, "gateway-tunnel", &cftunnel.TunnelArgs{
			Domain:              cfg.Cloudflare.Tunnel.Domain,
			Subdomain:           cfg.Cloudflare.Tunnel.Subdomain,
			TunnelName:          "homelab-gateway",
			GatewayNamespace:    istioMesh.Namespace,
			GatewayService:      pulumi.String("istio-ingressgateway"),
			CloudflareAccountID: pulumi.String(cfg.Cloudflare.AccountID),
			CloudflareProvider:  providers.Cloudflare,
		}, pulumi.Provider(providers.Kubernetes), pulumi.Providers(providers.Cloudflare))
		if err != nil {
			return err
		}

		return nil
	})
}
