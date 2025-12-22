package kubevip

import (
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// createRBACResources creates ServiceAccount, ClusterRole, and ClusterRoleBinding for kube-vip
func createRBACResources(ctx *pulumi.Context, name string, namespace pulumi.StringInput, opts ...pulumi.ResourceOption) (*corev1.ServiceAccount, *rbacv1.ClusterRole, *rbacv1.ClusterRoleBinding, error) {
	// ServiceAccount
	serviceAccount, err := corev1.NewServiceAccount(ctx, name+"-sa", &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("kube-vip"),
			Namespace: namespace,
		},
	}, opts...)
	if err != nil {
		return nil, nil, nil, err
	}

	// ClusterRole
	clusterRole, err := rbacv1.NewClusterRole(ctx, name+"-cr", &rbacv1.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("kube-vip"),
		},
		Rules: rbacv1.PolicyRuleArray{
			// Permissions for service, endpoint, and node discovery
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{
					pulumi.String("services"),
					pulumi.String("endpoints"),
					pulumi.String("nodes"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("list"),
					pulumi.String("get"),
					pulumi.String("watch"),
				},
			},
			// Permissions for configmap management
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("")},
				Resources: pulumi.StringArray{
					pulumi.String("configmaps"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("list"),
					pulumi.String("get"),
					pulumi.String("watch"),
					pulumi.String("create"),
					pulumi.String("update"),
				},
			},
			// Permissions for lease-based leader election
			&rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{pulumi.String("coordination.k8s.io")},
				Resources: pulumi.StringArray{
					pulumi.String("leases"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("list"),
					pulumi.String("get"),
					pulumi.String("watch"),
					pulumi.String("create"),
					pulumi.String("update"),
				},
			},
		},
	}, opts...)
	if err != nil {
		return nil, nil, nil, err
	}

	// ClusterRoleBinding
	clusterRoleBinding, err := rbacv1.NewClusterRoleBinding(ctx, name+"-crb", &rbacv1.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("kube-vip"),
		},
		RoleRef: &rbacv1.RoleRefArgs{
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			Kind:     pulumi.String("ClusterRole"),
			Name:     clusterRole.Metadata.Name().Elem(),
		},
		Subjects: rbacv1.SubjectArray{
			&rbacv1.SubjectArgs{
				Kind:      pulumi.String("ServiceAccount"),
				Name:      serviceAccount.Metadata.Name().Elem(),
				Namespace: namespace,
			},
		},
	}, opts...)
	if err != nil {
		return nil, nil, nil, err
	}

	return serviceAccount, clusterRole, clusterRoleBinding, nil
}
