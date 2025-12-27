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
	Cloudflare CloudflareConfig
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

// CloudflareConfig represents Cloudflare specific configuration
type CloudflareConfig struct {
	AccountID string       `json:"accountId"`
	APIToken  string       `json:"apiToken"`
	Tunnel    TunnelConfig `json:"tunnel"`
}

// TunnelConfig represents Cloudflare Tunnel specific configuration
type TunnelConfig struct {
	Domain    pulumi.StringOutput `json:"-"` // e.g., "liamwhite.fyi"
	Subdomain pulumi.StringOutput `json:"-"` // e.g., "*" for wildcard
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

	// Get Cloudflare config from Pulumi config
	type cfConfigInput struct {
		AccountID string `json:"accountId"`
		APIToken  string `json:"apiToken"`
	}

	var cfInput cfConfigInput
	if err := pulumiCfg.TryObject("cloudflare", &cfInput); err != nil {
		return nil, fmt.Errorf("cloudflare configuration is required: %w", err)
	}

	// Validate Cloudflare credentials are provided
	if cfInput.AccountID == "" {
		return nil, fmt.Errorf("cloudflare account ID is required in Pulumi config (homelab:cloudflare.accountId)")
	}
	if cfInput.APIToken == "" {
		return nil, fmt.Errorf("cloudflare API token is required in Pulumi config (homelab:cloudflare.apiToken)")
	}

	// Get tunnel config from the cloudflare object
	type tunnelInput struct {
		Domain    string `json:"domain"`
		Subdomain string `json:"subdomain"`
	}
	var tunnel tunnelInput
	if err := pulumiCfg.TryObject("cloudflare:tunnel", &tunnel); err != nil {
		// Use defaults
		tunnel = tunnelInput{
			Domain:    "liamwhite.fyi",
			Subdomain: "*",
		}
	}
	if tunnel.Domain == "" {
		tunnel.Domain = "liamwhite.fyi"
	}
	if tunnel.Subdomain == "" {
		tunnel.Subdomain = "*"
	}

	// Convert to CloudflareConfig
	cloudflareCfg := CloudflareConfig{
		AccountID: cfInput.AccountID,
		APIToken:  cfInput.APIToken,
		Tunnel: TunnelConfig{
			Domain:    pulumi.String(tunnel.Domain).ToStringOutput(),
			Subdomain: pulumi.String(tunnel.Subdomain).ToStringOutput(),
		},
	}

	// 3. Build final config
	return &Config{
		VIP:        infraCfg.Cluster.VIP,
		KubeVip:    kubeVipCfg,
		GatewayAPI: gatewayAPICfg,
		Istio:      istioCfg,
		Cloudflare: cloudflareCfg,
	}, nil
}
