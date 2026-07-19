package deploy

import (
	"fmt"

	cfdomain "github.com/liamawhite/homelab/pkg/components/cloudflare/domain"
	"github.com/liamawhite/homelab/pkg/deploy/applications"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// domainSpec describes one public hostname createDomains should register, as
// a CNAME record pointing at the Cloudflare Tunnel.
type domainSpec struct {
	name      string // logical resource name / lookup key
	subdomain string // hostname prefix, e.g. "homelab"
}

// Domains holds every cfdomain.Domain resource createDomains made, keyed by
// name, so callers can look up the exact resource if they need to depend on
// it directly.
type Domains struct {
	byName map[string]*cfdomain.Domain
}

// Get returns the Domain resource created for name. It panics if name wasn't
// included in createDomains' spec list - every hostname consumed anywhere in
// Program() must be registered there first, so a panic here means a caller
// and this file's spec list have drifted out of sync (a programmer error to
// fix at the call site, not a runtime condition to handle gracefully).
func (d *Domains) Get(name string) *cfdomain.Domain {
	dom, ok := d.byName[name]
	if !ok {
		panic(fmt.Sprintf("deploy: domain %q was never created by createDomains", name))
	}
	return dom
}

// createDomains registers every public hostname any pkg/deploy application
// needs a DNS record for, pointing each at the Cloudflare Tunnel's CNAME
// target. Centralized here (not left for each app or the tunnel itself to
// create its own DNS record) so exactly the hostnames actually routable
// through the tunnel are published - not a single blanket wildcard covering
// the whole domain regardless of whether an app exists for a given name.
// Follows the same convention as pkg/deploy/namespaces.go: add a spec entry
// here when wiring in a new app that needs a public hostname.
func createDomains(ctx *pulumi.Context, baseDomain pulumi.StringInput, tunnelTarget pulumi.StringInput, accountID pulumi.StringInput, provider *cloudflare.Provider, opts ...pulumi.ResourceOption) (*Domains, error) {
	specs := []domainSpec{
		{name: "public", subdomain: applications.PublicSubdomain},
	}

	zoneID := pulumi.All(baseDomain, accountID).ApplyT(func(inputs []interface{}) (string, error) {
		domain := inputs[0].(string)
		account := inputs[1].(string)

		zone, err := cloudflare.LookupZone(ctx, &cloudflare.LookupZoneArgs{
			Name:      pulumi.StringRef(domain),
			AccountId: pulumi.StringRef(account),
		}, pulumi.Provider(provider))
		if err != nil {
			return "", fmt.Errorf("failed to lookup zone for domain %s: %w", domain, err)
		}
		return zone.Id, nil
	}).(pulumi.StringOutput)

	result := &Domains{byName: make(map[string]*cfdomain.Domain, len(specs))}
	for _, spec := range specs {
		dom, err := cfdomain.NewDomain(ctx, spec.name, &cfdomain.DomainArgs{
			ZoneID:   zoneID,
			Hostname: pulumi.Sprintf("%s.%s", spec.subdomain, baseDomain),
			Target:   tunnelTarget,
			Provider: provider,
		}, opts...)
		if err != nil {
			return nil, fmt.Errorf("creating domain %s: %w", spec.name, err)
		}
		result.byName[spec.name] = dom
	}
	return result, nil
}
