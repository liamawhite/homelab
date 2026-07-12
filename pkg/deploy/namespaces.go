package deploy

import (
	"fmt"

	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Namespace names used across pkg/deploy components. Centralized here (not
// left as private consts inside each component package) so every component
// that needs a namespace references the same literal, and so this file is
// the one place that creates the corev1.Namespace object for each - no
// component creates its own namespace anymore, avoiding the
// two-Pulumi-resources-own-one-physical-namespace conflict class of bug
// this repo hit for istio-system before this file existed.
const (
	IstioSystemNamespace    = "istio-system"
	LonghornSystemNamespace = "longhorn-system"
	CloudflareNamespace     = "cloudflare"
)

// namespaceSpec describes one namespace createNamespaces should create.
type namespaceSpec struct {
	name    string
	labels  pulumi.StringMap
	aliases []pulumi.Alias
}

// Namespaces holds every corev1.Namespace resource createNamespaces made,
// keyed by name, so callers can look up the exact resource to pass into
// pulumi.DependsOn(...) and to read Metadata.Name() from for a component's
// Namespace arg.
type Namespaces struct {
	byName map[string]*corev1.Namespace
}

// Get returns the Namespace resource created for name. It panics if name
// wasn't included in createNamespaces' spec list - every namespace consumed
// anywhere in Program() must be registered there first, so a panic here
// means a caller and this file's spec list have drifted out of sync (a
// programmer error to fix at the call site, not a runtime condition to
// handle gracefully).
func (n *Namespaces) Get(name string) *corev1.Namespace {
	ns, ok := n.byName[name]
	if !ok {
		panic(fmt.Sprintf("deploy: namespace %q was never created by createNamespaces", name))
	}
	return ns
}

// createNamespaces creates every Kubernetes namespace any pkg/deploy
// component needs, up front, as the sole owner of each. Only namespaces
// actually consumed by Program()'s current call graph are listed here today
// (istio-system). LonghornSystemNamespace and CloudflareNamespace are
// defined above and documented here so that when pkg/components/longhorn
// and pkg/components/cloudflare/tunnel are eventually wired into Program(),
// their spec entries below are simply un-commented rather than invented
// fresh. Un-commenting must happen together with wiring in that component -
// otherwise its Namespace arg references a namespace nothing created.
func createNamespaces(ctx *pulumi.Context, opts ...pulumi.ResourceOption) (*Namespaces, error) {
	specs := []namespaceSpec{
		// TEMPORARY: adopting the physical istio-system namespace previously
		// owned by the "istio-crds-namespace" logical resource (created by
		// the old pkg/crds/istio.InstallCRDs before namespace creation was
		// centralized here) - without this alias, Pulumi would delete that
		// logical resource and create this one fresh, which for a Namespace
		// means a real delete+recreate of the physical object (cascading
		// deletes to everything inside it) rather than a safe rename. Remove
		// this alias once applied successfully once.
		{name: IstioSystemNamespace, labels: pulumi.StringMap{
			"istio.io/dataplane-mode": pulumi.String("ambient"),
		}, aliases: []pulumi.Alias{
			{Name: pulumi.String("istio-crds-namespace")},
		}},
		// Uncomment when pkg/components/longhorn is wired into Program():
		// {name: LonghornSystemNamespace, labels: pulumi.StringMap{
		// 	"istio.io/dataplane-mode": pulumi.String("ambient"),
		// }},
		{name: CloudflareNamespace, labels: pulumi.StringMap{
			"istio.io/dataplane-mode": pulumi.String("ambient"),
		}},
	}

	result := &Namespaces{byName: make(map[string]*corev1.Namespace, len(specs))}
	for _, spec := range specs {
		specOpts := opts
		if len(spec.aliases) > 0 {
			specOpts = append(append([]pulumi.ResourceOption{}, opts...), pulumi.Aliases(spec.aliases))
		}

		ns, err := corev1.NewNamespace(ctx, spec.name, &corev1.NamespaceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:   pulumi.String(spec.name),
				Labels: spec.labels,
			},
		}, specOpts...)
		if err != nil {
			return nil, fmt.Errorf("creating namespace %s: %w", spec.name, err)
		}
		result.byName[spec.name] = ns
	}
	return result, nil
}
