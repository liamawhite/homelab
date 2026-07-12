// Package longhorn provides a Pulumi component for deploying Longhorn distributed storage.
//
// Longhorn is a lightweight, reliable, and distributed block storage system for Kubernetes.
// This component deploys Longhorn via Helm chart with Istio ambient mesh integration.
//
// The component:
//   - Deploys Longhorn via Helm (all defaults) into a caller-supplied namespace
//   - Exports the default StorageClass for use by other components
//
// The namespace itself (including the Istio ambient-mode label) is created
// centrally by pkg/deploy/namespaces.go, not by this component.
//
// Example usage:
//
//	storage, err := longhorn.NewLonghorn(ctx, "longhorn", &longhorn.LonghornArgs{
//	    Version:   versions.Longhorn,
//	    Namespace: namespaces.Get(deploy.LonghornSystemNamespace).Metadata.Name(),
//	}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{istioMesh}))
package longhorn

import (
	"fmt"

	helmv4 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v4"
	storagev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/storage/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	helmRepository   = "https://charts.longhorn.io"
	storageClassName = "longhorn"
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
	// Namespace is longhorn-system's name, created centrally by
	// pkg/deploy/namespaces.go (with the Istio ambient-mode label) and
	// passed in here - this component does not create it.
	Namespace pulumi.StringInput
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

	longhorn.Namespace = args.Namespace.ToStringOutput()

	// 1. Deploy Longhorn Helm chart with default values
	chart, err := helmv4.NewChart(ctx, fmt.Sprintf("%s-chart", name), &helmv4.ChartArgs{
		Namespace: args.Namespace,
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

	// 2. Retrieve the default StorageClass created by Helm
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

	// 3. Register component outputs
	if err := ctx.RegisterResourceOutputs(longhorn, pulumi.Map{
		"namespace":           longhorn.Namespace,
		"defaultStorageClass": longhorn.DefaultStorageClass,
	}); err != nil {
		return nil, err
	}

	return longhorn, nil
}
