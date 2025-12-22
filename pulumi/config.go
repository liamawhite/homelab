package main

import (
	"fmt"

	infraconfig "github.com/liamawhite/homelab/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Config represents the complete Pulumi configuration
type Config struct {
	VIP     string
	KubeVip KubeVipConfig
}

// KubeVipConfig represents kube-vip specific configuration
type KubeVipConfig struct {
	Version string `json:"version"`
}

// LoadConfig loads configuration from infra.yaml and Pulumi config
// with proper precedence and validation
func LoadConfig(ctx *pulumi.Context) (*Config, error) {
	// 1. Load base configuration from infra.yaml
	infraCfg, err := infraconfig.LoadFromFile("../infra.yaml")
	if err != nil {
		return nil, err
	}

	// Validate VIP is configured
	if infraCfg.Cluster.VIP == "" {
		return nil, fmt.Errorf("cluster VIP is required in infra.yaml")
	}

	// 2. Get Pulumi-specific overrides
	pulumiCfg := config.New(ctx, "homelab")
	var kubeVipCfg KubeVipConfig

	// Try to get kube-vip object from Pulumi config
	if err := pulumiCfg.TryObject("kubevip", &kubeVipCfg); err != nil {
		// Use defaults from infra.yaml if Pulumi config not present
		kubeVipCfg = KubeVipConfig{
			Version: infraCfg.KubeVip.Version,
		}
	}

	// Apply defaults if fields are empty (Pulumi config takes precedence)
	if kubeVipCfg.Version == "" {
		kubeVipCfg.Version = infraCfg.KubeVip.Version
	}

	// 3. Build final config
	return &Config{
		VIP:     infraCfg.Cluster.VIP,
		KubeVip: kubeVipCfg,
	}, nil
}
