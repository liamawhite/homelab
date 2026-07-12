// Package domain provides a Pulumi component for a single public DNS
// record, pointing a hostname at the Cloudflare Tunnel that carries traffic
// for it to the shared Gateway. Callers register one Domain per hostname
// they want publicly reachable - see pkg/deploy/domains.go, which is the
// sole place these get created (mirrors the centralized-namespace-creation
// convention in pkg/deploy/namespaces.go: components take their inputs as
// args rather than deciding independently what DNS to publish).
package domain

import (
	"fmt"

	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Domain represents a single CNAME DNS record.
type Domain struct {
	pulumi.ResourceState

	Hostname pulumi.StringOutput
}

// DomainArgs contains the configuration for a Domain.
type DomainArgs struct {
	// ZoneID is the Cloudflare zone this record belongs to.
	ZoneID pulumi.StringInput
	// Hostname is the full hostname to register, e.g. "homelab.example.com".
	Hostname pulumi.StringInput
	// Target is the CNAME target - typically the Cloudflare Tunnel's
	// assigned CNAME (pkg/components/cloudflare/tunnel.Tunnel.TunnelCNAME).
	Target pulumi.StringInput
	// Provider is the Cloudflare provider to create the record with.
	Provider *cloudflare.Provider
}

// NewDomain creates a single proxied CNAME record for Hostname, pointing at
// Target.
func NewDomain(ctx *pulumi.Context, name string, args *DomainArgs, opts ...pulumi.ResourceOption) (*Domain, error) {
	dom := &Domain{}

	err := ctx.RegisterComponentResource("homelab:cloudflare:domain", name, dom, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(append(opts, pulumi.Parent(dom)), pulumi.Provider(args.Provider))

	record, err := cloudflare.NewRecord(ctx, fmt.Sprintf("%s-record", name), &cloudflare.RecordArgs{
		ZoneId:  args.ZoneID,
		Name:    args.Hostname,
		Type:    pulumi.String("CNAME"),
		Content: args.Target,
		Proxied: pulumi.Bool(true),
		Comment: pulumi.String("Managed by Pulumi - Cloudflare Tunnel DNS"),
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	dom.Hostname = record.Name

	// Register outputs
	if err := ctx.RegisterResourceOutputs(dom, pulumi.Map{
		"hostname": dom.Hostname,
	}); err != nil {
		return nil, err
	}

	return dom, nil
}
