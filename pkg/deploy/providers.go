package deploy

import (
	"fmt"

	infraconfig "github.com/liamawhite/homelab/pkg/config"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NewKubernetesProvider creates a Kubernetes provider from kubeconfig
// content resolved by the caller (see cli/pkg/k3s.ResolveClusterEndpoint)
// rather than a fixed file path, since the right server address to embed
// depends on whether kube-vip is up yet.
func NewKubernetesProvider(ctx *pulumi.Context, kubeconfig string) (*kubernetes.Provider, error) {
	provider, err := kubernetes.NewProvider(ctx, "k8s", &kubernetes.ProviderArgs{
		Kubeconfig: pulumi.String(kubeconfig),
	})
	if err != nil {
		return nil, err
	}

	return provider, nil
}

// NewCloudflareProvider creates a Cloudflare provider with an explicit API
// token (see infra.yaml's cloudflare.apiToken).
func NewCloudflareProvider(ctx *pulumi.Context, apiToken string) (*cloudflare.Provider, error) {
	provider, err := cloudflare.NewProvider(ctx, "cloudflare", &cloudflare.ProviderArgs{
		ApiToken: pulumi.String(apiToken),
	})
	if err != nil {
		return nil, err
	}

	return provider, nil
}

// Providers holds all infrastructure providers
type Providers struct {
	Kubernetes *kubernetes.Provider
	Cloudflare *cloudflare.Provider
}

// NewProviders validates the infra.yaml fields providers need and creates
// all required infrastructure providers.
func NewProviders(ctx *pulumi.Context, kubeconfig string, infraCfg *infraconfig.InfraConfig) (*Providers, error) {
	if infraCfg.Cluster.VIP == "" {
		return nil, fmt.Errorf("cluster VIP is required in infra.yaml")
	}
	if infraCfg.Cloudflare.AccountID == "" {
		return nil, fmt.Errorf("cloudflare account ID is required in infra.yaml")
	}
	if infraCfg.Cloudflare.Tunnel.Domain == "" {
		return nil, fmt.Errorf("cloudflare tunnel domain is required in infra.yaml")
	}
	if infraCfg.Tailscale.OAuthClientID == "" {
		return nil, fmt.Errorf("tailscale OAuth client ID is required in infra.yaml")
	}
	if infraCfg.Tailscale.OAuthClientSecret == "" {
		return nil, fmt.Errorf("tailscale OAuth client secret is required in infra.yaml")
	}
	if infraCfg.Tailscale.MagicDNSSuffix == "" {
		return nil, fmt.Errorf("tailscale MagicDNS suffix is required in infra.yaml")
	}

	// Create Kubernetes provider from the resolved kubeconfig
	k8sProvider, err := NewKubernetesProvider(ctx, kubeconfig)
	if err != nil {
		return nil, err
	}

	cfProvider, err := NewCloudflareProvider(ctx, infraCfg.Cloudflare.APIToken)
	if err != nil {
		return nil, err
	}

	return &Providers{
		Kubernetes: k8sProvider,
		Cloudflare: cfProvider,
	}, nil
}
