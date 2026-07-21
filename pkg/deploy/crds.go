package deploy

import (
	"github.com/liamawhite/homelab/pkg/crds/gatewayapi"
	"github.com/liamawhite/homelab/pkg/crds/istio"
	"github.com/liamawhite/homelab/pkg/crds/lights"
	"github.com/liamawhite/homelab/pkg/crds/prometheus"
	yamlv2 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// CRDs holds the resources installCRDs created, so callers can build
// precise pulumi.DependsOn options against them.
type CRDs struct {
	GatewayAPI *yamlv2.ConfigGroup
	Istio      *yamlv2.ConfigGroup
	Lights     *yamlv2.ConfigGroup
	Prometheus *yamlv2.ConfigGroup
}

// installCRDs installs the cluster-scoped CRDs later components depend on.
// istioSystemNamespace must be the already-created istio-system namespace
// name (see pkg/deploy/namespaces.go); callers must pass
// pulumi.DependsOn(istioSystemNS) in opts so istio.InstallCRDs' manifest
// (which includes a resource namespaced to istio-system) isn't raced
// against namespace creation.
func installCRDs(ctx *pulumi.Context, istioSystemNamespace string, opts ...pulumi.ResourceOption) (*CRDs, error) {
	gwAPI, err := gatewayapi.InstallCRDs(ctx, "gateway-api-crds", opts...)
	if err != nil {
		return nil, err
	}
	istioCRDs, err := istio.InstallCRDs(ctx, "istio-crds", istioSystemNamespace, opts...)
	if err != nil {
		return nil, err
	}
	lightsCRD, err := lights.InstallCRDs(ctx, "lights-crd", opts...)
	if err != nil {
		return nil, err
	}
	promCRDs, err := prometheus.InstallCRDs(ctx, "prometheus-crds", opts...)
	if err != nil {
		return nil, err
	}
	return &CRDs{GatewayAPI: gwAPI, Istio: istioCRDs, Lights: lightsCRD, Prometheus: promCRDs}, nil
}
