package applications

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liamawhite/homelab/pkg/components/hubcontroller"
	"github.com/liamawhite/homelab/pkg/components/lumenetescontroller"
	"github.com/liamawhite/homelab/pkg/config"
	lumenetesv1alpha1 "github.com/liamawhite/homelab/pkg/crds/lumenetes/crds/kubernetes/lumenetes/v1alpha1"
	"github.com/pulumi/pulumi-docker-build/sdk/go/dockerbuild"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumitime "github.com/pulumiverse/pulumi-time/sdk/go/time"
)

const lumenetesControllerImageName = "ghcr.io/liamawhite/lumenetes"

// defaultGroups are the Group CRs NewLumenetes seeds - the named rooms/areas
// known to exist today, with each one's exact Light membership declared
// here as the single source of truth (same "declared in code, not
// hand-edited live" convention as infra.yaml's lumenetes.hue.bridges for
// HueBridge). Groups with no known membership yet are left with an empty
// Lights list.
//
// ActiveScene is optional and nil for most groups - see
// createDefaultGroups' doc comment for what nil vs. set means. living-space
// is Pulumi-declared to the circadian schedule seeded below, as the
// deliberate exception: once a group's ActiveScene is meant to be
// permanently driven by code rather than toggled live, declaring it here
// is what makes `up` keep re-asserting it.
var defaultGroups = []struct {
	Name        string
	Lights      []string
	ActiveScene *lumenetesv1alpha1.GroupSpecActiveSceneArgs
}{
	{
		Name:        "living-space",
		ActiveScene: &lumenetesv1alpha1.GroupSpecActiveSceneArgs{Kind: pulumi.String("CircadianSchedule"), Name: pulumi.String("living-space-circadian")},
		Lights: []string{
			"c535e296-856a-4f5b-8d9f-bf0bc51ded05", // Kitchen Dryer
			"a9a33139-495b-4a68-a9b0-c2d377cbda10", // Kitchen Island
			"ba3b815d-96e0-4f49-9527-4c88c9961edc", // Kitchen Island
			"d289f279-7ce4-45a6-bd69-0ad2d8a46786", // Kitchen Microwave
			"0064ad7e-0d1f-478c-a380-330c0e3d5866", // Kitchen Sink
			"af886699-c462-4d60-9ac7-57ea1d46ff91", // Kitchen Sink
			"0fa8827c-6085-4d1e-8e19-295ca11be3a4", // Living Room 1
			"6b1df6d3-d138-42f9-b7e9-d48b5e499500", // Living Room Bedroom
			"00dafbef-8d78-4d58-8651-2f919325d663", // Living Room Hallway
			"bfcfcd15-b673-4a2d-9055-7b929ee2837a", // Living Room TV
			"1082961a-79da-488a-92c5-cd2a4173ca2f", // TV Center
			"c4699e76-6f49-489d-b723-3b0c8986489e", // TV Left
			"4700b1c0-1b13-4f09-82a1-2739b0eadd1c", // TV Right
		},
	},
	{
		Name: "main-bedroom",
		Lights: []string{
			"523d3d33-e24d-4f6f-99d9-204ca10b1f37", // Main Bedroom Ceiling
			"e4c3b4dd-88d6-46ba-894a-be18f020d7e9", // Main Bedroom (Tia)
			"fe7b9822-3b69-4f76-ad7b-cbbbce16df57", // Main Bedroom (Liam)
		},
	},
	{
		Name: "front-office",
		Lights: []string{
			"8f69d609-4faf-4f5b-ae89-eb7f316042ba", // Liam Office Ceiling
			"c5fbf27a-0be3-4d54-8f48-c871471ef8ae", // Desk Lightstrip
		},
	},
	{
		Name: "external-office",
		Lights: []string{
			"0af7838d-8570-44f1-a744-7ef5030334ae", // Pendant Light
			"9fa61050-79bc-4b1e-aa5a-81b85f45458b", // Ceiling Main
			"442d9f3a-c33e-4be2-ab35-d764afadb0a9", // Ceiling Top
			"74090b6f-1035-46b0-a5d7-91dc8e04f892", // Monitor Left
			"b2c9cdfc-9f9b-4729-985c-7d2c9c15dc54", // Monitor Right
		},
	},
	{
		Name: "external-front",
		Lights: []string{
			"a5bfd741-d400-4ee6-8d7f-466209ec8ee2", // Door Lantern
		},
	},
	{
		Name: "back-bedroom",
		Lights: []string{
			"5ed5f32b-cdce-4937-bce4-0d430ed6406e", // Baby Ceiling
		},
	},
}

// defaultCircadianSchedules are the CircadianSchedule CRs NewLumenetes
// seeds - each one's Group/Keyframes declared here as the single source of
// truth, same "declared in code, not hand-edited live" convention as
// defaultGroups. Unlike Group.Spec.ActiveScene (deliberately left for
// live/kubectl toggling - see createDefaultGroups' doc comment), a
// schedule's own fields are just configuration data with no "normal
// runtime usage" reason to drift from what's declared here, so Pulumi
// stays fully authoritative for them, with no IgnoreChanges needed.
// Latitude/Longitude are deliberately NOT declared per-entry here - every
// schedule shares the one infra.yaml-sourced location (this homelab is
// physically in one place), passed in by createDefaultCircadianSchedules
// so infra.yaml stays the single place that value is configured, while
// CircadianScheduleSpec itself still requires it at the CRD level (see
// that field's doc comment for why: a schedule created without one must
// fail validation, not silently compute sun times for Null Island - which
// is exactly what happened, live, before Latitude/Longitude existed on the
// CRD at all).
var defaultCircadianSchedules = []struct {
	Name      string
	Group     string
	Keyframes []lumenetesv1alpha1.CircadianScheduleSpecKeyframesArgs
}{
	{
		Name:  "living-space-circadian",
		Group: "living-space",
		Keyframes: []lumenetesv1alpha1.CircadianScheduleSpecKeyframesArgs{
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(-30), Brightness: pulumi.Int(15), ColorTempK: pulumi.Int(2200)},
			{Anchor: pulumi.String("solarNoon"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(100), ColorTempK: pulumi.Int(6500)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(-120), Brightness: pulumi.Int(85), ColorTempK: pulumi.Int(4000)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(-60), Brightness: pulumi.Int(70), ColorTempK: pulumi.Int(2700)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(60), Brightness: pulumi.Int(30), ColorTempK: pulumi.Int(2000)},
			{Anchor: pulumi.String("solarMidnight"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(5), ColorTempK: pulumi.Int(2000)},
		},
	},
	{
		Name:  "external-office-circadian",
		Group: "external-office",
		Keyframes: []lumenetesv1alpha1.CircadianScheduleSpecKeyframesArgs{
			// Office curve: full-bright/cool for the whole working day
			// rather than peaking briefly at solar noon, since desk work
			// hours don't track the sun the way a living space's usage
			// does.
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(40), ColorTempK: pulumi.Int(3000)},
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(90), Brightness: pulumi.Int(100), ColorTempK: pulumi.Int(5500)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(-90), Brightness: pulumi.Int(100), ColorTempK: pulumi.Int(5500)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(50), ColorTempK: pulumi.Int(3000)},
			{Anchor: pulumi.String("solarMidnight"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(10), ColorTempK: pulumi.Int(2200)},
		},
	},
}

// BuildLumenetesControllerImage builds and pushes the ONE shared image
// containing both applications/lumenetes/cmd/lumenetes-controller
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
// hubcontroller and pkg/components/lumenetescontroller separately) and its
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
// CONTEXT (not just where Dockerfile lives): applications/lumenetes
// /go.mod's local "replace" directive needs the root module's source tree
// present at the same relative offset the Dockerfile expects - see that
// Dockerfile's own top comment for why.
func BuildLumenetesControllerImage(ctx *pulumi.Context, name, ghcrUsername, ghcrToken string, opts ...pulumi.ResourceOption) (*dockerbuild.Image, error) {
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
			Location: pulumi.String(filepath.Join(repoRoot, "applications/lumenetes/Dockerfile")),
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
			pulumi.String(lumenetesControllerImageName + ":latest"),
			pulumi.Sprintf("%s:%s", lumenetesControllerImageName, buildTag),
		},
	}, opts...)
}

// LumenetesArgs is what NewLumenetes needs to build the shared image and
// deploy both hub-controller and lumenetes-controller against it.
type LumenetesArgs struct {
	// Namespace is created centrally by pkg/deploy/namespaces.go and
	// passed in here - neither component creates it.
	Namespace pulumi.StringInput
	// Bridges is infraCfg.Lumenetes.Hue.Bridges - every paired bridge's id
	// and application key.
	Bridges []config.HueBridgeConfig
	// Location is infraCfg.Lumenetes.Location - used to interpolate
	// CircadianSchedule keyframes against real sun position.
	Location config.LocationConfig
	// GHCRUsername/GHCRToken authenticate BuildLumenetesControllerImage's push.
	GHCRUsername string
	GHCRToken    string
	// HubPollInterval/LightsPollInterval are each controller's own full
	// poll-sweep cadence - see their respective Args' doc comments.
	HubPollInterval    pulumi.StringInput
	LightsPollInterval pulumi.StringInput
	// DryRun controls whether the Light reconciler enacts spec changes
	// against the bridge (false) or only logs drift (true).
	DryRun pulumi.BoolInput
	// PrometheusNamespace is "monitoring", created centrally by
	// pkg/deploy/namespaces.go - threaded through to lumenetescontroller's
	// own Prometheus scrape-target wiring (see that package's metrics.go).
	// Not needed by hub-controller, which gets no CiliumClusterwideNetworkPolicy
	// for its own metrics scraping (HostNetwork, outside Cilium's policy
	// baseline entirely).
	PrometheusNamespace pulumi.StringInput
}

// Lumenetes groups the Hue-specific deploy-time wiring: building the one
// shared image applications/lumenetes's two binaries both run
// from, and deploying hub-controller and lumenetes-controller against it.
// The controllers' own component implementations stay in
// pkg/components/{hubcontroller,lumenetescontroller} (same convention as
// every other component in this repo) - this just groups the orchestration
// pkg/deploy/deploy.go would otherwise have spread inline across itself
// alongside every unrelated component.
type Lumenetes struct {
	HubController       *hubcontroller.HubController
	LumenetesController *lumenetescontroller.LumenetesController
}

// NewLumenetes builds the shared lumenetes-controller/hub-controller image
// once, then deploys both components against it. hub-controller has no
// DependsOn on lumenetes-controller or vice versa: they only interact at
// runtime via the Kubernetes API (a missing/unreachable HueBridge is
// handled gracefully), not at deploy time - opts is the common set of
// dependencies both need (crds.Lumenetes, the lumenetes namespace, etc.),
// with the built image added on top of it for each.
func NewLumenetes(ctx *pulumi.Context, args *LumenetesArgs, opts ...pulumi.ResourceOption) (*Lumenetes, error) {
	image, err := BuildLumenetesControllerImage(ctx, "lumenetes-controller-image", args.GHCRUsername, args.GHCRToken, opts...)
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

	lc, err := lumenetescontroller.NewLumenetesController(ctx, "lumenetes-controller", &lumenetescontroller.LumenetesControllerArgs{
		Namespace:           args.Namespace,
		Bridges:             args.Bridges,
		PollInterval:        args.LightsPollInterval,
		DryRun:              args.DryRun,
		Image:               image.Ref,
		PrometheusNamespace: args.PrometheusNamespace,
	}, imageOpts...)
	if err != nil {
		return nil, err
	}

	if err := createDefaultGroups(ctx, opts...); err != nil {
		return nil, err
	}

	if err := createDefaultCircadianSchedules(ctx, args.Location, opts...); err != nil {
		return nil, err
	}

	return &Lumenetes{HubController: hub, LumenetesController: lc}, nil
}

// createDefaultGroups creates (or updates) each of defaultGroups with its
// declared Lights list - Pulumi is authoritative for Spec here, the same
// way it already is for HueBridge, so a later `up` corrects any out-of-band
// edit back to what's declared above rather than ignoring it. spec.activeScene
// is the one field that's conditional per group: when a defaultGroups entry
// leaves ActiveScene nil (the common case), it's left out of GroupSpecArgs
// entirely (an omitted optional field sends nil, not an explicit "", so
// this doesn't force every light off on the next `up`) and
// pulumi.IgnoreChanges pins that down further - kubectl/CLI-set
// activeScene values are meant to survive `up` indefinitely for those
// groups, since selecting a group's active scene is normally runtime
// usage, not drift to correct. When an entry declares ActiveScene (e.g.
// living-space's circadian schedule), that's a deliberate opt-in to the
// opposite: Pulumi becomes authoritative for it too, so `up` re-asserts it
// and a live kubectl override of that specific group gets corrected back.
func createDefaultGroups(ctx *pulumi.Context, opts ...pulumi.ResourceOption) error {
	for _, group := range defaultGroups {
		groupOpts := append([]pulumi.ResourceOption{}, opts...)
		if group.ActiveScene == nil {
			groupOpts = append(groupOpts, pulumi.IgnoreChanges([]string{"spec.activeScene"}))
		}
		spec := &lumenetesv1alpha1.GroupSpecArgs{
			Lights: pulumi.ToStringArray(group.Lights),
		}
		if group.ActiveScene != nil {
			// GroupSpecActiveSceneArgs (the plain value) already satisfies
			// GroupSpecActiveScenePtrInput directly - the generated
			// GroupSpecActiveScenePtr(...) wrapper panics on marshal here,
			// so this deliberately doesn't use it.
			spec.ActiveScene = *group.ActiveScene
		}
		_, err := lumenetesv1alpha1.NewGroup(ctx, fmt.Sprintf("group-%s", group.Name), &lumenetesv1alpha1.GroupArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String(group.Name),
			},
			Spec: spec,
		}, groupOpts...)
		if err != nil {
			return fmt.Errorf("failed to create group %q: %w", group.Name, err)
		}
	}
	return nil
}

// createDefaultCircadianSchedules creates (or updates) each of
// defaultCircadianSchedules with its declared Group/Keyframes plus the one
// shared location - Pulumi is fully authoritative here (see
// defaultCircadianSchedules' doc comment for why, unlike
// Group.Spec.ActiveScene). Selecting one of these as a Group's active
// target is a separate, deliberate step (kubectl-patch
// Group.Spec.ActiveScene to {kind: CircadianSchedule, name: <this
// schedule's name>}) - not done here, same reasoning as ActiveScene being
// excluded from createDefaultGroups.
func createDefaultCircadianSchedules(ctx *pulumi.Context, location config.LocationConfig, opts ...pulumi.ResourceOption) error {
	for _, schedule := range defaultCircadianSchedules {
		_, err := lumenetesv1alpha1.NewCircadianSchedule(ctx, fmt.Sprintf("circadian-schedule-%s", schedule.Name), &lumenetesv1alpha1.CircadianScheduleArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String(schedule.Name),
			},
			Spec: &lumenetesv1alpha1.CircadianScheduleSpecArgs{
				Group:     pulumi.String(schedule.Group),
				Latitude:  pulumi.Float64(location.Latitude),
				Longitude: pulumi.Float64(location.Longitude),
				Keyframes: toKeyframesArray(schedule.Keyframes),
			},
		}, opts...)
		if err != nil {
			return fmt.Errorf("failed to create circadian schedule %q: %w", schedule.Name, err)
		}
	}
	return nil
}

// toKeyframesArray adapts a []CircadianScheduleSpecKeyframesArgs to the
// generated CircadianScheduleSpecKeyframesArrayInput's underlying
// concrete slice type.
func toKeyframesArray(keyframes []lumenetesv1alpha1.CircadianScheduleSpecKeyframesArgs) lumenetesv1alpha1.CircadianScheduleSpecKeyframesArray {
	array := make(lumenetesv1alpha1.CircadianScheduleSpecKeyframesArray, len(keyframes))
	for i, kf := range keyframes {
		array[i] = kf
	}
	return array
}
