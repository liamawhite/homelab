// Package cilium provides a Pulumi component for the Cilium CNI, replacing
// K3s's default Flannel so that Kubernetes NetworkPolicy is actually
// enforced (Flannel implements no NetworkPolicy controller at all - objects
// apply successfully and simply do nothing).
//
// Cilium was chosen over Calico for this cluster specifically because
// Calico's Istio-ambient integration is tech-preview and explicitly
// documented as incompatible with Istio Waypoint proxies, which this
// cluster already uses (see pkg/deploy/applications/home.go). Cilium's
// Istio integration is GA with a documented, known-good Helm recipe.
//
// Deploying this is only half the migration - Flannel has to actually be
// disabled first (see pkg/k3s.DisableFlannel and the "homelab k3s
// disable-flannel" CLI command), and every existing pod has to be recreated
// afterward to pick up the new CNI wiring. This is a coordinated,
// disruptive, all-nodes-together maintenance-window operation, not a
// rolling/zero-downtime change - see this repo's CLAUDE.md and the commit
// history around this package's introduction for the full runbook.
package cilium

import (
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// podCIDR must match K3s's own default pod CIDR exactly - Cilium's IPAM
// takes over pod IP allocation from Flannel, and a mismatch here would
// hand out addresses K3s's own control plane doesn't expect.
const podCIDR = "10.42.0.0/16"

// Cilium represents the cluster-wide Cilium CNI installation.
type Cilium struct {
	pulumi.ResourceState
}

// CiliumArgs contains the configuration for Cilium.
type CiliumArgs struct {
	// Version is the Cilium version to deploy (e.g. versions.Cilium).
	Version string
}

// NewCilium installs Cilium as the cluster's CNI. Cilium installs into the
// pre-existing kube-system namespace (its own convention, matching how
// pkg/components/kubevip treats kube-system) - this component doesn't
// create or own any namespace.
func NewCilium(ctx *pulumi.Context, name string, args *CiliumArgs, opts ...pulumi.ResourceOption) (*Cilium, error) {
	c := &Cilium{}

	err := ctx.RegisterComponentResource("homelab:kubernetes:cilium", name, c, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(c))

	_, err = helmv4.NewChart(ctx, name, &helmv4.ChartArgs{
		Namespace: pulumi.String("kube-system"),
		Chart:     pulumi.String("cilium"),
		Version:   pulumi.String(args.Version),
		RepositoryOpts: &helmv4.RepositoryOptsArgs{
			Repo: pulumi.String("https://helm.cilium.io"),
		},
		Values: pulumi.Map{
			"ipam": pulumi.Map{
				"operator": pulumi.Map{
					"clusterPoolIPv4PodCIDRList": pulumi.StringArray{pulumi.String(podCIDR)},
				},
			},
			// Disables cilium-envoy. Not needed - L7 is already handled by
			// Istio waypoints - and stock Raspberry Pi OS kernels crash it:
			// the Pi kernel defconfig still sets CONFIG_ARM64_VA_BITS_39,
			// but cilium-envoy's embedded tcmalloc assumes 48-bit VA space.
			"envoy": pulumi.Map{
				"enabled": pulumi.Bool(false),
			},
			// Both required for correctly chaining with istio-cni, per
			// Cilium's own Istio integration docs.
			"cni": pulumi.Map{
				"exclusive": pulumi.Bool(false),
			},
			"socketLB": pulumi.Map{
				"hostNamespaceOnly": pulumi.Bool(true),
			},
			// Documented safe default when chaining with another CNI's
			// ambient/service-mesh redirection; not separately verified
			// against this cluster's own kube-proxy setup post-migration.
			"kubeProxyReplacement": pulumi.Bool(false),
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	if err := ctx.RegisterResourceOutputs(c, pulumi.Map{}); err != nil {
		return nil, err
	}

	return c, nil
}
