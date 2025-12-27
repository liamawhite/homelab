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
	Istio      IstioConfig
	Longhorn   LonghornConfig
	Cloudflare CloudflareConfig
}

// KubeVipConfig represents kube-vip specific configuration
type KubeVipConfig struct {
	Version string `json:"version"`
}

// IstioConfig represents Istio specific configuration
type IstioConfig struct {
	Version string `json:"version"`
}

// LonghornConfig represents Longhorn specific configuration
type LonghornConfig struct {
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
	Domain pulumi.StringOutput `json:"-"`
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
	var istioCfg IstioConfig
	var longhornCfg LonghornConfig

	// Get kube-vip config from Pulumi config
	if err := pulumiCfg.TryObject("kubevip", &kubeVipCfg); err != nil {
		return nil, fmt.Errorf("kube-vip configuration is required in Pulumi config (homelab:kubevip.version): %w", err)
	}
	if kubeVipCfg.Version == "" {
		return nil, fmt.Errorf("kube-vip version is required (set in Pulumi config homelab:kubevip.version)")
	}

	// Get Istio config from Pulumi config
	if err := pulumiCfg.TryObject("istio", &istioCfg); err != nil {
		return nil, fmt.Errorf("istio configuration is required in Pulumi config (homelab:istio.version): %w", err)
	}
	if istioCfg.Version == "" {
		return nil, fmt.Errorf("istio version is required (set in Pulumi config homelab:istio.version)")
	}

	// Get Longhorn config from Pulumi config
	if err := pulumiCfg.TryObject("longhorn", &longhornCfg); err != nil {
		return nil, fmt.Errorf("longhorn configuration is required in Pulumi config (homelab:longhorn.version): %w", err)
	}
	if longhornCfg.Version == "" {
		return nil, fmt.Errorf("longhorn version is required (set in Pulumi config homelab:longhorn.version)")
	}

	// Get Cloudflare config from Pulumi config
	type tunnelInput struct {
		Domain string `json:"domain"`
	}
	type cfConfigInput struct {
		AccountID string      `json:"accountId"`
		APIToken  string      `json:"apiToken"`
		Tunnel    tunnelInput `json:"tunnel"`
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
	if cfInput.Tunnel.Domain == "" {
		return nil, fmt.Errorf("cloudflare tunnel domain is required in Pulumi config (homelab:cloudflare.tunnel.domain)")
	}

	// Convert to CloudflareConfig
	cloudflareCfg := CloudflareConfig{
		AccountID: cfInput.AccountID,
		APIToken:  cfInput.APIToken,
		Tunnel: TunnelConfig{
			Domain: pulumi.String(cfInput.Tunnel.Domain).ToStringOutput(),
		},
	}

	// 3. Build final config
	return &Config{
		VIP:        infraCfg.Cluster.VIP,
		KubeVip:    kubeVipCfg,
		Istio:      istioCfg,
		Longhorn:   longhornCfg,
		Cloudflare: cloudflareCfg,
	}, nil
}
