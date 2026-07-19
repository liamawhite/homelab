package deploy

import (
	"fmt"

	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// lookupZoneID resolves the Cloudflare zone ID for baseDomain once, so every
// caller that needs to create DNS/Ruleset resources in this zone
// (pkg/components/tailscale/ingress.NewIngress and createTailscaleRedirects)
// shares one lookup instead of each issuing its own. createDomains has its
// own separate, pre-existing lookup - left as is, this is a new, additive
// helper for the new Tailscale-related consumers, not a refactor of it.
func lookupZoneID(ctx *pulumi.Context, baseDomain, accountID pulumi.StringInput, provider *cloudflare.Provider) pulumi.StringOutput {
	return pulumi.All(baseDomain, accountID).ApplyT(func(inputs []interface{}) (string, error) {
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
}
