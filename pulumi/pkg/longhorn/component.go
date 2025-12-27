// Package longhorn provides a Pulumi component for deploying Longhorn distributed storage.
//
// Longhorn is a lightweight, reliable, and distributed block storage system for Kubernetes.
// This component deploys Longhorn via Helm chart with Istio ambient mesh integration.
//
// The component:
//   - Creates a namespace with Istio ambient mode enabled
//   - Deploys Longhorn via Helm (all defaults)
//   - Exports the default StorageClass for use by other components
//
// Example usage:
//
//	storage, err := longhorn.NewLonghorn(ctx, "longhorn", &longhorn.LonghornArgs{},
//	    pulumi.Provider(k8sProvider),
//	    pulumi.DependsOn([]pulumi.Resource{istioMesh}),
//	)
package longhorn

import (
	"fmt"

	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	storagev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/storage/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	helmRepository   = "https://charts.longhorn.io"
	namespaceName    = "longhorn-system"
	storageClassName = "longhorn"
	ambientModeLabel = "istio.io/dataplane-mode"
	ambientModeValue = "ambient"
)

// Longhorn represents the Longhorn storage system component
type Longhorn struct {
	pulumi.ResourceState

	Namespace           pulumi.StringOutput
	DefaultStorageClass pulumi.StringOutput
}

// LonghornArgs contains the configuration for Longhorn
type LonghornArgs struct {
	// Version is the Longhorn Helm chart version to deploy
	Version string
}

// NewLonghorn creates a new Longhorn component with distributed storage
func NewLonghorn(
	ctx *pulumi.Context,
	name string,
	args *LonghornArgs,
	opts ...pulumi.ResourceOption,
) (*Longhorn, error) {
	// Validate required args
	if args.Version == "" {
		return nil, fmt.Errorf("longhorn version is required")
	}

	longhorn := &Longhorn{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:longhorn", name, longhorn, opts...)
	if err != nil {
		return nil, err
	}

	// All child resources should be parented to this component
	localOpts := append(opts, pulumi.Parent(longhorn))

	// 1. Create longhorn-system namespace with Istio ambient mesh labels
	namespace, err := corev1.NewNamespace(ctx, name, &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(namespaceName),
			Labels: pulumi.StringMap{
				ambientModeLabel: pulumi.String(ambientModeValue),
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}
	longhorn.Namespace = namespace.Metadata.Name().Elem()

	// 2. Deploy Longhorn Helm chart with default values
	chart, err := helmv4.NewChart(ctx, fmt.Sprintf("%s-chart", name), &helmv4.ChartArgs{
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("longhorn"),
		Version:   pulumi.String(args.Version),
		RepositoryOpts: &helmv4.RepositoryOptsArgs{
			Repo: pulumi.String(helmRepository),
		},
		Values: pulumi.Map{}, // No custom values - all defaults
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 3. Retrieve the default StorageClass created by Helm
	storageClass, err := storagev1.GetStorageClass(ctx,
		fmt.Sprintf("%s-default-sc", name),
		pulumi.ID(storageClassName),
		&storagev1.StorageClassState{},
		append(localOpts, pulumi.DependsOn([]pulumi.Resource{chart}))...,
	)
	if err != nil {
		return nil, err
	}
	longhorn.DefaultStorageClass = storageClass.Metadata.Name().Elem()

	// 4. Register component outputs
	if err := ctx.RegisterResourceOutputs(longhorn, pulumi.Map{
		"namespace":           longhorn.Namespace,
		"defaultStorageClass": longhorn.DefaultStorageClass,
	}); err != nil {
		return nil, err
	}

	return longhorn, nil
}
