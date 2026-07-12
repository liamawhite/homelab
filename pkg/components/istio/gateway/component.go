// Package gateway provides a Pulumi component for the single shared Istio
// ingress Gateway (Kubernetes Gateway API), open to HTTPRoutes from any
// namespace.
package gateway

import (
	"fmt"

	gatewayv1 "github.com/liamawhite/homelab/pkg/crds/gatewayapi/crds/kubernetes/gateway/v1"
	securityv1 "github.com/liamawhite/homelab/pkg/crds/istio/crds/kubernetes/security/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// gatewayName is the shared Gateway object's own name - a stable
	// constant since there's only ever one.
	gatewayName = "shared-gateway"

	// serviceName is the ingress Service Istio's Gateway API auto-deployment
	// provisions for the Gateway above - "<gateway name>-<gatewayclass name>".
	serviceName = gatewayName + "-" + istioGatewayClassName

	// istioGatewayClassName is the GatewayClass istiod auto-creates once it
	// observes the Gateway API CRDs (controllerName istio.io/gateway-controller).
	// This component references it by name rather than creating its own
	// GatewayClass object - owning one too would mean two systems (istiod's
	// controller and Pulumi) both believing they own one physical object,
	// the same conflict class this repo hit for istio-system before
	// namespace creation was centralized in pkg/deploy/namespaces.go.
	istioGatewayClassName = "istio"
)

// Gateway represents the single shared Istio ingress Gateway.
type Gateway struct {
	pulumi.ResourceState

	Name      pulumi.StringOutput
	Namespace pulumi.StringOutput

	// ServiceName/ServiceNamespace are the ingress Service Istio's Gateway
	// API auto-deployment provisions for this Gateway object. Verified
	// against a live cluster (kubectl get svc -n istio-system): Istio names
	// it "<gateway name>-<gatewayclass name>", not just the Gateway's own
	// name.
	ServiceName      pulumi.StringOutput
	ServiceNamespace pulumi.StringOutput
}

// GatewayArgs contains the configuration for the shared Gateway.
type GatewayArgs struct {
	// Namespace is istio-system's name, created centrally by
	// pkg/deploy/namespaces.go and passed in here - this component does not
	// create it.
	Namespace pulumi.StringInput
	// Domain is used to build the wildcard hostname for the HTTP listener
	// (*.Domain), e.g. infraCfg.Cloudflare.Tunnel.Domain.
	Domain pulumi.StringInput

	// CloudflareTeamDomain is the Zero Trust team domain (the <team-name>
	// in https://<team-name>.cloudflareaccess.com), used as the JWT
	// issuer/JWKS source for validating Access-issued tokens.
	CloudflareTeamDomain pulumi.StringInput
	// CloudflareAccessAUD is the Access application's audience tag
	// (pkg/components/cloudflare/auth.Access.AUD), checked as the JWT's
	// aud claim.
	CloudflareAccessAUD pulumi.StringInput
}

// NewGateway creates the shared Istio ingress Gateway.
func NewGateway(ctx *pulumi.Context, name string, args *GatewayArgs, opts ...pulumi.ResourceOption) (*Gateway, error) {
	gw := &Gateway{}

	err := ctx.RegisterComponentResource("homelab:istio:gateway", name, gw, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(gw))

	// 1. Create the shared Gateway, with an HTTP listener open to
	// HTTPRoutes from any namespace - the whole point of a "shared"
	// gateway is that future app deployments, in their own namespaces,
	// attach routes to it. No TLS: Cloudflare Tunnel terminates TLS at the
	// edge and connects to origin over plain HTTP.
	_, err = gatewayv1.NewGateway(ctx, name, &gatewayv1.GatewayArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(gatewayName),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &gatewayv1.GatewaySpecArgs{
			GatewayClassName: pulumi.String(istioGatewayClassName),
			Listeners: gatewayv1.GatewaySpecListenersArray{
				&gatewayv1.GatewaySpecListenersArgs{
					Name:     pulumi.String("http"),
					Port:     pulumi.Int(80),
					Protocol: pulumi.String("HTTP"),
					Hostname: pulumi.Sprintf("*.%s", args.Domain),
					AllowedRoutes: &gatewayv1.GatewaySpecListenersAllowedRoutesArgs{
						Namespaces: &gatewayv1.GatewaySpecListenersAllowedRoutesNamespacesArgs{
							From: pulumi.String("All"),
						},
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	gatewayMatchLabels := pulumi.StringMap{
		"gateway.networking.k8s.io/gateway-name": pulumi.String(gatewayName),
	}
	issuer := pulumi.Sprintf("https://%s.cloudflareaccess.com", args.CloudflareTeamDomain)
	jwksURI := pulumi.Sprintf("https://%s.cloudflareaccess.com/cdn-cgi/access/certs", args.CloudflareTeamDomain)

	// 2. Validate JWTs Cloudflare Access issues after a successful login,
	// at the gateway's own listener - defense in depth behind Access
	// itself, so a request only reaches anything behind the gateway if it
	// carries a token Cloudflare actually signed for this application
	// (right issuer, right audience). Access presents the token via the
	// Cf-Access-Jwt-Assertion header (service/API clients) or the
	// CF_Authorization cookie (browser sessions after the login redirect).
	_, err = securityv1.NewRequestAuthentication(ctx, fmt.Sprintf("%s-jwt", name), &securityv1.RequestAuthenticationArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("cloudflare-access-jwt"),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &securityv1.RequestAuthenticationSpecArgs{
			Selector: &securityv1.RequestAuthenticationSpecSelectorArgs{
				MatchLabels: gatewayMatchLabels,
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

	// 3. Require the JWT validated above, AND explicitly check its issuer
	// and audience claims match Cloudflare/this Access application - not
	// just "some RequestAuthentication validated it" (requestPrincipals:
	// ["*"]) but the specific claims we expect. request.auth.claims[iss]
	// and [aud] are only populated once RequestAuthentication has already
	// verified the token's signature, so this also implies "has a valid
	// token" without needing requestPrincipals separately. Without this
	// policy, pkg/deploy/meshsecurity.go's mesh-wide default-deny policy
	// blocks the gateway entirely (verified live: 403 "RBAC: access
	// denied"); without the claim checks, a valid JWT for any
	// issuer/audience the RequestAuthentication accepts would be enough,
	// rather than specifically this Cloudflare Access app's.
	_, err = securityv1.NewAuthorizationPolicy(ctx, fmt.Sprintf("%s-allow", name), &securityv1.AuthorizationPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("shared-gateway-allow"),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &securityv1.AuthorizationPolicySpecArgs{
			Action: pulumi.String("ALLOW"),
			Selector: &securityv1.AuthorizationPolicySpecSelectorArgs{
				MatchLabels: gatewayMatchLabels,
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

	gw.Name = pulumi.String(gatewayName).ToStringOutput()
	gw.Namespace = args.Namespace.ToStringOutput()
	gw.ServiceName = pulumi.String(serviceName).ToStringOutput()
	gw.ServiceNamespace = args.Namespace.ToStringOutput()

	// Register outputs
	if err := ctx.RegisterResourceOutputs(gw, pulumi.Map{
		"name":             gw.Name,
		"namespace":        gw.Namespace,
		"serviceName":      gw.ServiceName,
		"serviceNamespace": gw.ServiceNamespace,
	}); err != nil {
		return nil, err
	}

	return gw, nil
}
