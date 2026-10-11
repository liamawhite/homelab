package applications

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/components/hubcontroller"
	"github.com/liamawhite/homelab/pkg/components/lumenetescontroller"
	"github.com/liamawhite/homelab/pkg/components/tailscale/ingress"
	"github.com/liamawhite/homelab/pkg/config"
	lumenetesv1alpha1 "github.com/liamawhite/homelab/pkg/crds/lumenetes/crds/kubernetes/lumenetes/v1alpha1"
	"github.com/liamawhite/homelab/pkg/versions"
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// defaultGroups are the Group CRs NewLumenetes seeds - the named rooms/areas
// known to exist today, with each one's exact Light membership declared
// here as the single source of truth (same "declared in code, not
// hand-edited live" convention as infra.yaml's lumenetes.hue.bridges for
// HueBridge). Groups with no known membership yet are left with an empty
// Lights list.
//
// ActiveScene is optional and nil for most groups - see
// createDefaultGroups' doc comment for what nil vs. set means. living-space
// and external-office are Pulumi-declared to their circadian schedules
// seeded below, as the deliberate exception: once a group's ActiveScene is
// meant to be permanently driven by code rather than toggled live,
// declaring it here is what makes `up` keep re-asserting it.
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
		Name:        "external-office",
		ActiveScene: &lumenetesv1alpha1.GroupSpecActiveSceneArgs{Kind: pulumi.String("CircadianSchedule"), Name: pulumi.String("external-office-circadian")},
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
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(-60), Brightness: pulumi.Int(15), ColorTempK: pulumi.Int(2200)},
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(60), ColorTempK: pulumi.Int(3000)},
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(60), Brightness: pulumi.Int(100), ColorTempK: pulumi.Int(4000)},
			{Anchor: pulumi.String("solarNoon"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(100), ColorTempK: pulumi.Int(5000)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(-120), Brightness: pulumi.Int(85), ColorTempK: pulumi.Int(4000)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(-60), Brightness: pulumi.Int(70), ColorTempK: pulumi.Int(2700)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(60), Brightness: pulumi.Int(30), ColorTempK: pulumi.Int(1700)},
			{Anchor: pulumi.String("solarMidnight"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(5), ColorTempK: pulumi.Int(1700)},
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
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(-30), Brightness: pulumi.Int(0), ColorTempK: pulumi.Int(2200)},
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(40), ColorTempK: pulumi.Int(3000), On: pulumi.String("on")},
			{Anchor: pulumi.String("sunrise"), OffsetMinutes: pulumi.Int(60), Brightness: pulumi.Int(100), ColorTempK: pulumi.Int(5500)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(-90), Brightness: pulumi.Int(100), ColorTempK: pulumi.Int(5500)},
			{Anchor: pulumi.String("sunset"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(50), ColorTempK: pulumi.Int(3000)},
			{Anchor: pulumi.String("solarMidnight"), OffsetMinutes: pulumi.Int(0), Brightness: pulumi.Int(0), ColorTempK: pulumi.Int(2200), On: pulumi.String("off")},
		},
	},
}

// LumenetesArgs is what NewLumenetes needs to use the published image and
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
	// Retained for call-site compatibility; publishing runs in GitHub Actions.
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
	// TailscaleOperatorNamespace/TailscaleMagicDNSSuffix/CloudflareZoneID/
	// CloudflareBaseDomain/CloudflareProvider are threaded straight through
	// to lumenetescontroller's own UI exposure (see that package's ui.go) -
	// see PrivateArgs' doc comments for each. Not needed by hub-controller,
	// which serves no UI of its own.
	TailscaleOperatorNamespace pulumi.StringInput
	TailscaleMagicDNSSuffix    pulumi.StringInput
	CloudflareZoneID           pulumi.StringInput
	CloudflareBaseDomain       pulumi.StringInput
	CloudflareProvider         *cloudflare.Provider
}

// Lumenetes groups the Hue-specific deploy-time wiring: building the one
// published image containing lumenetes-controller and hub-controller
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

// TailscaleRedirect returns the embedded read-only web UI's
// Cloudflare-redirect data - see Private.TailscaleRedirect's doc comment for
// why this only hands back data rather than applying anything itself.
func (l *Lumenetes) TailscaleRedirect() ingress.RedirectRoute {
	return l.LumenetesController.TailscaleRedirect()
}

// NewLumenetes deploys both controllers using the same published image.
func NewLumenetes(ctx *pulumi.Context, args *LumenetesArgs, opts ...pulumi.ResourceOption) (*Lumenetes, error) {

	hub, err := hubcontroller.NewHubController(ctx, "hub-controller", &hubcontroller.HubControllerArgs{
		Namespace:    args.Namespace,
		Bridges:      args.Bridges,
		PollInterval: args.HubPollInterval,
		Image:        pulumi.String(versions.LumenetesImage),
	}, opts...)
	if err != nil {
		return nil, err
	}

	lc, err := lumenetescontroller.NewLumenetesController(ctx, "lumenetes-controller", &lumenetescontroller.LumenetesControllerArgs{
		Namespace:                  args.Namespace,
		Bridges:                    args.Bridges,
		PollInterval:               args.LightsPollInterval,
		DryRun:                     args.DryRun,
		Image:                      pulumi.String(versions.LumenetesImage),
		PrometheusNamespace:        args.PrometheusNamespace,
		TailscaleOperatorNamespace: args.TailscaleOperatorNamespace,
		TailscaleMagicDNSSuffix:    args.TailscaleMagicDNSSuffix,
		CloudflareZoneID:           args.CloudflareZoneID,
		CloudflareBaseDomain:       args.CloudflareBaseDomain,
		CloudflareProvider:         args.CloudflareProvider,
	}, opts...)
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
