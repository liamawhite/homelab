package istio

import (
	"fmt"

	istiov1beta1 "github.com/liamawhite/homelab/pulumi/pkg/istio/crds/kubernetes/networking/v1beta1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Istio represents the Istio service mesh component
type Istio struct {
	pulumi.ResourceState

	Namespace      pulumi.StringOutput
	GatewayService pulumi.StringOutput
}

// IstioArgs contains the configuration for Istio
type IstioArgs struct {
	// Version is the Istio version to deploy (e.g., "1.28.2")
	Version string
	// GatewayDomain is the domain for the gateway
	GatewayDomain pulumi.StringInput
	// HealthSubdomain is the subdomain for the health check endpoint (e.g., "health")
	HealthSubdomain pulumi.StringInput
}

// NewIstio creates a new Istio component with ambient mesh profile
func NewIstio(ctx *pulumi.Context, name string, args *IstioArgs, opts ...pulumi.ResourceOption) (*Istio, error) {
	istio := &Istio{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:istio", name, istio, opts...)
	if err != nil {
		return nil, err
	}

	// All child resources should be parented to this component
	localOpts := append(opts, pulumi.Parent(istio))

	// 1. Create istio-system namespace
	namespace, err := corev1.NewNamespace(ctx, name, &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("istio-system"),
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}
	istio.Namespace = namespace.Metadata.Name().Elem()

	// Helm repository configuration
	repositoryOpts := &helmv4.RepositoryOptsArgs{
		Repo: pulumi.String("https://istio-release.storage.googleapis.com/charts"),
	}

	// 2. Install Istio CRDs (base chart)
	_, err = helmv4.NewChart(ctx, fmt.Sprintf("%s-crds", name), &helmv4.ChartArgs{
		Namespace:      namespace.Metadata.Name(),
		Chart:          pulumi.String("base"),
		Version:        pulumi.String(args.Version),
		RepositoryOpts: repositoryOpts,
		Values:         pulumi.Map{},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 3. Install istiod (control plane)
	_, err = helmv4.NewChart(ctx, fmt.Sprintf("%s-istiod", name), &helmv4.ChartArgs{
		Namespace:      namespace.Metadata.Name(),
		Chart:          pulumi.String("istiod"),
		Version:        pulumi.String(args.Version),
		RepositoryOpts: repositoryOpts,
		Values: pulumi.Map{
			"profile": pulumi.String("ambient"),
			"pilot": pulumi.Map{
				"resources": pulumi.Map{
					"limits": pulumi.Map{
						"cpu":    pulumi.String("200m"),
						"memory": pulumi.String("128Mi"),
					},
					"requests": pulumi.Map{
						"cpu":    pulumi.String("20m"),
						"memory": pulumi.String("64Mi"),
					},
				},
			},
			"meshConfig": pulumi.Map{
				"accessLogFile": pulumi.String("/dev/stdout"),
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 4. Install CNI plugin (K3s-specific paths)
	_, err = helmv4.NewChart(ctx, fmt.Sprintf("%s-cni", name), &helmv4.ChartArgs{
		Namespace:      namespace.Metadata.Name(),
		Chart:          pulumi.String("cni"),
		Version:        pulumi.String(args.Version),
		RepositoryOpts: repositoryOpts,
		Values: pulumi.Map{
			"profile": pulumi.String("ambient"),
			"global": pulumi.Map{
				"platform": pulumi.String("k3s"),
			},
			// K3s-specific CNI paths
			// https://github.com/k3s-io/k3s/issues/11076
			"cni": pulumi.Map{
				"cniConfDir": pulumi.String("/var/lib/rancher/k3s/agent/etc/cni/net.d"),
				"cniBinDir":  pulumi.String("/var/lib/rancher/k3s/data/cni"),
				"resources": pulumi.Map{
					"limits": pulumi.Map{
						"cpu":    pulumi.String("100m"),
						"memory": pulumi.String("64Mi"),
					},
					"requests": pulumi.Map{
						"cpu":    pulumi.String("10m"),
						"memory": pulumi.String("32Mi"),
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 5. Install ztunnel (zero-trust tunnel for ambient mesh)
	_, err = helmv4.NewChart(ctx, fmt.Sprintf("%s-ztunnel", name), &helmv4.ChartArgs{
		Namespace:      namespace.Metadata.Name(),
		Chart:          pulumi.String("ztunnel"),
		Version:        pulumi.String(args.Version),
		RepositoryOpts: repositoryOpts,
		Values: pulumi.Map{
			"resources": pulumi.Map{
				"limits": pulumi.Map{
					"cpu":    pulumi.String("200m"),
					"memory": pulumi.String("128Mi"),
				},
				"requests": pulumi.Map{
					"cpu":    pulumi.String("20m"),
					"memory": pulumi.String("96Mi"),
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 5. Deploy Istio Ingress Gateway
	// The Helm chart creates a service with the same name as the release
	ingressReleaseName := "istio-ingressgateway"
	_, err = helmv4.NewChart(ctx, ingressReleaseName, &helmv4.ChartArgs{
		Chart:          pulumi.String("gateway"),
		Version:        pulumi.String(args.Version),
		Namespace:      namespace.Metadata.Name(),
		RepositoryOpts: repositoryOpts,
		Values: pulumi.Map{
			"service": pulumi.Map{
				"type": pulumi.String("ClusterIP"), // Use ClusterIP since we're behind Cloudflare Tunnel
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}
	istio.GatewayService = pulumi.String(ingressReleaseName).ToStringOutput()

	// 6. Create main Istio Gateway resource
	gatewayName := "main-gateway"
	_, err = istiov1beta1.NewGateway(ctx, fmt.Sprintf("%s-gateway", name), &istiov1beta1.GatewayArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(gatewayName),
			Namespace: namespace.Metadata.Name(),
		},
		Spec: &istiov1beta1.GatewaySpecArgs{
			// Select the istio ingress gateway pods
			Selector: pulumi.StringMap{
				"istio": pulumi.String("ingressgateway"),
			},
			Servers: istiov1beta1.GatewaySpecServersArray{
				// HTTP server
				&istiov1beta1.GatewaySpecServersArgs{
					Port: &istiov1beta1.GatewaySpecServersPortArgs{
						Number:   pulumi.Int(80),
						Name:     pulumi.String("http"),
						Protocol: pulumi.String("HTTP"),
					},
					Hosts: pulumi.StringArray{
						pulumi.Sprintf("*.%s", args.GatewayDomain),
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 7. Create health check VirtualService
	healthHostname := pulumi.Sprintf("%s.%s", args.HealthSubdomain, args.GatewayDomain)
	_, err = istiov1beta1.NewVirtualService(ctx, fmt.Sprintf("%s-health-check", name), &istiov1beta1.VirtualServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("health-check"),
			Namespace: namespace.Metadata.Name(),
		},
		Spec: &istiov1beta1.VirtualServiceSpecArgs{
			Hosts:    pulumi.StringArray{healthHostname},
			Gateways: pulumi.StringArray{pulumi.String(gatewayName)},
			Http: istiov1beta1.VirtualServiceSpecHttpArray{
				&istiov1beta1.VirtualServiceSpecHttpArgs{
					Match: istiov1beta1.VirtualServiceSpecHttpMatchArray{
						&istiov1beta1.VirtualServiceSpecHttpMatchArgs{
							Uri: &istiov1beta1.VirtualServiceSpecHttpMatchUriArgs{
								Prefix: pulumi.String("/"),
							},
						},
					},
					DirectResponse: &istiov1beta1.VirtualServiceSpecHttpDirectResponseArgs{
						Status: pulumi.Int(200),
						Body: &istiov1beta1.VirtualServiceSpecHttpDirectResponseBodyArgs{
							String: pulumi.String("OK - Cloudflare Tunnel → Istio Gateway → Health Check\n"),
						},
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// Register outputs
	if err := ctx.RegisterResourceOutputs(istio, pulumi.Map{
		"namespace":      istio.Namespace,
		"gatewayService": istio.GatewayService,
	}); err != nil {
		return nil, err
	}

	return istio, nil
}
