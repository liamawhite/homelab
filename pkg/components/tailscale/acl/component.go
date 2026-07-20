// Package acl provides a Pulumi component for the tailnet's ACL policy -
// the tailnet-wide access control and tagOwners rules the Tailscale
// Kubernetes Operator (pkg/components/tailscale) needs to register itself
// and its proxies under tag:k8s-operator/tag:k8s. Ported from the legacy
// _migrateme/project/vpn.ts, which managed this by hand via the
// @pulumi/tailscale provider.
package acl

import (
	"encoding/json"
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/tailscale"
	tstailscale "github.com/pulumi/pulumi-tailscale/sdk/go/tailscale"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Acl represents the tailnet's ACL policy.
type Acl struct {
	pulumi.ResourceState
}

// AclArgs contains the configuration for Acl.
type AclArgs struct {
	// Provider is the Tailscale admin-API provider
	// (pkg/deploy.Providers.Tailscale) - built from a dedicated
	// policy_file-scoped OAuth credential, not the operator's own (see
	// pkg/deploy/providers.go's NewTailscaleProvider for why these are
	// kept separate).
	Provider *tstailscale.Provider
}

// policy is the tailnet ACL: accept-all plus tagOwners for the two tags
// pkg/components/tailscale's operator and its proxies use (see
// tailscale.OperatorTag/ProxyTag) - ported verbatim from
// _migrateme/project/vpn.ts.
type policy struct {
	ACLs      []aclRule           `json:"acls"`
	TagOwners map[string][]string `json:"tagOwners"`
}

type aclRule struct {
	Action string   `json:"action"`
	Src    []string `json:"src"`
	Dst    []string `json:"dst"`
}

// NewAcl declares the tailnet's ACL policy.
func NewAcl(ctx *pulumi.Context, name string, args *AclArgs, opts ...pulumi.ResourceOption) (*Acl, error) {
	a := &Acl{}

	err := ctx.RegisterComponentResource("homelab:tailscale:acl", name, a, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(append(opts, pulumi.Parent(a)), pulumi.Provider(args.Provider))

	body, err := json.Marshal(policy{
		ACLs: []aclRule{
			{Action: "accept", Src: []string{"*"}, Dst: []string{"*:*"}},
		},
		TagOwners: map[string][]string{
			tailscale.OperatorTag: {},
			tailscale.ProxyTag:    {tailscale.OperatorTag},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling tailnet ACL policy: %w", err)
	}

	// overwriteExistingContent: true - the tailnet already has some policy
	// live today, never previously managed by Pulumi, and this makes the
	// policy declared here authoritative immediately rather than requiring
	// a `pulumi import` first (this repo's CLI has no import command
	// today - see cli/cmd/pulumi). Deliberate, not a default left
	// unconsidered.
	_, err = tstailscale.NewAcl(ctx, fmt.Sprintf("%s-policy", name), &tstailscale.AclArgs{
		Acl:                      pulumi.String(string(body)),
		OverwriteExistingContent: pulumi.Bool(true),
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	if err := ctx.RegisterResourceOutputs(a, pulumi.Map{}); err != nil {
		return nil, err
	}

	return a, nil
}
