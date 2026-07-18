// Package apiserver provides the CiliumClusterwideNetworkPolicy resource
// that lets workloads reach the Kubernetes API server under
// pkg/components/cilium's default-deny baseline - split out into its own
// component the same way pkg/components/dns is, since (like CoreDNS) the
// API server isn't a Cilium-managed endpoint owned by any other component
// in this repo.
package apiserver

import (
	"fmt"

	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ClusterAPIServer represents the cluster's Kubernetes API server network
// policy baseline.
type ClusterAPIServer struct {
	pulumi.ResourceState
}

// NewClusterAPIServer creates the CiliumClusterwideNetworkPolicy resource
// that lets pods carrying AccessLabelKey/AccessLabelValue reach the
// Kubernetes API server once egress default-deny is in effect cluster-wide.
// Callers must pass pulumi.DependsOn on the Cilium installation (see
// pkg/components/cilium.NewCilium) so the CiliumClusterwideNetworkPolicy
// CRD Cilium's Helm chart installs already exists before this is applied.
func NewClusterAPIServer(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*ClusterAPIServer, error) {
	c := &ClusterAPIServer{}

	err := ctx.RegisterComponentResource("homelab:kubernetes:cluster-apiserver", name, c, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(c))

	// Via Cilium's built-in "kube-apiserver" entity - resolves to the real
	// API server endpoint(s) regardless of which nodes are currently
	// control-plane members, unlike a hand-maintained IPBlock of node IPs.
	// No corresponding ingress policy - the API server isn't a
	// Cilium-managed endpoint.
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-kube-apiserver", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-kube-apiserver"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					AccessLabelKey: pulumi.String(AccessLabelValue),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("kube-apiserver")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("6443"), Protocol: pulumi.String("TCP")},
							},
						},
					},
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
