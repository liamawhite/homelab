package gatewayapi

import (
	_ "embed"

	yamlv2 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// crdManifest is the standard-channel Gateway API CRD manifest downloaded by
// gen-crds.sh at versions.GatewayAPI, embedded at build time so installing
// it doesn't depend on that file being present on disk at runtime.
//
//go:embed gateway-api-crds.yaml
var crdManifest string

// InstallCRDs applies the Gateway API CRDs (GatewayClass, Gateway,
// HTTPRoute, GRPCRoute, ReferenceGrant, BackendTLSPolicy, ...) to the
// cluster.
func InstallCRDs(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*yamlv2.ConfigGroup, error) {
	return yamlv2.NewConfigGroup(ctx, name, &yamlv2.ConfigGroupArgs{
		Yaml: pulumi.String(crdManifest),
	}, opts...)
}
