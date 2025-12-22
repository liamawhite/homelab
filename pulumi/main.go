package main

import (
	"github.com/liamawhite/homelab/pulumi/pkg/gatewayapi"
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

		// 2. Create Kubernetes provider from kubeconfig
		provider, err := NewProvider(ctx)
		if err != nil {
			return err
		}

		// 3. Deploy kube-vip
		_, err = kubevip.NewKubeVip(ctx, "kube-vip", &kubevip.KubeVipArgs{
			VIP:     cfg.VIP,
			Version: cfg.KubeVip.Version,
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// 4. Deploy Gateway API (dependency for Istio)
		gateway, err := gatewayapi.NewGatewayAPI(ctx, "gateway-api", &gatewayapi.GatewayAPIArgs{
			Version: cfg.GatewayAPI.Version,
		}, pulumi.Provider(provider))
		if err != nil {
			return err
		}

		// 5. Deploy Istio (depends on Gateway API)
		_, err = istio.NewIstio(ctx, "istio", &istio.IstioArgs{
			Version: cfg.Istio.Version,
		}, pulumi.Provider(provider), pulumi.DependsOn([]pulumi.Resource{gateway}))
		if err != nil {
			return err
		}

		return nil
	})
}
