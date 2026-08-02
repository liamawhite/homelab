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
	"fmt"

	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
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
			//
			// confPath/binPath: without these, Cilium's chart defaults to
			// the upstream-standard /etc/cni/net.d and /opt/cni/bin - but
			// k3s uses its own non-standard paths
			// (/var/lib/rancher/k3s/agent/etc/cni/net.d and
			// /var/lib/rancher/k3s/data/cni), which is exactly what
			// istio-cni's own DaemonSet is already correctly configured
			// with. Cilium was silently writing its 05-cilium.conflist to
			// the wrong directory on every node in this cluster - masked
			// on the Pi nodes only because a stale 10-flannel.conflist
			// (left over from before this cluster migrated off Flannel via
			// --flannel-backend=none) happened to already exist in the
			// correct k3s-specific directory, giving istio-cni *something*
			// to chain onto even though it was never Cilium's own config.
			// A genuinely fresh node with no Flannel history (the first
			// being nix-gpu-3070-1) has nothing there at all, so istio-cni
			// hangs forever waiting for a CNI conf file that never arrives
			// - see os/README.md / CLAUDE.md for that node's history.
			"cni": pulumi.Map{
				"exclusive": pulumi.Bool(false),
				"confPath":  pulumi.String("/var/lib/rancher/k3s/agent/etc/cni/net.d"),
				"binPath":   pulumi.String("/var/lib/rancher/k3s/data/cni"),
			},
			"socketLB": pulumi.Map{
				"hostNamespaceOnly": pulumi.Bool(true),
			},
			// Without these, Cilium falls back to auto-detecting the API
			// server address from the KUBERNETES_SERVICE_HOST/_PORT env vars
			// kubelet injects into every pod - which point at the Service
			// ClusterIP (10.43.0.1), not a real routable address. That's a
			// cold-start deadlock with kubeProxyReplacement: Cilium is the
			// only thing that makes ClusterIPs routable, so a brand-new
			// node's cilium-agent can never reach the apiserver to finish
			// bootstrapping. Existing nodes don't hit this because their
			// kernel-level Cilium datapath, programmed before this
			// migration, persists across agent restarts - but it left this
			// latent until the first genuinely new node join surfaced it.
			// Point at the VIP (kube-vip, pkg/components/kubevip) rather
			// than any single node so this survives that node's loss.
			"k8sServiceHost": pulumi.String("192.168.1.50"),
			"k8sServicePort": pulumi.String("6443"),
			// K3s embeds its own kube-proxy and keeps running it regardless
			// of CNI choice unless told otherwise - live nftables evidence
			// (non-zero KUBE-SERVICES/KUBE-PROXY-FIREWALL/KUBE-PROXY-CANARY
			// counters) showed both it and Cilium's kubeProxyReplacement
			// programming Service routing at the same time. pkg/k3s's install
			// command now passes --disable-kube-proxy so Cilium is the only
			// thing doing this. socketLB.hostNamespaceOnly + cni.exclusive:
			// false above are exactly the two settings Cilium's own Istio
			// integration docs call out as required to keep this compatible
			// with istio-cni's traffic interception.
			"kubeProxyReplacement": pulumi.Bool(true),
			// Pinned explicitly, even though this cluster already reports
			// "Host: Legacy" in cilium-dbg status without it - confirmed via
			// live testing that neither VXLAN tunnel mode nor native routing
			// mode ever actually enables eBPF host routing here (likely a
			// Raspberry Pi kernel/eBPF feature gap, not something either
			// routing mode controls). Setting this explicitly guards against
			// silently drifting onto eBPF host routing on a future kernel or
			// Cilium upgrade, which would bypass istio-cni's iptables-based
			// kubelet-probe SNAT exclusion rule the same way it does upstream
			// - see issue #6. Doesn't fix that issue's actual root cause
			// (confirmed live, no effect either way today), it just keeps us
			// pinned to the one behavior we've verified.
			"bpf": pulumi.Map{
				"hostLegacyRouting": pulumi.Bool(true),
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// Mesh-wide network policy baseline - every cluster this component sets
	// up as the CNI comes with this, same reasoning as
	// pkg/components/istio bundling its own AuthorizationPolicy/
	// PeerAuthentication baseline rather than leaving callers to remember
	// to wire security up separately.
	//
	// default-deny: every pod-networked endpoint denied on both ingress
	// and egress by default, EXCEPT ingress from the kubelet on the pod's
	// own node - without that exception, every pod's liveness/readiness
	// probes break the moment ingress defaults to deny, since those
	// originate from the node itself (Cilium's "host" entity), not from
	// another pod. No egress exceptions at all yet, including DNS - this
	// is a deliberate clean-slate baseline; per-workload/per-namespace
	// egress allow policies (DNS, istiod, kube-apiserver, cloudflared,
	// etc.) still need to be designed and added on top of this.
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-default-deny", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("default-deny"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{},
			EnableDefaultDeny: &ciliumv2.CiliumClusterwideNetworkPolicySpecEnableDefaultDenyArgs{
				Ingress: pulumi.Bool(true),
				Egress:  pulumi.Bool(true),
			},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEntities: pulumi.StringArray{pulumi.String("host"), pulumi.String("remote-node")},
				},
			},
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
