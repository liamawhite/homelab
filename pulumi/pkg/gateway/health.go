package gateway

import (
	istiov1beta1 "github.com/liamawhite/homelab/pulumi/pkg/istio/crds/kubernetes/networking/v1beta1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NewHealthCheck creates a health check endpoint using Istio VirtualService with direct response
func NewHealthCheck(ctx *pulumi.Context, namespace pulumi.StringInput, gatewayName string, hostname string, opts ...pulumi.ResourceOption) error {
	// Create VirtualService with direct response
	_, err := istiov1beta1.NewVirtualService(ctx, "health-check", &istiov1beta1.VirtualServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Namespace: namespace,
		},
		Spec: &istiov1beta1.VirtualServiceSpecArgs{
			Hosts:    pulumi.StringArray{pulumi.String(hostname)},
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
	}, opts...)

	return err
}
