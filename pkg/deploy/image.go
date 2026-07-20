package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi-docker-build/sdk/go/dockerbuild"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumitime "github.com/pulumiverse/pulumi-time/sdk/go/time"
)

const lightsControllerImageName = "ghcr.io/liamawhite/lights-controller"

// buildLightsControllerImage builds and pushes the ONE shared image
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
// lightscontroller and pkg/components/hubcontroller separately) and its
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
func buildLightsControllerImage(ctx *pulumi.Context, name, ghcrUsername, ghcrToken string, opts ...pulumi.ResourceOption) (*dockerbuild.Image, error) {
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
