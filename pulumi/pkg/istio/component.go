package istio

import (
	"fmt"

	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Istio represents the Istio service mesh component
type Istio struct {
	pulumi.ResourceState

	Namespace pulumi.StringOutput
}

// IstioArgs contains the configuration for Istio
type IstioArgs struct {
	// Version is the Istio version to deploy (e.g., "1.28.2")
	Version string
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
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("base"),
		Version:   pulumi.String(args.Version),
		RepositoryOpts: repositoryOpts,
		Values:         pulumi.Map{},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 3. Install istiod (control plane)
	_, err = helmv4.NewChart(ctx, fmt.Sprintf("%s-istiod", name), &helmv4.ChartArgs{
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("istiod"),
		Version:   pulumi.String(args.Version),
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
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("cni"),
		Version:   pulumi.String(args.Version),
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
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("ztunnel"),
		Version:   pulumi.String(args.Version),
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

	// Register outputs
	if err := ctx.RegisterResourceOutputs(istio, pulumi.Map{
		"namespace": istio.Namespace,
	}); err != nil {
		return nil, err
	}

	return istio, nil
}
