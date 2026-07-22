package lights

import (
	_ "embed"

	yamlv2 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// lightCRDManifest, huebridgeCRDManifest, switchCRDManifest, and
// groupCRDManifest are the generated CRD manifests (see gen-crds.sh),
// embedded at build time so installing them doesn't depend on these files
// being present on disk at runtime.
//
//go:embed light-crd.yaml
var lightCRDManifest string

//go:embed huebridge-crd.yaml
var huebridgeCRDManifest string

//go:embed switch-crd.yaml
var switchCRDManifest string

//go:embed group-crd.yaml
var groupCRDManifest string

// InstallCRDs applies the Light, HueBridge, Switch, and Group CRDs to the
// cluster. Nothing in any manifest is namespaced (all four CRDs are
// cluster-scoped, like the CustomResourceDefinition objects that define
// them), so unlike pkg/crds/istio.InstallCRDs this takes no namespace
// argument.
func InstallCRDs(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*yamlv2.ConfigGroup, error) {
	return yamlv2.NewConfigGroup(ctx, name, &yamlv2.ConfigGroupArgs{
		Yaml: pulumi.String(lightCRDManifest + "\n---\n" + huebridgeCRDManifest + "\n---\n" + switchCRDManifest + "\n---\n" + groupCRDManifest),
	}, opts...)
}
