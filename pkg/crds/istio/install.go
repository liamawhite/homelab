package istio

import (
	_ "embed"
	"fmt"

	yamlv2 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// crdManifest is the Istio "base" chart's CRDs (plus the couple of
// supporting resources it ships alongside them - a reader ServiceAccount
// and the istiod default ValidatingWebhookConfiguration), extracted by
// gen-crds.sh at versions.Istio via `helm template --include-crds` and
// embedded at build time so installing it doesn't depend on that file
// being present on disk at runtime.
//
//go:embed istio-crds.yaml
var crdManifest string

// crdNamespace is where the bundled istio-reader-service-account lives.
const crdNamespace = "istio-system"

// InstallCRDs applies the Istio CRDs (Gateway, VirtualService,
// DestinationRule, ServiceEntry, ...) to the cluster. namespace must be
// istio-system - the embedded manifest isn't templated, so this is a
// consistency check against the caller's namespace (created centrally by
// pkg/deploy/namespaces.go), not a substitution. Callers must pass
// pulumi.DependsOn on that namespace resource in opts, since the bundled
// istio-reader-service-account is namespaced to it.
func InstallCRDs(ctx *pulumi.Context, name string, namespace string, opts ...pulumi.ResourceOption) (*yamlv2.ConfigGroup, error) {
	if namespace != crdNamespace {
		return nil, fmt.Errorf("istio: InstallCRDs only supports namespace %q, got %q", crdNamespace, namespace)
	}

	return yamlv2.NewConfigGroup(ctx, name, &yamlv2.ConfigGroupArgs{
		Yaml: pulumi.String(crdManifest),
	}, opts...)
}
