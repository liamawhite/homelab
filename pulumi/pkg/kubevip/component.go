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
	resourceOpts := append(opts, pulumi.Parent(kubeVip))

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
