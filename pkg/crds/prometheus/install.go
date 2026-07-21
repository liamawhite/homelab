package prometheus

import (
	_ "embed"

	yamlv2 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// crdManifest is prometheus-operator's release bundle, filtered down to just
// its CustomResourceDefinition documents (see gen-crds.sh), embedded at
// build time so installing it doesn't depend on that file being present on
// disk at runtime.
//
//go:embed prometheus-operator-crds.yaml
var crdManifest string

// InstallCRDs applies the prometheus-operator CRDs (Prometheus,
// ServiceMonitor, PodMonitor, PrometheusRule, Alertmanager, ...) to the
// cluster. Every CustomResourceDefinition is itself cluster-scoped, and
// nothing else is bundled alongside them (see gen-crds.sh), so unlike
// pkg/crds/istio.InstallCRDs this takes no namespace argument.
func InstallCRDs(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*yamlv2.ConfigGroup, error) {
	return yamlv2.NewConfigGroup(ctx, name, &yamlv2.ConfigGroupArgs{
		Yaml: pulumi.String(crdManifest),
	}, opts...)
}
