package deploy

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/istio"
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
	TailscaleNamespace      = "tailscale"
	HealthNamespace         = "health"
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
// component needs, up front, as the sole owner of each.
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
			istio.DataplaneModeLabelKey: pulumi.String(istio.DataplaneModeAmbient),
		}, aliases: []pulumi.Alias{
			{Name: pulumi.String("istio-crds-namespace")},
		}},
		{name: LonghornSystemNamespace, labels: pulumi.StringMap{
			istio.DataplaneModeLabelKey: pulumi.String(istio.DataplaneModeAmbient),
		}},
		{name: CloudflareNamespace, labels: pulumi.StringMap{
			istio.DataplaneModeLabelKey: pulumi.String(istio.DataplaneModeAmbient),
		}},
		// The tailscale-operator's own namespace - its dynamically created
		// per-Ingress proxy pods land here too, and need ztunnel capture to
		// reach an app's waypoint (see pkg/components/tailscale).
		{name: TailscaleNamespace, labels: pulumi.StringMap{
			istio.DataplaneModeLabelKey: pulumi.String(istio.DataplaneModeAmbient),
		}},
		// Shared by both "public" and "private" (pkg/deploy/applications) -
		// a deliberate one-namespace-for-two-apps exception to the rest of
		// this file's per-app convention, since they're explicitly the same
		// health-check demo split by exposure mechanism, not unrelated
		// apps. Ambient-enrolled like every other namespace here. Was
		// briefly given its own liveness probe (unlike the "default"
		// namespace it used to live in) to test whether any
		// ambient-enrolled workload with an HTTP probe hits the same
		// kubelet-probe-vs-ztunnel conflict cloudflared did - confirmed it
		// does, see pkg/deploy/applications/public.go and issue #6.
		{name: HealthNamespace, labels: pulumi.StringMap{
			istio.DataplaneModeLabelKey: pulumi.String(istio.DataplaneModeAmbient),
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
