// Package auth provides a Pulumi component for gating traffic behind
// Cloudflare Access (Zero Trust) - login enforced at Cloudflare's edge,
// before any request reaches the tunnel.
package auth

import (
	"fmt"

	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Access represents a Cloudflare Access application protecting a hostname,
// with an allow policy for a fixed set of emails.
type Access struct {
	pulumi.ResourceState

	ApplicationID pulumi.StringOutput
	// AUD is the Access application's audience tag, used to validate the
	// JWT's audience claim wherever Access-issued tokens get re-verified
	// (e.g. the shared Gateway's RequestAuthentication).
	AUD pulumi.StringOutput
	// TeamDomain echoes AccessArgs.TeamDomain - exposed so callers building
	// an accessjwt.AccessJWTArgs from this Access don't need their own
	// separate reference to the raw config value.
	TeamDomain pulumi.StringOutput
	// AllowedEmails echoes AccessArgs.AllowedEmails, for the same reason.
	AllowedEmails pulumi.StringArrayOutput
}

// AccessArgs contains the configuration for an Access application.
type AccessArgs struct {
	// AccountID is the Cloudflare account the application belongs to.
	AccountID pulumi.StringInput
	// Domain is the hostname (or wildcard, e.g. "*.example.com") Access
	// protects.
	Domain pulumi.StringInput
	// AllowedEmails are the only emails permitted to authenticate.
	AllowedEmails []string
	// SessionDuration is how often a user is forced to re-authenticate,
	// e.g. "24h" or "2h45m".
	SessionDuration pulumi.StringInput
	// TeamDomain is the Zero Trust team domain (the <team-name> in
	// https://<team-name>.cloudflareaccess.com) - not used to create the
	// Access application itself, only echoed back on Access so callers
	// validating Access-issued JWTs elsewhere (e.g.
	// pkg/components/cloudflare/accessjwt) have a single object to depend
	// on instead of threading this value through separately.
	TeamDomain string
}

// NewAccess creates a new Cloudflare Access application and an allow policy
// restricting it to AllowedEmails.
func NewAccess(ctx *pulumi.Context, name string, args *AccessArgs, opts ...pulumi.ResourceOption) (*Access, error) {
	access := &Access{}

	err := ctx.RegisterComponentResource("homelab:cloudflare:access", name, access, opts...)
	if err != nil {
		return nil, err
	}

	resourceOpts := append(opts, pulumi.Parent(access))

	// 1. Create the Access application
	app, err := cloudflare.NewZeroTrustAccessApplication(ctx, fmt.Sprintf("%s-app", name), &cloudflare.ZeroTrustAccessApplicationArgs{
		AccountId:       args.AccountID,
		Name:            pulumi.String(name),
		Domain:          args.Domain,
		SessionDuration: args.SessionDuration,
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}
	access.ApplicationID = app.ID().ToStringOutput()
	access.AUD = app.Aud
	access.TeamDomain = pulumi.String(args.TeamDomain).ToStringOutput()
	access.AllowedEmails = pulumi.ToStringArray(args.AllowedEmails).ToStringArrayOutput()

	// 2. Create the allow policy restricting the application to AllowedEmails
	_, err = cloudflare.NewZeroTrustAccessPolicy(ctx, fmt.Sprintf("%s-policy", name), &cloudflare.ZeroTrustAccessPolicyArgs{
		AccountId:     args.AccountID,
		ApplicationId: app.ID(),
		Name:          pulumi.String(fmt.Sprintf("%s-allow", name)),
		Decision:      pulumi.String("allow"),
		Precedence:    pulumi.Int(1),
		Includes: cloudflare.ZeroTrustAccessPolicyIncludeArray{
			&cloudflare.ZeroTrustAccessPolicyIncludeArgs{
				Emails: pulumi.ToStringArray(args.AllowedEmails),
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// Register outputs
	if err := ctx.RegisterResourceOutputs(access, pulumi.Map{
		"applicationId": access.ApplicationID,
		"aud":           access.AUD,
		"teamDomain":    access.TeamDomain,
		"allowedEmails": access.AllowedEmails,
	}); err != nil {
		return nil, err
	}

	return access, nil
}
