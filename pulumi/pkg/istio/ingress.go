package istio

import (
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// DeployIngressGateway deploys the Istio ingress gateway using Helm
func DeployIngressGateway(ctx *pulumi.Context, namespace pulumi.StringOutput, version string, opts ...pulumi.ResourceOption) (*helmv4.Chart, error) {
	// Helm repository configuration
	repositoryOpts := &helmv4.RepositoryOptsArgs{
		Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
	}

	// Deploy Istio ingress gateway
	gateway, err := helmv4.NewChart(ctx, "istio-ingressgateway", &helmv4.ChartArgs{
		Chart:          pulumi.String("gateway"),
		Version:        pulumi.String(version),
		Namespace:      namespace,
		RepositoryOpts: repositoryOpts,
		Values: pulumi.Map{
			"service": pulumi.Map{
				"type": pulumi.String("ClusterIP"), // Use ClusterIP since we're behind Cloudflare Tunnel
			},
		},
	}, opts...)

	return gateway, err
}
