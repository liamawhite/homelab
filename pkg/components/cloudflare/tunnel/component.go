package tunnel

import (
	"encoding/json"
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/dns"
	ciliumv2 "github.com/liamawhite/homelab/pkg/crds/cilium/crds/kubernetes/cilium/v2"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Tunnel represents a Cloudflare Tunnel component
type Tunnel struct {
	pulumi.ResourceState

	TunnelID    pulumi.StringOutput
	TunnelCNAME pulumi.StringOutput
	Namespace   pulumi.StringOutput // echoes TunnelArgs.Namespace, not owned by this component
	Secret      *corev1.Secret
	Deployment  *appsv1.Deployment
}

// TunnelRoute is one ingress rule: a hostname (Subdomain + the Tunnel's own
// Domain) forwarded to a backend Kubernetes Service. Cloudflare's API models
// a tunnel's whole ingress list as a single object with no way to manage
// entries independently (confirmed against the cfd_tunnel configurations
// API - one PUT replaces the entire list), so callers can't register their
// own route directly; each app instead builds and hands back its own
// TunnelRoute (e.g. pkg/deploy/applications/home.go's Home.TunnelRoute())
// for whatever central place collects the full Routes list passed in here.
type TunnelRoute struct {
	Subdomain        string
	ServiceName      pulumi.StringInput
	ServiceNamespace pulumi.StringInput
	ServicePort      int
}

// TunnelArgs contains the configuration for Cloudflare Tunnel
type TunnelArgs struct {
	// Domain is the base domain shared by every Route's Subdomain.
	Domain pulumi.StringInput
	// Namespace is the namespace cloudflared itself runs in, created
	// centrally by pkg/deploy/namespaces.go and passed in here - this
	// component does not create it.
	Namespace           pulumi.StringInput
	TunnelName          string
	Routes              []TunnelRoute
	CloudflareAccountID pulumi.StringInput
	CloudflareProvider  *cloudflare.Provider
}

// NewTunnel creates a new Cloudflare Tunnel component
func NewTunnel(ctx *pulumi.Context, name string, args *TunnelArgs, opts ...pulumi.ResourceOption) (*Tunnel, error) {
	tunnel := &Tunnel{}

	err := ctx.RegisterComponentResource("homelab:cloudflare:tunnel", name, tunnel, opts...)
	if err != nil {
		return nil, err
	}

	// Child resources should have this component as their parent
	resourceOpts := append(opts, pulumi.Parent(tunnel))

	// Cloudflare-package resources need the Cloudflare provider passed
	// explicitly - resourceOpts alone (Provider(providers.Kubernetes) from
	// the caller) only resolves for kubernetes-package resources, so
	// without this the cloudflare provider falls back to the default
	// provider (unconfigured, no apiToken) and every Cloudflare resource
	// call below fails at apply time.
	cfResourceOpts := append(append([]pulumi.ResourceOption{}, resourceOpts...), pulumi.Provider(args.CloudflareProvider))

	// Generate a random suffix for unique tunnel naming
	randomSuffix, err := random.NewRandomPet(ctx, fmt.Sprintf("%s-suffix", name), &random.RandomPetArgs{
		Length: pulumi.Int(2),
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// Generate a deterministic random 32-byte secret for the tunnel
	randomSecret, err := random.NewRandomBytes(ctx, fmt.Sprintf("%s-secret", name), &random.RandomBytesArgs{
		Length: pulumi.Int(32),
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// 1. Create Cloudflare ZeroTrust Tunnel with random suffix
	tunnelName := pulumi.Sprintf("%s-%s", args.TunnelName, randomSuffix.ID())
	cfTunnel, err := cloudflare.NewZeroTrustTunnelCloudflared(ctx, fmt.Sprintf("%s-tunnel", name), &cloudflare.ZeroTrustTunnelCloudflaredArgs{
		AccountId: args.CloudflareAccountID,
		Name:      tunnelName,
		Secret:    randomSecret.Base64,
	}, cfResourceOpts...)
	if err != nil {
		return nil, err
	}

	tunnel.TunnelID = cfTunnel.ID().ToStringOutput()
	tunnel.TunnelCNAME = cfTunnel.Cname

	// 2. Configure tunnel ingress rules - one per Route, evaluated in order,
	// plus a trailing catch-all (required by Cloudflare).
	ingressRules := make(cloudflare.ZeroTrustTunnelCloudflaredConfigConfigIngressRuleArray, 0, len(args.Routes)+1)
	for _, route := range args.Routes {
		ingressRules = append(ingressRules, &cloudflare.ZeroTrustTunnelCloudflaredConfigConfigIngressRuleArgs{
			Hostname: pulumi.Sprintf("%s.%s", route.Subdomain, args.Domain),
			Service:  pulumi.Sprintf("http://%s.%s.svc.cluster.local:%d", route.ServiceName, route.ServiceNamespace, route.ServicePort),
		})
	}
	ingressRules = append(ingressRules, &cloudflare.ZeroTrustTunnelCloudflaredConfigConfigIngressRuleArgs{
		Service: pulumi.String("http_status:404"),
	})

	_, err = cloudflare.NewZeroTrustTunnelCloudflaredConfig(ctx, fmt.Sprintf("%s-config", name), &cloudflare.ZeroTrustTunnelCloudflaredConfigArgs{
		AccountId: args.CloudflareAccountID,
		TunnelId:  cfTunnel.ID().ToStringOutput(),
		Config: &cloudflare.ZeroTrustTunnelCloudflaredConfigConfigArgs{
			IngressRules: ingressRules,
		},
	}, cfResourceOpts...)
	if err != nil {
		return nil, err
	}

	tunnel.Namespace = args.Namespace.ToStringOutput()

	// 3. Create tunnel credentials JSON
	credentials := pulumi.All(args.CloudflareAccountID, cfTunnel.ID().ToStringOutput(), randomSecret.Base64).
		ApplyT(func(args []any) (string, error) {
			accountID := args[0].(string)
			tunnelID := args[1].(string)
			secret := args[2].(string)

			creds := map[string]string{
				"AccountTag":   accountID,
				"TunnelID":     tunnelID,
				"TunnelSecret": secret,
			}

			credsJSON, err := json.Marshal(creds)
			if err != nil {
				return "", fmt.Errorf("failed to marshal credentials: %w", err)
			}

			return string(credsJSON), nil
		}).(pulumi.StringOutput)

	// 5. Create Secret with tunnel credentials
	tunnelSecret, err := corev1.NewSecret(ctx, fmt.Sprintf("%s-secret", name), &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("tunnel-credentials"),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Type: pulumi.String("Opaque"),
		StringData: pulumi.StringMap{
			"credentials.json": credentials,
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	tunnel.Secret = tunnelSecret

	// 5. Dedicated ServiceAccount for cloudflared - every app gets its own
	// rather than running as its namespace's shared "default" account.
	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("cloudflared"),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// 6. Create tunnel token for cloudflared
	tunnelToken := cfTunnel.TunnelToken

	// 7. Create cloudflared Deployment
	deployment, err := appsv1.NewDeployment(ctx, fmt.Sprintf("%s-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("cloudflared"),
			Namespace: args.Namespace.ToStringPtrOutput(),
			Labels: pulumi.StringMap{
				"app": pulumi.String("cloudflared"),
			},
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(2),
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: pulumi.StringMap{
					"app": pulumi.String("cloudflared"),
				},
			},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: pulumi.StringMap{
						"app": pulumi.String("cloudflared"),
						// cloudflared resolves Cloudflare edge hostnames to
						// establish the tunnel - needs DNS access under
						// Cilium's default-deny egress baseline.
						dns.AccessLabelKey: pulumi.String(dns.AccessLabelValue),
						// cloudflared also needs to actually reach the
						// Cloudflare Tunnel edge itself (see
						// allow-egress-cloudflare-tunnel below).
						AccessLabelKey: pulumi.String(AccessLabelValue),
						// Stays ambient-enrolled (unlike
						// istiod/ztunnel/istio-cni) - ztunnel has to
						// capture this pod's own egress for it to reach an
						// app's waypoint at all (confirmed live: opting
						// out via istio.io/dataplane-mode: none breaks
						// cloudflared's connection to the home app's
						// waypoint - default-deny plus the waypoint's own
						// ingress policy only accepting traffic from the
						// waypoint's own identity means a non-ambient
						// source's direct, un-tunneled connection just
						// gets dropped). See the missing LivenessProbe
						// below for the other half of this trade-off.
					},
				},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("cloudflared"),
							Image: pulumi.String("cloudflare/cloudflared:latest"),
							Args: pulumi.StringArray{
								pulumi.String("tunnel"),
								pulumi.String("--no-autoupdate"),
								pulumi.String("--metrics"),
								pulumi.String("0.0.0.0:2000"),
								pulumi.String("run"),
								pulumi.String("--token"),
								tunnelToken,
							},
							// No LivenessProbe: kubelet's own HTTP GET
							// probe against this pod's IP is exactly the
							// kind of traffic ztunnel's ambient capture
							// intercepts, and it doesn't handle plain HTTP
							// - the connection just hangs until the probe
							// times out, killing an otherwise-healthy
							// process (confirmed live: CPU usage stayed
							// ~2m/100m the whole time - a false positive,
							// not real unresponsiveness). This is a
							// documented Istio+Cilium ambient
							// incompatibility (istio/istio#49277, #57911)
							// with no working fix found for this cluster:
							// patching istio-cni's HOST_PROBE_SNAT_IP is a
							// security regression (defeats the point of
							// the default link-local SNAT address, which
							// exists to prevent IP spoofing on pods), and
							// Cilium's bpf.hostLegacyRouting (confirmed
							// active via cilium-dbg status) had no effect,
							// likely because this cluster's VXLAN tunnel
							// mode already forces legacy host routing
							// regardless of that setting. Traded away in
							// favor of staying ambient-enrolled (see the
							// Labels comment above) - Kubernetes still
							// restarts this container on a real crash
							// (non-zero exit) regardless of whether a
							// liveness probe is configured.
							Resources: &corev1.ResourceRequirementsArgs{
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("100m"),
									"memory": pulumi.String("128Mi"),
								},
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("20m"),
									"memory": pulumi.String("64Mi"),
								},
							},
						},
					},
				},
			},
		},
	}, append(resourceOpts, pulumi.DependsOn([]pulumi.Resource{tunnelSecret, cfTunnel}))...)
	if err != nil {
		return nil, err
	}

	tunnel.Deployment = deployment

	// 8. Only pods carrying AccessLabelKey/AccessLabelValue can reach the
	// Cloudflare Tunnel edge - egress access is opt-in per workload, not
	// blanket, same as every other network policy in this repo. Cloudflare
	// edge IPs aren't expressible as a fixed CIDR, hence ToEntities "world"
	// restricted by port, same reasoning as pkg/components/dns's
	// allow-egress-coredns-upstream. Port 7844 (UDP+TCP) is the tunnel
	// protocol itself (QUIC, falling back to h2mux); port 443 is needed for
	// cloudflared's own control-plane calls (e.g. api.cloudflare.com).
	// Requires the Cilium CiliumClusterwideNetworkPolicy CRD to already
	// exist - callers must pass pulumi.DependsOn on the Cilium installation
	// (see pkg/components/cilium.NewCilium).
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-cloudflare-tunnel", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-cloudflare-tunnel"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					AccessLabelKey: pulumi.String(AccessLabelValue),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEntities: pulumi.StringArray{pulumi.String("world")},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("7844"), Protocol: pulumi.String("UDP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("7844"), Protocol: pulumi.String("TCP")},
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("443"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// 9. Only pods carrying AccessLabelKey/AccessLabelValue (i.e.
	// cloudflared) can reach waypoints that opt in via
	// WaypointAccessLabelKey/Value - needed for cloudflared to actually
	// deliver a tunneled HTTP request to an app's Service once ambient
	// routes it through that Service's waypoint (see
	// pkg/components/istio/waypoint), on the waypoint's HBONE mesh
	// listener (port 15008). This is the generic, reusable half of that
	// path; the waypoint-to-app leg is specific to each app's own pods and
	// lives with that app instead (see pkg/deploy/applications/home.go).
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-egress-waypoints", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-egress-cloudflare-tunnel-waypoints"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					AccessLabelKey: pulumi.String(AccessLabelValue),
				},
			},
			Egress: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressArgs{
					ToEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToEndpointsArgs{
							MatchLabels: pulumi.StringMap{
								WaypointAccessLabelKey: pulumi.String(WaypointAccessLabelValue),
							},
						},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecEgressToPortsPortsArgs{Port: pulumi.String("15008"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// 10. Ingress counterpart to #9 - default-deny blocks ingress
	// independently of egress, so a waypoint that opted in via
	// WaypointAccessLabelKey also needs to actually accept the connection
	// (same lesson as istiod's own missing ingress policy, see
	// pkg/components/istio).
	_, err = ciliumv2.NewCiliumClusterwideNetworkPolicy(ctx, fmt.Sprintf("%s-allow-ingress-waypoints", name), &ciliumv2.CiliumClusterwideNetworkPolicyArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("allow-ingress-cloudflare-tunnel-waypoints"),
		},
		Spec: &ciliumv2.CiliumClusterwideNetworkPolicySpecArgs{
			EndpointSelector: &ciliumv2.CiliumClusterwideNetworkPolicySpecEndpointSelectorArgs{
				MatchLabels: pulumi.StringMap{
					WaypointAccessLabelKey: pulumi.String(WaypointAccessLabelValue),
				},
			},
			Ingress: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArray{
				&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressArgs{
					FromEndpoints: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressFromEndpointsArgs{
							MatchLabels: pulumi.StringMap{
								AccessLabelKey: pulumi.String(AccessLabelValue),
							},
						},
					},
					ToPorts: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArray{
						&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsArgs{
							Ports: ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArray{
								&ciliumv2.CiliumClusterwideNetworkPolicySpecIngressToPortsPortsArgs{Port: pulumi.String("15008"), Protocol: pulumi.String("TCP")},
							},
						},
					},
				},
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// Register outputs
	if err := ctx.RegisterResourceOutputs(tunnel, pulumi.Map{
		"tunnelId":    tunnel.TunnelID,
		"tunnelCname": tunnel.TunnelCNAME,
		"namespace":   tunnel.Namespace,
		"deployment":  deployment.Metadata.Name(),
	}); err != nil {
		return nil, err
	}

	return tunnel, nil
}
