// Package applications groups the Pulumi deploy-time wiring for
// applications/lights-controller's two binaries: building the one shared
// image they both run from, and deploying hub-controller and
// lights-controller against it. The controllers' own component
// implementations stay in pkg/components/{hubcontroller,lightscontroller}
// (same convention as every other component in this repo); this file only
// groups the Hue-specific orchestration that pkg/deploy/deploy.go would
// otherwise have spread inline across itself alongside every unrelated
// component.
package applications

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liamawhite/homelab/pkg/components/hubcontroller"
	"github.com/liamawhite/homelab/pkg/components/lightscontroller"
	"github.com/liamawhite/homelab/pkg/config"
	lightsv1alpha1 "github.com/liamawhite/homelab/pkg/crds/lights/crds/kubernetes/lights/v1alpha1"
	"github.com/pulumi/pulumi-docker-build/sdk/go/dockerbuild"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumitime "github.com/pulumiverse/pulumi-time/sdk/go/time"
)

const lightsControllerImageName = "ghcr.io/liamawhite/lights-controller"

// defaultGroups are the Group CRs InstallLights seeds with an empty
// membership list - just the named rooms/areas known to exist today.
// Which Lights actually belong to each is left for kubectl edit
// afterward (see createDefaultGroups' IgnoreChanges), the same "declare
// the identity, let something else own the mutable content" split
// Switch.Spec.Bindings already relies on.
var defaultGroups = []string{"living-space", "main-bedroom", "front-office", "external-office"}

// BuildLightsControllerImage builds and pushes the ONE shared image
// containing both applications/lights-controller/cmd/lights-controller
// and .../cmd/hub-controller, via buildx/BuildKit (through the Pulumi
// Docker Build provider's embedded client) as part of `homelab
// up`/`preview` itself - no separate manual `make docker-push` step, and
// no dependency on the operator's local `docker login` state:
// ghcrUsername/ghcrToken (infraCfg.GHCR, a PAT with write:packages scope)
// authenticate the push declaratively, the same way every other
// credential in this repo comes from infra.yaml rather than ambient host
// state.
//
// Built once here (rather than inside each of pkg/components/
// hubcontroller and pkg/components/lightscontroller separately) and its
// Ref passed into both: the two binaries share almost their entire
// codebase, so building/pushing two separately-tagged images would just
// double the build time and registry bookkeeping (including GHCR's
// per-package visibility setting) for no benefit - what actually
// separates the two controllers at runtime is their Deployment
// (ServiceAccount, RBAC, hostNetwork, command:), not the image.
//
// Context/Dockerfile locations must be absolute paths, not relative ones:
// the docker-build provider runs as its own out-of-process plugin with its
// own working directory (a Pulumi Automation API temp dir), which is NOT
// this CLI's cwd - confirmed live, a relative Dockerfile path here 404s
// against that plugin's temp dir instead of the repo. os.Getwd() here is
// safe because this component's own code (unlike the provider plugin)
// runs in the same process as the CLI, whose cwd is the repo root by
// convention (same assumption cli/cmd/pulumi/pulumi.go's .pulumi-state
// resolution already makes). The repo root is also required as the build
// CONTEXT (not just where Dockerfile lives): applications/lights-controller
// /go.mod's local "replace" directive needs the root module's source tree
// present at the same relative offset the Dockerfile expects - see that
// Dockerfile's own top comment for why.
func BuildLightsControllerImage(ctx *pulumi.Context, name, ghcrUsername, ghcrToken string, opts ...pulumi.ResourceOption) (*dockerbuild.Image, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve working directory: %w", err)
	}

	// A time.Static Pulumi resource, not Go's time.Now(): time.Now()
	// called directly in program code re-evaluates on every single
	// preview/up invocation, not just when something actually changed -
	// that would make a preview's plan diverge from what the following up
	// actually does, and make every up non-idempotent (a new tag, forcing
	// an image rebuild+replace, on every single run even with zero source
	// changes). A Pulumi resource's value is computed once and persisted
	// in state, staying stable across subsequent runs unless deliberately
	// reset (delete this resource from state to force a new timestamp).
	buildTime, err := pulumitime.NewStatic(ctx, fmt.Sprintf("%s-build-time", name), nil, opts...)
	if err != nil {
		return nil, err
	}

	// RFC3339 (e.g. "2026-07-20T15:49:43Z") contains colons, invalid in a
	// Docker tag - strip punctuation down to a sortable, tag-safe string.
	buildTag := buildTime.Rfc3339.ApplyT(func(ts string) string {
		return strings.NewReplacer(":", "", "-", "").Replace(ts)
	}).(pulumi.StringOutput)

	return dockerbuild.NewImage(ctx, name, &dockerbuild.ImageArgs{
		Context: &dockerbuild.BuildContextArgs{
			Location: pulumi.String(repoRoot),
		},
		Dockerfile: &dockerbuild.DockerfileArgs{
			Location: pulumi.String(filepath.Join(repoRoot, "applications/lights-controller/Dockerfile")),
		},
		Platforms: dockerbuild.PlatformArray{
			dockerbuild.Platform_Linux_arm64,
		},
		Push: pulumi.Bool(true),
		Registries: dockerbuild.RegistryArray{
			&dockerbuild.RegistryArgs{
				Address:  pulumi.String("ghcr.io"),
				Username: pulumi.StringPtr(ghcrUsername),
				Password: pulumi.StringPtr(ghcrToken),
			},
		},
		// Tags: the stable per-deployment-lineage timestamp above for a
		// unique, distinguishable entry in the GHCR history, plus "latest"
		// as a convenience pointer for manual pulls. A true content-hash
		// tag isn't possible here - the digest (available as this same
		// resource's own Digest/ContextHash *outputs*) is only known
		// after the push completes, and a resource can't take its own
		// output as an input. A git-SHA tag was considered instead (this
		// repo's legacy TS side already does this for its own custom
		// images, per .versions.ts), but would reuse the same tag across
		// builds made from an uncommitted working tree - the normal case
		// while iterating - defeating "unique per build". Deploy
		// correctness doesn't depend on any of this anyway: both
		// Deployments use image.Ref, which is already digest-pinned
		// regardless of tag naming.
		Tags: pulumi.StringArray{
			pulumi.String(lightsControllerImageName + ":latest"),
			pulumi.Sprintf("%s:%s", lightsControllerImageName, buildTag),
		},
	}, opts...)
}

// LightsArgs is what InstallLights needs to build the shared image and
// deploy both hub-controller and lights-controller against it.
type LightsArgs struct {
	// Namespace is created centrally by pkg/deploy/namespaces.go and
	// passed in here - neither component creates it.
	Namespace pulumi.StringInput
	// Bridges is infraCfg.Lights.Hue.Bridges - every paired bridge's id
	// and application key.
	Bridges []config.HueBridgeConfig
	// GHCRUsername/GHCRToken authenticate BuildLightsControllerImage's push.
	GHCRUsername string
	GHCRToken    string
	// HubPollInterval/LightsPollInterval are each controller's own full
	// poll-sweep cadence - see their respective Args' doc comments.
	HubPollInterval    pulumi.StringInput
	LightsPollInterval pulumi.StringInput
	// DryRun controls whether the Light reconciler enacts spec changes
	// against the bridge (false) or only logs drift (true).
	DryRun pulumi.BoolInput
}

// Lights groups the two Hue-related deployments and the one shared image
// they both run.
type Lights struct {
	HubController    *hubcontroller.HubController
	LightsController *lightscontroller.LightsController
}

// InstallLights builds the shared lights-controller/hub-controller image
// once, then deploys both components against it. hub-controller has no
// DependsOn on lights-controller or vice versa: they only interact at
// runtime via the Kubernetes API (a missing/unreachable HueBridge is
// handled gracefully), not at deploy time - opts is the common set of
// dependencies both need (crds.Lights, the lights namespace, etc.), with
// the built image added on top of it for each.
//
// Resource names below ("lights-controller-image", "hub-controller",
// "lights-controller") are the exact, unchanged names deploy.go used to
// register these directly - Pulumi identifies a resource by its URN
// (parent name + these child names), so renaming them here would delete
// and recreate every real Deployment/RBAC object/HueBridge CR underneath
// for no reason.
func InstallLights(ctx *pulumi.Context, args *LightsArgs, opts ...pulumi.ResourceOption) (*Lights, error) {
	image, err := BuildLightsControllerImage(ctx, "lights-controller-image", args.GHCRUsername, args.GHCRToken, opts...)
	if err != nil {
		return nil, err
	}
	imageOpts := append([]pulumi.ResourceOption{pulumi.DependsOn([]pulumi.Resource{image})}, opts...)

	hub, err := hubcontroller.NewHubController(ctx, "hub-controller", &hubcontroller.HubControllerArgs{
		Namespace:    args.Namespace,
		Bridges:      args.Bridges,
		PollInterval: args.HubPollInterval,
		Image:        image.Ref,
	}, imageOpts...)
	if err != nil {
		return nil, err
	}

	lc, err := lightscontroller.NewLightsController(ctx, "lights-controller", &lightscontroller.LightsControllerArgs{
		Namespace:    args.Namespace,
		Bridges:      args.Bridges,
		PollInterval: args.LightsPollInterval,
		DryRun:       args.DryRun,
		Image:        image.Ref,
	}, imageOpts...)
	if err != nil {
		return nil, err
	}

	if err := createDefaultGroups(ctx, opts...); err != nil {
		return nil, err
	}

	return &Lights{HubController: hub, LightsController: lc}, nil
}

// createDefaultGroups seeds each of defaultGroups with an empty Lights
// list if it doesn't already exist. IgnoreChanges("spec") means this only
// ever sets Spec on first creation - populating a group's actual Light
// names is left to kubectl edit, and is never reverted by a later `up`.
func createDefaultGroups(ctx *pulumi.Context, opts ...pulumi.ResourceOption) error {
	for _, name := range defaultGroups {
		groupOpts := append([]pulumi.ResourceOption{pulumi.IgnoreChanges([]string{"spec"})}, opts...)
		_, err := lightsv1alpha1.NewGroup(ctx, fmt.Sprintf("group-%s", name), &lightsv1alpha1.GroupArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String(name),
			},
			Spec: &lightsv1alpha1.GroupSpecArgs{
				Lights: pulumi.StringArray{},
			},
		}, groupOpts...)
		if err != nil {
			return fmt.Errorf("failed to create group %q: %w", name, err)
		}
	}
	return nil
}
