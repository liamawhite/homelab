package deploy

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// createTailscaleRedirects registers every app's Cloudflare-side redirect to
// its real tailnet address, as one Ruleset rule per app. Centralized here
// (not left for each app or pkg/components/tailscale/ingress to create its
// own Ruleset) because Cloudflare's Rulesets Engine allows exactly one
// "phase entry point" ruleset per zone per phase (here,
// http_request_dynamic_redirect - "Single Redirects", not the confusingly
// similarly-named http_request_redirect phase, which is the entirely
// separate account-level "Bulk Redirects" product and needs different
// permissions) - N independent Ruleset resources all targeting the same
// zone+phase would conflict, only one can own that phase's rule list.
// Mirrors domains.go's domainSpec/createDomains shape and the reason
// pkg/components/cloudflare/tunnel.TunnelRoute is centrally collected too.
//
// The API token needs Zone > Single Redirect > Edit for this phase
// specifically (confirmed against
// https://developers.cloudflare.com/rules/url-forwarding/single-redirects/create-api/).
func createTailscaleRedirects(ctx *pulumi.Context, zoneID pulumi.StringInput, baseDomain pulumi.StringInput, provider *cloudflare.Provider, routes []ingress.RedirectRoute, opts ...pulumi.ResourceOption) (*cloudflare.Ruleset, error) {
	rules := make(cloudflare.RulesetRuleArray, 0, len(routes))
	for _, r := range routes {
		rules = append(rules, &cloudflare.RulesetRuleArgs{
			Action:      pulumi.String("redirect"),
			Description: pulumi.String(fmt.Sprintf("%s -> tailnet", r.Subdomain)),
			Expression:  pulumi.Sprintf(`http.host eq "%s.%s"`, r.Subdomain, baseDomain),
			ActionParameters: &cloudflare.RulesetRuleActionParametersArgs{
				FromValue: &cloudflare.RulesetRuleActionParametersFromValueArgs{
					TargetUrl: &cloudflare.RulesetRuleActionParametersFromValueTargetUrlArgs{
						Value: r.Target,
					},
					// 302, not 301: the tailnet target isn't a stable,
					// permanent address (proxy pods are recreated,
					// hostnames can be repointed) - a 301 risks browsers/
					// intermediate caches pinning a target that later
					// changes.
					StatusCode:          pulumi.Int(302),
					PreserveQueryString: pulumi.Bool(true),
				},
			},
		})
	}

	return cloudflare.NewRuleset(ctx, "tailscale-redirects", &cloudflare.RulesetArgs{
		ZoneId: zoneID,
		Kind:   pulumi.String("zone"),
		Phase:  pulumi.String("http_request_dynamic_redirect"),
		Name:   pulumi.String("tailscale-redirects"),
		Rules:  rules,
	}, append(opts, pulumi.Provider(provider))...)
}
