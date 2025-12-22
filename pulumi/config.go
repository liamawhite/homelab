package main

import (
	"fmt"

	infraconfig "github.com/liamawhite/homelab/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Config represents the complete Pulumi configuration
type Config struct {
	VIP        string
	KubeVip    KubeVipConfig
	GatewayAPI GatewayAPIConfig
	Istio      IstioConfig
}

// KubeVipConfig represents kube-vip specific configuration
type KubeVipConfig struct {
	Version string `json:"version"`
}

// GatewayAPIConfig represents Gateway API specific configuration
type GatewayAPIConfig struct {
	Version string `json:"version"`
}

// IstioConfig represents Istio specific configuration
type IstioConfig struct {
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
	var gatewayAPICfg GatewayAPIConfig
	var istioCfg IstioConfig

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

	// Try to get Gateway API object from Pulumi config (with defaults)
	if err := pulumiCfg.TryObject("gatewayapi", &gatewayAPICfg); err != nil {
		// Use defaults if Pulumi config not present
		gatewayAPICfg = GatewayAPIConfig{
			Version: "1.2.0",
		}
	}

	// Apply defaults if fields are empty
	if gatewayAPICfg.Version == "" {
		gatewayAPICfg.Version = "1.2.0"
	}

	// Try to get Istio object from Pulumi config (with defaults)
	if err := pulumiCfg.TryObject("istio", &istioCfg); err != nil {
		// Use defaults if Pulumi config not present
		istioCfg = IstioConfig{
			Version: "1.28.2",
		}
	}

	// Apply defaults if fields are empty
	if istioCfg.Version == "" {
		istioCfg.Version = "1.28.2"
	}

	// 3. Build final config
	return &Config{
		VIP:        infraCfg.Cluster.VIP,
		KubeVip:    kubeVipCfg,
		GatewayAPI: gatewayAPICfg,
		Istio:      istioCfg,
	}, nil
}
