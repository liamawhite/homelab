package gateway

import (
	istiov1beta1 "github.com/liamawhite/homelab/pulumi/pkg/istio/crds/kubernetes/networking/v1beta1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Gateway represents an Istio Gateway
type Gateway struct {
	pulumi.ResourceState

	Gateway *istiov1beta1.Gateway
}

// GatewayArgs contains the configuration for Gateway
type GatewayArgs struct {
	// Namespace is the namespace to deploy the gateway into
	Namespace pulumi.StringInput
}

// NewGateway creates a new Istio Gateway resource
func NewGateway(ctx *pulumi.Context, name string, args *GatewayArgs, opts ...pulumi.ResourceOption) (*Gateway, error) {
	gateway := &Gateway{}
	err := ctx.RegisterComponentResource("homelab:kubernetes:gateway", name, gateway, opts...)
	if err != nil {
		return nil, err
	}

	// All child resources should be parented to this component
	localOpts := append(opts, pulumi.Parent(gateway))

	// Create Istio Gateway with HTTP listener
	gw, err := istiov1beta1.NewGateway(ctx, name, &istiov1beta1.GatewayArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace,
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
						pulumi.String("*"),
					},
				},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	gateway.Gateway = gw

	// Register outputs
	if err := ctx.RegisterResourceOutputs(gateway, pulumi.Map{
		"gateway": gw,
	}); err != nil {
		return nil, err
	}

	return gateway, nil
}
