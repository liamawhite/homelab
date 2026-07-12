// Package accessjwt provides a Pulumi component that validates Cloudflare
// Access-issued JWTs for a single Service sitting behind an Istio ambient
// waypoint. Waypoints ignore selector-based AuthorizationPolicy/
// RequestAuthentication entirely ("selector policies will be ignored" -
// istio.io) - they require spec.targetRefs, which is the whole reason this
// couldn't just be the old pkg/components/istio/gateway logic copy-pasted
// with a new selector.
package accessjwt

import (
	"fmt"

	securityv1 "github.com/liamawhite/homelab/pkg/crds/istio/crds/kubernetes/security/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// AccessJWT represents JWT validation for a single Service.
type AccessJWT struct {
	pulumi.ResourceState
}

// AccessJWTArgs contains the configuration for AccessJWT.
type AccessJWTArgs struct {
	// Namespace must match ServiceName's namespace - targetRefs requires
	// the policy to live in the same namespace as the Service it targets.
	Namespace pulumi.StringInput
	// ServiceName is the Service to protect - must already be routed
	// through a waypoint (istio.io/use-waypoint label) for this to have any
	// effect; targetRefs-scoped policies are otherwise never evaluated.
	ServiceName pulumi.StringInput
	// CloudflareTeamDomain is the Zero Trust team domain (the <team-name>
	// in https://<team-name>.cloudflareaccess.com), used as the JWT
	// issuer/JWKS source for validating Access-issued tokens.
	CloudflareTeamDomain pulumi.StringInput
	// CloudflareAccessAUD is the Access application's audience tag
	// (pkg/components/cloudflare/auth.Access.AUD), checked as the JWT's
	// aud claim.
	CloudflareAccessAUD pulumi.StringInput
}

// NewAccessJWT creates JWT validation for a single Service: a
// RequestAuthentication that verifies the token's signature, and an
// AuthorizationPolicy that requires one AND explicitly checks its issuer
// and audience claims match Cloudflare/this Access application.
func NewAccessJWT(ctx *pulumi.Context, name string, args *AccessJWTArgs, opts ...pulumi.ResourceOption) (*AccessJWT, error) {
	a := &AccessJWT{}

	err := ctx.RegisterComponentResource("homelab:cloudflare:accessjwt", name, a, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(a))

	serviceTargetRef := &securityv1.AuthorizationPolicySpecTargetRefsArgs{
		Group: pulumi.String(""),
		Kind:  pulumi.String("Service"),
		Name:  args.ServiceName,
	}

	issuer := pulumi.Sprintf("https://%s.cloudflareaccess.com", args.CloudflareTeamDomain)
	jwksURI := pulumi.Sprintf("https://%s.cloudflareaccess.com/cdn-cgi/access/certs", args.CloudflareTeamDomain)

	// 1. Validate JWTs Cloudflare Access issues after a successful login -
	// defense in depth behind Access itself, so a request only reaches the
	// Service if it carries a token Cloudflare actually signed for this
	// application (right issuer, right audience). Access presents the
	// token via the Cf-Access-Jwt-Assertion header (service/API clients) or
	// the CF_Authorization cookie (browser sessions after the login
	// redirect).
	_, err = securityv1.NewRequestAuthentication(ctx, fmt.Sprintf("%s-jwt", name), &securityv1.RequestAuthenticationArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fmt.Sprintf("%s-cloudflare-access-jwt", name)),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &securityv1.RequestAuthenticationSpecArgs{
			TargetRefs: securityv1.RequestAuthenticationSpecTargetRefsArray{
				&securityv1.RequestAuthenticationSpecTargetRefsArgs{
					Group: pulumi.String(""),
					Kind:  pulumi.String("Service"),
					Name:  args.ServiceName,
				},
			},
			JwtRules: securityv1.RequestAuthenticationSpecJwtRulesArray{
				&securityv1.RequestAuthenticationSpecJwtRulesArgs{
					Issuer:    issuer,
					JwksUri:   jwksURI,
					Audiences: pulumi.StringArray{args.CloudflareAccessAUD},
					FromHeaders: securityv1.RequestAuthenticationSpecJwtRulesFromHeadersArray{
						&securityv1.RequestAuthenticationSpecJwtRulesFromHeadersArgs{
							Name: pulumi.String("Cf-Access-Jwt-Assertion"),
						},
					},
					FromCookies: pulumi.StringArray{pulumi.String("CF_Authorization")},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 2. Require the JWT validated above, AND explicitly check its issuer
	// and audience claims match Cloudflare/this Access application - not
	// just "some RequestAuthentication validated it" but the specific
	// claims expected. request.auth.claims[iss]/[aud] are only populated
	// once RequestAuthentication has already verified the token's
	// signature. Without this policy, pkg/components/istio's mesh-wide
	// waypoint-default-deny blocks the Service entirely; without the claim
	// checks, a valid JWT for any issuer/audience the RequestAuthentication
	// accepts would be enough, rather than specifically this Cloudflare
	// Access app's.
	_, err = securityv1.NewAuthorizationPolicy(ctx, fmt.Sprintf("%s-allow", name), &securityv1.AuthorizationPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fmt.Sprintf("%s-allow", name)),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &securityv1.AuthorizationPolicySpecArgs{
			Action: pulumi.String("ALLOW"),
			TargetRefs: securityv1.AuthorizationPolicySpecTargetRefsArray{
				serviceTargetRef,
			},
			Rules: securityv1.AuthorizationPolicySpecRulesArray{
				&securityv1.AuthorizationPolicySpecRulesArgs{
					When: securityv1.AuthorizationPolicySpecRulesWhenArray{
						&securityv1.AuthorizationPolicySpecRulesWhenArgs{
							Key:    pulumi.String("request.auth.claims[iss]"),
							Values: pulumi.StringArray{issuer},
						},
						&securityv1.AuthorizationPolicySpecRulesWhenArgs{
							Key:    pulumi.String("request.auth.claims[aud]"),
							Values: pulumi.StringArray{args.CloudflareAccessAUD},
						},
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	if err := ctx.RegisterResourceOutputs(a, pulumi.Map{}); err != nil {
		return nil, err
	}

	return a, nil
}
