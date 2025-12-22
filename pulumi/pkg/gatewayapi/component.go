package gatewayapi

import (
	"embed"
	"fmt"

	yamlv2 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Embed Gateway API YAML files
//
//go:embed gateway-api-*.yaml
var yamlFiles embed.FS

// GatewayAPI represents the Kubernetes Gateway API CRDs
type GatewayAPI struct {
	pulumi.ResourceState
}

// GatewayAPIArgs contains the configuration for Gateway API
type GatewayAPIArgs struct {
	// Version is the Gateway API version to deploy (e.g., "1.2.0")
	Version string
}

// NewGatewayAPI creates a new Gateway API component
func NewGatewayAPI(ctx *pulumi.Context, name string, args *GatewayAPIArgs, opts ...pulumi.ResourceOption) (*GatewayAPI, error) {
	gateway := &GatewayAPI{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:gatewayapi", name, gateway, opts...)
	if err != nil {
		return nil, err
	}

	// All child resources should be parented to this component
	localOpts := append(opts, pulumi.Parent(gateway))

	// Read embedded YAML file for the requested version
	filename := fmt.Sprintf("gateway-api-v%s.yaml", args.Version)
	yamlContent, err := yamlFiles.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded Gateway API YAML for version %s: %w", args.Version, err)
	}

	// Install Gateway API CRDs from embedded YAML
	_, err = yamlv2.NewConfigGroup(ctx, name, &yamlv2.ConfigGroupArgs{
		Yaml: pulumi.String(string(yamlContent)),
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// Register outputs
	if err := ctx.RegisterResourceOutputs(gateway, pulumi.Map{}); err != nil {
		return nil, err
	}

	return gateway, nil
}
