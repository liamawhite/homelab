package kubevip

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// KubeVip represents a kube-vip deployment for control plane HA
type KubeVip struct {
	pulumi.ResourceState

	Namespace           pulumi.StringOutput
	ServiceAccount      *corev1.ServiceAccount
	ClusterRole         *rbacv1.ClusterRole
	ClusterRoleBinding  *rbacv1.ClusterRoleBinding
	DaemonSet           *appsv1.DaemonSet
}

// KubeVipArgs contains the configuration for kube-vip
type KubeVipArgs struct {
	VIP     string // Control plane VIP address
	Version string // kube-vip container image version
}

// NewKubeVip creates a new kube-vip component resource
func NewKubeVip(ctx *pulumi.Context, name string, args *KubeVipArgs, opts ...pulumi.ResourceOption) (*KubeVip, error) {
	kubeVip := &KubeVip{}

	err := ctx.RegisterComponentResource("homelab:kubernetes:kubevip", name, kubeVip, opts...)
	if err != nil {
		return nil, err
	}

	// Child resources should have this component as their parent
	//
	// IMPORTANT: This option protects kube-vip during provider replacements.
	//
	// Why provider replacements happen:
	// After kube-vip is deployed, the kubeconfig is typically updated to use the VIP
	// (192.168.1.50) instead of a specific node IP (e.g., 192.168.1.51). This changes
	// the kubeconfig content, which causes Pulumi to see the Kubernetes provider as
	// "changed". When the provider changes, all resources using that provider get new
	// URNs and Pulumi wants to replace them.
	//
	// DeleteBeforeReplace(false): When Pulumi needs to replace a resource, it will create
	// the new resource first, then delete the old one. This ensures zero downtime during
	// replacements. Without this, Pulumi would delete first, then create, causing a service
	// outage.
	resourceOpts := append(opts,
		pulumi.Parent(kubeVip),
		pulumi.DeleteBeforeReplace(false),
	)

	// Use kube-system namespace
	namespace := pulumi.String("kube-system")

	// Create RBAC resources
	serviceAccount, clusterRole, clusterRoleBinding, err := createRBACResources(
		ctx,
		name,
		namespace,
		resourceOpts...,
	)
	if err != nil {
		return nil, err
	}

	// Create DaemonSet
	daemonSet, err := createDaemonSet(
		ctx,
		name,
		args,
		namespace,
		serviceAccount.Metadata.Name().Elem(),
		append(resourceOpts, pulumi.DependsOn([]pulumi.Resource{
			serviceAccount,
			clusterRole,
			clusterRoleBinding,
		}))...,
	)
	if err != nil {
		return nil, err
	}

	// Set outputs
	kubeVip.Namespace = namespace.ToStringOutput()
	kubeVip.ServiceAccount = serviceAccount
	kubeVip.ClusterRole = clusterRole
	kubeVip.ClusterRoleBinding = clusterRoleBinding
	kubeVip.DaemonSet = daemonSet

	// Register outputs
	if err := ctx.RegisterResourceOutputs(kubeVip, pulumi.Map{
		"namespace":          kubeVip.Namespace,
		"serviceAccount":     serviceAccount.Metadata.Name(),
		"clusterRole":        clusterRole.Metadata.Name(),
		"clusterRoleBinding": clusterRoleBinding.Metadata.Name(),
		"daemonSet":          daemonSet.Metadata.Name(),
	}); err != nil {
		return nil, err
	}

	return kubeVip, nil
}
