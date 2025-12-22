package main

import (
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

		return nil
	})
}
