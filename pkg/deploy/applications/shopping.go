package applications

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	waypoint "github.com/liamawhite/homelab/pkg/components/istio/waypoint"
	"github.com/liamawhite/homelab/pkg/components/tailscale"
	"github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumitime "github.com/pulumiverse/pulumi-time/sdk/go/time"

	"github.com/pulumi/pulumi-docker-build/sdk/go/dockerbuild"
)

const (
	shoppingImageName = "ghcr.io/liamawhite/shopping"
	shoppingPort      = 8080

	// shoppingUID is the non-root UID the container's binary runs as
	// (matches applications/shopping/Dockerfile's scratch stage - no
	// /etc/passwd entry exists there either, so this is a bare numeric
	// UID/GID, same convention as workouts/lumenetes). The PVC's fsGroup
	// below has to match it: Longhorn (like any dynamically provisioned
	// volume) mounts owned by root by default, which a non-root container
	// can't write to without fsGroup chowning the mount to this GID first.
	shoppingUID = 65534
)

// BuildShoppingImage builds and pushes the shopping app's image (Go backend
// with the built React frontend go:embedded into it - see
// applications/shopping/Dockerfile) via buildx/BuildKit through the Pulumi
// Docker Build provider, as part of `homelab up`/`preview` itself - same
// convention as BuildWorkoutsImage, see its doc comment for why (declarative
// GHCR auth, a stable-per-lineage timestamp tag instead of time.Now(),
// digest-pinned Deployments regardless of tag naming).
//
// Like workouts, this module has no local go.mod "replace" back to the root
// module, so the build context is this module's own directory
// (applications/shopping), not the repo root - repoRoot is still resolved
// via os.Getwd() for the same reason BuildWorkoutsImage does (this code runs
// in-process with the CLI, whose cwd is the repo root by convention; the
// docker-build provider plugin's cwd is not).
func BuildShoppingImage(ctx *pulumi.Context, name, ghcrUsername, ghcrToken string, opts ...pulumi.ResourceOption) (*dockerbuild.Image, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory: %w", err)
	}

	buildTime, err := pulumitime.NewStatic(ctx, fmt.Sprintf("%s-build-time", name), nil, opts...)
	if err != nil {
		return nil, err
	}

	buildTag := buildTime.Rfc3339.ApplyT(func(ts string) string {
		return strings.NewReplacer(":", "", "-", "").Replace(ts)
	}).(pulumi.StringOutput)

	return dockerbuild.NewImage(ctx, name, &dockerbuild.ImageArgs{
		Context: &dockerbuild.BuildContextArgs{
			Location: pulumi.String(filepath.Join(repoRoot, "applications/shopping")),
		},
		Dockerfile: &dockerbuild.DockerfileArgs{
			Location: pulumi.String(filepath.Join(repoRoot, "applications/shopping/Dockerfile")),
		},
		Platforms: dockerbuild.PlatformArray{
			dockerbuild.Platform_Linux_arm64,
			dockerbuild.Platform_Linux_amd64,
		},
		Push: pulumi.Bool(true),
		Registries: dockerbuild.RegistryArray{
			&dockerbuild.RegistryArgs{
				Address:  pulumi.String("ghcr.io"),
				Username: pulumi.StringPtr(ghcrUsername),
				Password: pulumi.StringPtr(ghcrToken),
			},
		},
		Tags: pulumi.StringArray{
			pulumi.String(shoppingImageName + ":latest"),
			pulumi.Sprintf("%s:%s", shoppingImageName, buildTag),
		},
	}, opts...)
}

// Shopping represents the shopping-list app: a single Go binary (Connect API
// + embedded React frontend) backed by SQLite on a Longhorn PVC, reachable
// only over Tailscale - same exposure pattern as Workouts/Private/Longhorn.
type Shopping struct {
	pulumi.ResourceState

	Namespace   pulumi.StringOutput
	ServiceName pulumi.StringOutput

	redirect ingress.RedirectRoute
}

// TailscaleRedirect returns this app's Cloudflare-redirect data - see
// Private.TailscaleRedirect's doc comment for why this only hands back data
// rather than applying anything itself.
func (s *Shopping) TailscaleRedirect() ingress.RedirectRoute {
	return s.redirect
}

// ShoppingArgs contains the configuration for Shopping.
type ShoppingArgs struct {
	// Namespace is created centrally by pkg/deploy/namespaces.go
	// (ShoppingNamespace) and passed in here - this component does not
	// create it.
	Namespace pulumi.StringInput

	// StorageClassName is longhorn.NewLonghorn's DefaultStorageClass
	// output - the PVC below binds against it.
	StorageClassName pulumi.StringInput

	// TailscaleOperatorNamespace/TailscaleMagicDNSSuffix/CloudflareZoneID/
	// CloudflareBaseDomain/CloudflareProvider are threaded straight through
	// to ingress.NewIngress - see PrivateArgs' doc comments for each.
	TailscaleOperatorNamespace pulumi.StringInput
	TailscaleMagicDNSSuffix    pulumi.StringInput
	CloudflareZoneID           pulumi.StringInput
	CloudflareBaseDomain       pulumi.StringInput
	CloudflareProvider         *cloudflare.Provider

	// GHCRUsername/GHCRToken authenticate BuildShoppingImage's push.
	GHCRUsername string
	GHCRToken    string
}

// NewShopping builds the shopping image, provisions its PVC, and deploys it
// behind a Tailscale-only waypoint - same shape as NewWorkouts, including the
// PVC and the Recreate deployment strategy a single-writer ReadWriteOnce
// volume needs (see the Deployment's Strategy field below for why).
func NewShopping(ctx *pulumi.Context, name string, args *ShoppingArgs, opts ...pulumi.ResourceOption) (*Shopping, error) {
	shopping := &Shopping{}

	err := ctx.RegisterComponentResource("homelab:applications:shopping", name, shopping, opts...)
	if err != nil {
		return nil, err
	}

	localOpts := append(opts, pulumi.Parent(shopping))

	labels := pulumi.StringMap{"app": pulumi.String(name)}

	image, err := BuildShoppingImage(ctx, fmt.Sprintf("%s-image", name), args.GHCRUsername, args.GHCRToken, localOpts...)
	if err != nil {
		return nil, err
	}
	imageOpts := append(append([]pulumi.ResourceOption{}, localOpts...), pulumi.DependsOn([]pulumi.Resource{image}))

	// 1. Dedicated ServiceAccount for this app.
	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 2. A standalone PVC for the SQLite file - ReadWriteOnce since only
	// one pod ever writes it (single replica, Recreate strategy below).
	pvc, err := corev1.NewPersistentVolumeClaim(ctx, fmt.Sprintf("%s-data", name), &corev1.PersistentVolumeClaimArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(fmt.Sprintf("%s-data", name)),
			Namespace: args.Namespace.ToStringPtrOutput(),
		},
		Spec: &corev1.PersistentVolumeClaimSpecArgs{
			AccessModes:      pulumi.StringArray{pulumi.String("ReadWriteOnce")},
			StorageClassName: args.StorageClassName,
			Resources: &corev1.VolumeResourceRequirementsArgs{
				Requests: pulumi.StringMap{"storage": pulumi.String("1Gi")},
			},
		},
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	const dataVolumeName = "data"

	// 3. Deploy the backend. Strategy is Recreate, not the default
	// RollingUpdate: with a single-replica ReadWriteOnce PVC, RollingUpdate
	// can deadlock - the new pod can't schedule/mount until the old one
	// releases the volume, but the old pod isn't torn down until the new
	// one is Ready.
	deployment, err := appsv1.NewDeployment(ctx, fmt.Sprintf("%s-deployment", name), &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
			Labels:    labels,
		},
		Spec: &appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Strategy: &appsv1.DeploymentStrategyArgs{
				Type: pulumi.String("Recreate"),
			},
			Selector: &metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{Labels: labels},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
					// fsGroup chowns the PVC mount to this GID on attach -
					// without it the mount stays root-owned and the
					// non-root container (see shoppingUID's doc comment)
					// can't write its SQLite file at all.
					SecurityContext: &corev1.PodSecurityContextArgs{
						FsGroup: pulumi.Int(shoppingUID),
					},
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String(name),
							Image: image.Ref,
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{Name: pulumi.String("DB_PATH"), Value: pulumi.String("/data/shopping.db")},
							},
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{ContainerPort: pulumi.Int(shoppingPort)},
							},
							VolumeMounts: corev1.VolumeMountArray{
								&corev1.VolumeMountArgs{
									Name:      pulumi.String(dataVolumeName),
									MountPath: pulumi.String("/data"),
								},
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("200m"),
									"memory": pulumi.String("128Mi"),
								},
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("25m"),
									"memory": pulumi.String("32Mi"),
								},
							},
						},
					},
					Volumes: corev1.VolumeArray{
						&corev1.VolumeArgs{
							Name: pulumi.String(dataVolumeName),
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSourceArgs{
								ClaimName: pvc.Metadata.Name().Elem(),
							},
						},
					},
				},
			},
		},
	}, append(append([]pulumi.ResourceOption{}, imageOpts...), pulumi.DependsOn([]pulumi.Resource{pvc}))...)
	if err != nil {
		return nil, err
	}

	// 4. Dedicated waypoint for this app's Service, opted into Tailscale's
	// access policy - same as Workouts/Private.
	wp, err := waypoint.NewWaypoint(ctx, fmt.Sprintf("%s-waypoint", name), &waypoint.WaypointArgs{
		Namespace: args.Namespace,
		Labels: pulumi.StringMap{
			tailscale.WaypointAccessLabelKey: pulumi.String(tailscale.WaypointAccessLabelValue),
		},
		TargetLabels: labels,
	}, localOpts...)
	if err != nil {
		return nil, err
	}

	// 5. Expose it via a Service, routed through the waypoint above.
	service, err := corev1.NewService(ctx, fmt.Sprintf("%s-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(name),
			Namespace: args.Namespace.ToStringPtrOutput(),
			Labels: pulumi.StringMap{
				"istio.io/use-waypoint": wp.Name,
			},
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: labels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{
					Port:       pulumi.Int(80),
					TargetPort: pulumi.Int(shoppingPort),
				},
			},
		},
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{deployment}))...)
	if err != nil {
		return nil, err
	}

	// 6. Put this Service on Tailscale - same as Workouts/Private.
	tsIngress, err := ingress.NewIngress(ctx, name, &ingress.IngressArgs{
		Namespace:            args.Namespace,
		ServiceName:          service.Metadata.Name().Elem(),
		ServicePort:          80,
		Hostname:             name,
		OperatorNamespace:    args.TailscaleOperatorNamespace,
		MagicDNSSuffix:       args.TailscaleMagicDNSSuffix,
		CloudflareZoneID:     args.CloudflareZoneID,
		CloudflareBaseDomain: args.CloudflareBaseDomain,
		CloudflareProvider:   args.CloudflareProvider,
	}, append(localOpts, pulumi.DependsOn([]pulumi.Resource{service, wp}))...)
	if err != nil {
		return nil, err
	}

	shopping.Namespace = args.Namespace.ToStringOutput()
	shopping.ServiceName = service.Metadata.Name().Elem()
	shopping.redirect = tsIngress.Redirect

	if err := ctx.RegisterResourceOutputs(shopping, pulumi.Map{
		"namespace":   shopping.Namespace,
		"serviceName": shopping.ServiceName,
	}); err != nil {
		return nil, err
	}

	return shopping, nil
}
