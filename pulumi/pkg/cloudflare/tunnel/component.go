package tunnel

import (
	"encoding/json"
	"fmt"

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
	DNSRecord   *cloudflare.Record
	Namespace   *corev1.Namespace
	Secret      *corev1.Secret
	Deployment  *appsv1.Deployment
}

// TunnelArgs contains the configuration for Cloudflare Tunnel
type TunnelArgs struct {
	Domain              pulumi.StringInput
	Subdomain           string
	TunnelName          string
	GatewayNamespace    pulumi.StringInput
	GatewayService      pulumi.StringInput
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
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	tunnel.TunnelID = cfTunnel.ID().ToStringOutput()
	tunnel.TunnelCNAME = cfTunnel.Cname

	// 2. Configure tunnel ingress rules
	gatewayServiceURL := pulumi.Sprintf("http://%s.%s.svc.cluster.local", args.GatewayService, args.GatewayNamespace)
	hostnamePattern := pulumi.Sprintf("%s.%s", args.Subdomain, args.Domain)

	_, err = cloudflare.NewZeroTrustTunnelCloudflaredConfig(ctx, fmt.Sprintf("%s-config", name), &cloudflare.ZeroTrustTunnelCloudflaredConfigArgs{
		AccountId: args.CloudflareAccountID,
		TunnelId:  cfTunnel.ID().ToStringOutput(),
		Config: &cloudflare.ZeroTrustTunnelCloudflaredConfigConfigArgs{
			IngressRules: cloudflare.ZeroTrustTunnelCloudflaredConfigConfigIngressRuleArray{
				// Route wildcard subdomain to Gateway service
				&cloudflare.ZeroTrustTunnelCloudflaredConfigConfigIngressRuleArgs{
					Hostname: hostnamePattern,
					Service:  gatewayServiceURL,
				},
				// Catch-all rule (required by Cloudflare)
				&cloudflare.ZeroTrustTunnelCloudflaredConfigConfigIngressRuleArgs{
					Service: pulumi.String("http_status:404"),
				},
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	// 3. Lookup Zone ID and create DNS record
	zoneID := pulumi.All(args.Domain, args.CloudflareAccountID).ApplyT(func(inputs []interface{}) (string, error) {
		domain := inputs[0].(string)
		accountID := inputs[1].(string)

		zone, err := cloudflare.LookupZone(ctx, &cloudflare.LookupZoneArgs{
			Name:      pulumi.StringRef(domain),
			AccountId: pulumi.StringRef(accountID),
		}, pulumi.Provider(args.CloudflareProvider))
		if err != nil {
			return "", fmt.Errorf("failed to lookup zone for domain %s: %w", domain, err)
		}
		return zone.Id, nil
	}).(pulumi.StringOutput)

	dnsRecord, err := cloudflare.NewRecord(ctx, fmt.Sprintf("%s-dns", name), &cloudflare.RecordArgs{
		ZoneId:  zoneID,
		Name:    pulumi.Sprintf("%s.%s", args.Subdomain, args.Domain),
		Type:    pulumi.String("CNAME"),
		Content: cfTunnel.Cname,
		Proxied: pulumi.Bool(true),
		Comment: pulumi.String("Managed by Pulumi - Cloudflare Tunnel for Istio Gateway"),
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	tunnel.DNSRecord = dnsRecord

	// 4. Create Kubernetes namespace for cloudflared
	namespace, err := corev1.NewNamespace(ctx, fmt.Sprintf("%s-namespace", name), &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("cloudflare-tunnel"),
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	tunnel.Namespace = namespace

	// 5. Create tunnel credentials JSON
	credentials := pulumi.All(args.CloudflareAccountID, cfTunnel.ID().ToStringOutput(), randomSecret.Base64).
		ApplyT(func(args []interface{}) (string, error) {
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

	// 6. Create Secret with tunnel credentials
	tunnelSecret, err := corev1.NewSecret(ctx, fmt.Sprintf("%s-secret", name), &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("tunnel-credentials"),
			Namespace: namespace.Metadata.Name(),
		},
		Type: pulumi.String("Opaque"),
		StringData: pulumi.StringMap{
			"credentials.json": credentials,
		},
	}, append(resourceOpts, pulumi.DependsOn([]pulumi.Resource{namespace}))...)
	if err != nil {
		return nil, err
	}

	tunnel.Secret = tunnelSecret

	// 7. Create tunnel token for cloudflared
	tunnelToken := cfTunnel.TunnelToken

	// 8. Create cloudflared Deployment
	deployment, err := appsv1.NewDeployment(ctx, fmt.Sprintf("%s-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("cloudflared"),
			Namespace: namespace.Metadata.Name(),
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
					},
				},
				Spec: &corev1.PodSpecArgs{
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
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/ready"),
									Port: pulumi.Int(2000),
								},
								InitialDelaySeconds: pulumi.Int(10),
								PeriodSeconds:       pulumi.Int(10),
							},
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
	}, append(resourceOpts, pulumi.DependsOn([]pulumi.Resource{namespace, tunnelSecret, cfTunnel}))...)
	if err != nil {
		return nil, err
	}

	tunnel.Deployment = deployment

	// Register outputs
	if err := ctx.RegisterResourceOutputs(tunnel, pulumi.Map{
		"tunnelId":    tunnel.TunnelID,
		"tunnelCname": tunnel.TunnelCNAME,
		"dnsRecord":   dnsRecord.Name,
		"namespace":   namespace.Metadata.Name(),
		"deployment":  deployment.Metadata.Name(),
	}); err != nil {
		return nil, err
	}

	return tunnel, nil
}
