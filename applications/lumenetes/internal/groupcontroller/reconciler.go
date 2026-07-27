// Package groupcontroller implements the Reconciler that keeps a Group's
// Status.MissingLights/LightCount in sync with Spec.Lights, and enacts
// Spec.ActiveScene onto the group's target Lights' Spec.
package groupcontroller

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/circadian"
	"github.com/liamawhite/lumenetes/internal/sun"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler is a controller-runtime, watch-driven reconciler for Group: it
// fires on every Group create/update and, via the manager's periodic resync
// relist (see cmd/lumenetes-controller's --resync-period), re-checks each
// Spec.Lights entry against the live set of Light CRs even with no new Group
// edit - which is what catches a referenced Light being deleted later. It
// also re-enacts Spec.ActiveScene on every reconcile for the same reason:
// enforcement is deliberately continuous, not gated on ActiveScene having
// changed, so a light manually overridden while a scene/schedule/off is
// selected gets corrected back on the next resync. No Poller/EventConsumer
// needed - Group has no external (bridge-side) state to sync from, unlike
// Light/Switch, and enactment here is a plain Get-then-Update on each
// target Light's Spec - no bridge/hue dependency at all. internal/
// lightscontroller.Reconciler does the actual bridge push from there,
// unchanged.
type Reconciler struct {
	Client client.Client
	// Now returns the current time - nil-safe, defaults to time.Now.
	// Overridden in tests for a deterministic "now" relative to a
	// CircadianSchedule's computed sun times (Latitude/Longitude live on
	// the CircadianSchedule itself, not here - see enactCircadianSchedule).
	Now func() time.Time
}

var _ reconcile.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var group lumenetesv1alpha1.Group
	if err := r.Client.Get(ctx, req.NamespacedName, &group); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	found := make([]bool, len(group.Spec.Lights))
	var existsGroup errgroup.Group
	for i, name := range group.Spec.Lights {
		existsGroup.Go(func() error {
			var light lumenetesv1alpha1.Light
			if err := r.Client.Get(ctx, client.ObjectKey{Name: name}, &light); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			found[i] = true
			return nil
		})
	}
	if err := existsGroup.Wait(); err != nil {
		return ctrl.Result{}, err
	}
	existing := make(map[string]bool, len(group.Spec.Lights))
	for i, name := range group.Spec.Lights {
		if found[i] {
			existing[name] = true
		}
	}

	missing := missingLights(group.Spec.Lights, existing)
	count := int32(len(group.Spec.Lights))

	sceneErr, enactErr := r.enactActiveScene(ctx, logger, &group)
	if enactErr != nil {
		logger.Error(enactErr, "failed to enact active scene", "group", group.Name, "activeScene", fmt.Sprintf("%+v", group.Spec.ActiveScene))
	}

	if slices.Equal(group.Status.MissingLights, missing) && group.Status.LightCount == count && group.Status.ActiveSceneError == sceneErr {
		return ctrl.Result{}, enactErr
	}

	group.Status.MissingLights = missing
	group.Status.LightCount = count
	group.Status.ActiveSceneError = sceneErr
	group.Status.LastSynced = metav1.Now()
	if err := r.Client.Status().Update(ctx, &group); err != nil {
		logger.Error(err, "failed to update group status", "group", group.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, enactErr
}

// missingLights returns the subset of specLights not present in existing,
// preserving specLights' order. Plain, dependency-free function so it's
// unit-testable without envtest.
func missingLights(specLights []string, existing map[string]bool) []string {
	var missing []string
	for _, name := range specLights {
		if !existing[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// enactActiveScene enforces group.Spec.ActiveScene onto its target Lights'
// Spec. It returns the string to record in Status.ActiveSceneError (empty
// unless ActiveScene names a referent that's missing, targets a different
// group, or fails to interpolate), and a Go error only for failures that
// should trigger controller-runtime's requeue-with-backoff. A single broken
// light doesn't abort enactment for the rest of the group - per-light
// failures are logged and skipped, with the first one returned so
// Reconcile still requeues.
func (r *Reconciler) enactActiveScene(ctx context.Context, logger logr.Logger, group *lumenetesv1alpha1.Group) (string, error) {
	ref := group.Spec.ActiveScene
	if ref == nil {
		return "", nil
	}
	switch ref.Kind {
	case lumenetesv1alpha1.ActiveSceneKindOff:
		return "", r.enactOff(ctx, logger, group)
	case lumenetesv1alpha1.ActiveSceneKindReactive:
		return "", r.enactReactive(ctx, logger, group)
	case lumenetesv1alpha1.ActiveSceneKindCircadianSchedule:
		return r.enactCircadianSchedule(ctx, logger, group, ref.Name)
	case lumenetesv1alpha1.ActiveSceneKindScene, "":
		return r.enactScene(ctx, logger, group, ref.Name)
	default:
		return fmt.Sprintf("unknown activeScene kind %q", ref.Kind), nil
	}
}

// enactOff turns off (Spec.On = false - brightness/color/colorTempK are
// left as they are) and clears Spec.Reactive on every light in
// group.Spec.Lights, in parallel - a group's lights are independent
// Kubernetes objects with no ordering requirement between them, and the
// shared controller-runtime client is safe for concurrent use (it already
// serves MaxConcurrentReconciles>1 elsewhere in this binary). Clearing
// Reactive here matters even though Off itself has nothing to do with
// reactivity: a light previously owned by a Reactive-mode Group must not
// stay invisible to internal/lightscontroller.Reconciler once this Group
// takes over with Off - see LightSpec.Reactive's doc comment for why every
// non-Reactive enactment path is responsible for clearing it. One light's
// failure doesn't stop the rest: every goroutine runs to completion
// regardless of its siblings, and the first error is returned so Reconcile
// still requeues.
func (r *Reconciler) enactOff(ctx context.Context, logger logr.Logger, group *lumenetesv1alpha1.Group) error {
	var g errgroup.Group
	for _, name := range group.Spec.Lights {
		g.Go(func() error {
			var light lumenetesv1alpha1.Light
			if err := r.Client.Get(ctx, client.ObjectKey{Name: name}, &light); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				logger.Error(err, "failed to get light to turn off", "group", group.Name, "light", name)
				return err
			}
			if !light.Spec.On && !light.Spec.Reactive {
				return nil
			}
			light.Spec.On = false
			light.Spec.Reactive = false
			if err := r.Client.Update(ctx, &light); err != nil {
				logger.Error(err, "failed to turn off light", "group", group.Name, "light", name)
				return err
			}
			return nil
		})
	}
	return g.Wait()
}

// enactReactive mirrors each light in group.Spec.Lights' own observed
// Status back onto its Spec, and sets Spec.Reactive - the inverse of every
// other ActiveScene kind, which push a computed target down.
// internal/lightscontroller.Reconciler checks Spec.Reactive itself before
// ever enacting (see LightSpec.Reactive's doc comment) rather than this
// function racing to mirror Status before lightscontroller's own reconcile
// of the same Light change fires - setting the flag is what actually stops
// the fight, not the mirroring itself.
func (r *Reconciler) enactReactive(ctx context.Context, logger logr.Logger, group *lumenetesv1alpha1.Group) error {
	var g errgroup.Group
	for _, name := range group.Spec.Lights {
		g.Go(func() error {
			if err := r.applyReactiveLightState(ctx, name); err != nil {
				logger.Error(err, "failed to apply reactive light state", "group", group.Name, "light", name)
				return err
			}
			return nil
		})
	}
	return g.Wait()
}

// applyReactiveLightState copies the named Light's own Status onto its
// Spec and sets Spec.Reactive = true - On/Brightness/Color/ColorTempK
// unconditionally, no capability-sentinel special-casing needed since
// Status already carries the same sentinel convention Spec would
// (Brightness==-1/Color==""/ColorTempK==0 - see LightStatus's doc
// comment). Spec.Name is deliberately left alone: renaming is a separate
// concern from "light state". A currently-unreachable light's Status is
// stale (last known before it went unreachable, per LightStatus.Reachable's
// doc comment) and isn't propagated - this self-heals on the next
// reconcile once reachable again. Note this means Spec.Reactive itself
// also doesn't get set on an unreachable light until it's reachable again -
// acceptable, since lightscontroller.Reconciler already refuses to enact
// against an unreachable light's stale Status regardless (see its own
// Reachable check).
func (r *Reconciler) applyReactiveLightState(ctx context.Context, name string) error {
	var light lumenetesv1alpha1.Light
	if err := r.Client.Get(ctx, client.ObjectKey{Name: name}, &light); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if !light.Status.Reachable {
		return nil
	}
	next := light.Spec
	next.On = light.Status.On
	next.Brightness = light.Status.Brightness
	next.Color = light.Status.Color
	next.ColorTempK = light.Status.ColorTempK
	next.Reactive = true
	if next == light.Spec {
		return nil
	}
	light.Spec = next
	return r.Client.Update(ctx, &light)
}

// enactScene resolves name as a Scene, validates it targets this group,
// and applies each of its Lights entries that's a member of
// group.Spec.Lights onto the target Light's Spec, in parallel (see
// enactOff's doc comment for why concurrent per-light writes are safe
// here). Membership is re-derived directly against group.Spec.Lights
// rather than trusting Scene.Status.InvalidLights, which can be stale
// (Scene's own reconciler may not have caught up yet).
func (r *Reconciler) enactScene(ctx context.Context, logger logr.Logger, group *lumenetesv1alpha1.Group, name string) (string, error) {
	var scene lumenetesv1alpha1.Scene
	if err := r.Client.Get(ctx, client.ObjectKey{Name: name}, &scene); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Sprintf("scene %q not found", name), nil
		}
		return "", err
	}
	if scene.Spec.Group != group.Name {
		return fmt.Sprintf("scene %q targets group %q, not %q", scene.Name, scene.Spec.Group, group.Name), nil
	}

	members := make(map[string]bool, len(group.Spec.Lights))
	for _, memberName := range group.Spec.Lights {
		members[memberName] = true
	}

	var g errgroup.Group
	for _, state := range scene.Spec.Lights {
		if !members[state.Name] {
			continue
		}
		g.Go(func() error {
			if err := r.applySceneLightState(ctx, state); err != nil {
				logger.Error(err, "failed to apply scene light state", "group", group.Name, "scene", scene.Name, "light", state.Name)
				return err
			}
			return nil
		})
	}
	return "", g.Wait()
}

// enactCircadianSchedule resolves name as a CircadianSchedule, validates it
// targets this group, interpolates its current output for r.now(), and
// applies that (brightness, colorTempK) pair uniformly to every light in
// group.Spec.Lights - reusing applySceneLightState/applySceneStateToSpec
// unchanged via a synthetic SceneLightState per light, so the existing
// per-light capability-sentinel skip (Brightness==-1, ColorTempK==0)
// applies here for free. Never touches CircadianSchedule.Status - that's
// internal/circadianschedulecontroller's job, keeping "a reconciler only
// writes its own resource's Status" consistent.
//
// Also explicitly clears Spec.Color to "" (the existing "not managed"
// sentinel) on every light it touches - a circadian curve only ever
// controls Brightness/ColorTempK, never Color, so without this a light's
// Spec.Color stays wherever it was seeded at Light creation, permanently
// stale once ColorTempK actually starts driving the bridge into color-
// temperature mode. internal/lightscontroller.diffLight would then see
// that stale Spec.Color disagree with the bridge's real (and correct)
// rendered color, "fix" it by pushing the stale value back to the bridge,
// and knock it straight back into xy-color mode - confirmed live: this
// produced a real, repeating ~1s flash to the correct color followed by a
// revert to a stale, wrong one, roughly once per enactment.
func (r *Reconciler) enactCircadianSchedule(ctx context.Context, logger logr.Logger, group *lumenetesv1alpha1.Group, name string) (string, error) {
	var schedule lumenetesv1alpha1.CircadianSchedule
	if err := r.Client.Get(ctx, client.ObjectKey{Name: name}, &schedule); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Sprintf("circadian schedule %q not found", name), nil
		}
		return "", err
	}
	if schedule.Spec.Group != group.Name {
		return fmt.Sprintf("circadian schedule %q targets group %q, not %q", schedule.Name, schedule.Spec.Group, group.Name), nil
	}

	coords := sun.Coordinates{Latitude: schedule.Spec.Latitude, Longitude: schedule.Spec.Longitude}
	brightness, colorTempK, onState, err := circadian.Interpolate(schedule.Spec.Keyframes, coords, r.now())
	if err != nil {
		return fmt.Sprintf("circadian schedule %q: %v", schedule.Name, err), nil
	}

	var on *bool
	switch onState {
	case lumenetesv1alpha1.CircadianOnStateOn:
		onVal := true
		on = &onVal
	case lumenetesv1alpha1.CircadianOnStateOff:
		onVal := false
		on = &onVal
	}

	relinquishColor := ""
	var g errgroup.Group
	for _, lightName := range group.Spec.Lights {
		state := lumenetesv1alpha1.SceneLightState{Name: lightName, On: on, Brightness: &brightness, ColorTempK: &colorTempK, Color: &relinquishColor}
		g.Go(func() error {
			if err := r.applySceneLightState(ctx, state); err != nil {
				logger.Error(err, "failed to apply circadian schedule state", "group", group.Name, "circadianSchedule", schedule.Name, "light", lightName)
				return err
			}
			return nil
		})
	}
	return "", g.Wait()
}

// applySceneLightState applies state to the named Light's Spec - a plain
// spec write, exactly equivalent to a user editing the Light. NotFound is
// tolerated (already surfaced via Status.MissingLights/Scene's own
// InvalidLights), not an error here.
func (r *Reconciler) applySceneLightState(ctx context.Context, state lumenetesv1alpha1.SceneLightState) error {
	var light lumenetesv1alpha1.Light
	if err := r.Client.Get(ctx, client.ObjectKey{Name: state.Name}, &light); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	next := applySceneStateToSpec(light.Spec, state)
	if next == light.Spec {
		return nil
	}
	light.Spec = next
	return r.Client.Update(ctx, &light)
}

const minBrightness, maxBrightness int32 = 0, 100

// applySceneStateToSpec computes current's next LightSpec after state - a
// trimmed adaptation of internal/switchcontroller.applyActionToSpec for
// Scene's absolute-only fields (no Toggle/BrightnessDelta/TargetLights).
// Brightness/Color/ColorTempK are no-ops if the target light doesn't
// support that capability, using the same sentinel convention used
// throughout this codebase (Brightness==-1, Color=="", ColorTempK==0) -
// without this, a Scene entry setting brightness/color on an incapable
// light would durably desync that Light's Spec from Status and spin
// lightscontroller.Reconciler's enact-cooldown loop with a permanent
// EnactError for no reason. Also used by enactCircadianSchedule via a
// synthetic SceneLightState, so Reactive is unconditionally cleared here
// (not gated on any state field) for both callers - either one enacting
// means this Group has taken this light out of Reactive mode, if it was
// ever in it (see LightSpec.Reactive's doc comment).
func applySceneStateToSpec(current lumenetesv1alpha1.LightSpec, state lumenetesv1alpha1.SceneLightState) lumenetesv1alpha1.LightSpec {
	next := current

	if state.On != nil {
		next.On = *state.On
	}
	if state.Brightness != nil && current.Brightness != -1 {
		next.Brightness = clampBrightness(*state.Brightness)
	}
	if state.Color != nil && current.Color != "" {
		next.Color = *state.Color
	}
	if state.ColorTempK != nil && current.ColorTempK != 0 {
		next.ColorTempK = *state.ColorTempK
	}
	next.Reactive = false
	return next
}

func clampBrightness(v int32) int32 {
	if v < minBrightness {
		return minBrightness
	}
	if v > maxBrightness {
		return maxBrightness
	}
	return v
}
