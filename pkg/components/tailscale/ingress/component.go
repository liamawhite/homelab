// Package ingress provides a Pulumi component that puts a single Service on
// Tailscale: a k8s Ingress the tailscale-operator (pkg/components/tailscale)
// reconciles into a dynamically created proxy pod, the Istio
// AuthorizationPolicy bypass that pod's traffic needs to actually reach the
// Service through its waypoint, and the Cloudflare-side bookkeeping for a
// memorable, Access-gated redirect entry point.
package ingress

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/tailscale"
	securityv1 "github.com/liamawhite/homelab/pkg/crds/istio/crds/kubernetes/security/v1"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	networkingv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/networking/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Ingress represents a single Service's Tailscale ingress path.
type Ingress struct {
	pulumi.ResourceState

	Hostname pulumi.StringOutput
	Redirect RedirectRoute
}

// RedirectRoute is this app's Cloudflare-redirect data - a hostname on the
// Cloudflare zone that should redirect to this app's tailnet address.
// Cloudflare's Rulesets API only allows one "phase entry point" ruleset per
// zone+phase - there's no way to register a redirect rule independently, so
// callers can't apply this directly; each app instead hands back this data
// for pkg/deploy's central collection point (pkg/deploy/redirects.go),
// mirroring exactly why pkg/components/cloudflare/tunnel.TunnelRoute exists.
type RedirectRoute struct {
	// Subdomain is the Cloudflare-side hostname prefix - the same label as
	// the Tailscale hostname, since it lives in a completely separate DNS
	// zone (Cloudflare's authoritative DNS vs Tailscale's own MagicDNS), so
	// there's no collision to avoid by picking a different name.
	Subdomain string
	// Target is the full https:// tailnet URL to redirect to.
	Target pulumi.StringInput
}

// IngressArgs contains the configuration for Ingress.
type IngressArgs struct {
	// Namespace must match ServiceName's namespace - the Ingress and the
	// AuthorizationPolicy bypass both live here.
	Namespace pulumi.StringInput
	// ServiceName is the backend Service - must already be routed through a
	// waypoint (istio.io/use-waypoint label) for the AuthorizationPolicy
	// bypass to have any effect.
	ServiceName pulumi.StringInput
	// ServicePort is the backend Service's port.
	ServicePort int
	// Hostname is the tailscale-side hostname prefix, e.g. "private" ->
	// reachable at private.<MagicDNSSuffix>. This is set via
	// spec.tls[0].hosts[0], not the tailscale.com/hostname annotation - that
	// annotation is only honored on Service-typed Tailscale egress objects,
	// not Ingress.
	Hostname string
	// OperatorNamespace is where pkg/components/tailscale's operator (and
	// its dynamically created per-Ingress proxy pods) run - used to build
	// the AuthorizationPolicy bypass's source-principal check below.
	OperatorNamespace pulumi.StringInput

	// MagicDNSSuffix is your tailnet's real MagicDNS suffix
	// (infraCfg.Tailscale.MagicDNSSuffix) - used only to build the redirect
	// target URL, never itself exposed as a Cloudflare hostname.
	MagicDNSSuffix pulumi.StringInput
	// CloudflareZoneID is the Cloudflare zone this app's redirect DNS
	// record belongs to - precomputed once in pkg/deploy and shared across
	// every caller (see pkg/deploy/zone.go).
	CloudflareZoneID pulumi.StringInput
	// CloudflareBaseDomain is infraCfg.Cloudflare.Tunnel.Domain.
	CloudflareBaseDomain pulumi.StringInput
	// CloudflareProvider is the Cloudflare provider to create the redirect
	// DNS record with.
	CloudflareProvider *cloudflare.Provider
}

// NewIngress puts a single Service on Tailscale.
func NewIngress(ctx *pulumi.Context, name string, args *IngressArgs, opts ...pulumi.ResourceOption) (*Ingress, error) {
	ing := &Ingress{}

	err := ctx.RegisterComponentResource("homelab:tailscale:ingress", name, ing, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(ing))

	// 1. The k8s Ingress the tailscale-operator reconciles into a
	// dynamically created proxy pod that joins the tailnet.
	_, err = networkingv1.NewIngress(ctx, fmt.Sprintf("%s-tailscale-ingress", name), &networkingv1.IngressArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(args.Hostname),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &networkingv1.IngressSpecArgs{
			IngressClassName: pulumi.String(tailscale.IngressClassName),
			Tls: networkingv1.IngressTLSArray{
				&networkingv1.IngressTLSArgs{Hosts: pulumi.StringArray{pulumi.String(args.Hostname)}},
			},
			Rules: networkingv1.IngressRuleArray{
				&networkingv1.IngressRuleArgs{
					Http: &networkingv1.HTTPIngressRuleValueArgs{
						Paths: networkingv1.HTTPIngressPathArray{
							&networkingv1.HTTPIngressPathArgs{
								Path:     pulumi.String("/"),
								PathType: pulumi.String("Prefix"),
								Backend: &networkingv1.IngressBackendArgs{
									Service: &networkingv1.IngressServiceBackendArgs{
										Name: args.ServiceName,
										Port: &networkingv1.ServiceBackendPortArgs{Number: pulumi.Int(args.ServicePort)},
									},
								},
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

	// 2. Once any ALLOW AuthorizationPolicy targets a Service via
	// TargetRefs, Istio denies everything not matching one of its rules -
	// this Service has no other ALLOW policy, so without this it would be
	// denied outright by pkg/components/istio's mesh-wide
	// waypoint-default-deny AuthorizationPolicy. This is the only app-layer
	// authorization Tailscale-routed requests get - auth is otherwise fully
	// delegated to tailnet membership/ACLs, by design. Primary enforcement
	// is still the Cilium CCNP pair in pkg/components/tailscale (only
	// tailscale-labeled pods can reach the waypoint's HBONE port at all).
	//
	// Checks only the source principal, no JWT/claims (unlike
	// pkg/components/cloudflare/accessjwt's equivalent policy) - an exact
	// ServiceAccount match, same precision as cloudflared's policy. The
	// per-Ingress proxy StatefulSet/pod name is dynamic (e.g.
	// "ts-private-lf7s4-0"), but the ServiceAccount it runs as is fixed and
	// shared across every proxy the tailscale-operator chart ever creates -
	// confirmed live (kubectl -o jsonpath='{.spec.serviceAccountName}' ->
	// "proxies") - so this can and should pin the exact identity rather
	// than a namespace-prefix wildcard.
	_, err = securityv1.NewAuthorizationPolicy(ctx, fmt.Sprintf("%s-allow-tailscale", name), &securityv1.AuthorizationPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fmt.Sprintf("%s-allow-tailscale", name)),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &securityv1.AuthorizationPolicySpecArgs{
			Action: pulumi.String("ALLOW"),
			TargetRefs: securityv1.AuthorizationPolicySpecTargetRefsArray{
				&securityv1.AuthorizationPolicySpecTargetRefsArgs{
					Group: pulumi.String(""),
					Kind:  pulumi.String("Service"),
					Name:  args.ServiceName,
				},
			},
			Rules: securityv1.AuthorizationPolicySpecRulesArray{
				&securityv1.AuthorizationPolicySpecRulesArgs{
					From: securityv1.AuthorizationPolicySpecRulesFromArray{
						&securityv1.AuthorizationPolicySpecRulesFromArgs{
							Source: &securityv1.AuthorizationPolicySpecRulesFromSourceArgs{
								Principals: pulumi.StringArray{
									pulumi.Sprintf("cluster.local/ns/%s/sa/%s", args.OperatorNamespace, tailscale.ProxiesServiceAccountName),
								},
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

	// 3. A proxied, inert placeholder DNS record for
	// <Hostname>.<CloudflareBaseDomain> - self-contained per-app (unlike
	// the redirect rule below), so created directly here rather than via
	// pkg/components/cloudflare/domain (that component's stated purpose is
	// specifically "CNAME to the Tunnel", a different semantic). Content
	// 192.0.2.1 is RFC 5737 TEST-NET-1, deliberately unreachable - it's
	// never actually dialed because the redirect rule below always serves a
	// 302 before any origin fetch is attempted.
	_, err = cloudflare.NewRecord(ctx, fmt.Sprintf("%s-tailscale-record", name), &cloudflare.RecordArgs{
		ZoneId:  args.CloudflareZoneID,
		Name:    pulumi.Sprintf("%s.%s", args.Hostname, args.CloudflareBaseDomain),
		Type:    pulumi.String("A"),
		Content: pulumi.String("192.0.2.1"),
		Proxied: pulumi.Bool(true),
		Comment: pulumi.String("Managed by Pulumi - Tailscale redirect (no real origin, see pkg/deploy/redirects.go)"),
	}, append(localOpts, pulumi.Provider(args.CloudflareProvider))...)
	if err != nil {
		return nil, err
	}

	ing.Hostname = pulumi.String(args.Hostname).ToStringOutput()
	ing.Redirect = RedirectRoute{
		Subdomain: args.Hostname,
		Target:    pulumi.Sprintf("https://%s.%s", args.Hostname, args.MagicDNSSuffix),
	}

	if err := ctx.RegisterResourceOutputs(ing, pulumi.Map{
		"hostname": ing.Hostname,
	}); err != nil {
		return nil, err
	}

	return ing, nil
}
