// Package groupcontroller implements the Reconciler that keeps a Group's
// Status.MissingLights/LightCount in sync with Spec.Lights, and enacts
// Spec.ActiveScene onto the group's target Lights' Spec.
package groupcontroller

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// offScene is the reserved Spec.ActiveScene value that forces every light
// in a Group off, without needing an actual Scene object to exist for it -
// see GroupSpec.ActiveScene's doc comment. A Scene literally named "off"
// would be inert: this literal is checked before any Scene lookup.
const offScene = "off"

// Reconciler is a controller-runtime, watch-driven reconciler for Group: it
// fires on every Group create/update and, via the manager's periodic resync
// relist (see cmd/lumenetes-controller's --resync-period), re-checks each
// Spec.Lights entry against the live set of Light CRs even with no new Group
// edit - which is what catches a referenced Light being deleted later. It
// also re-enacts Spec.ActiveScene on every reconcile for the same reason:
// enforcement is deliberately continuous, not gated on ActiveScene having
// changed, so a light manually overridden while a scene/off is selected
// gets corrected back on the next resync. No Poller/EventConsumer needed -
// Group has no external (bridge-side) state to sync from, unlike Light/
// Switch, and enactment here is a plain Get-then-Update on each target
// Light's Spec - no bridge/hue dependency at all. internal/
// lightscontroller.Reconciler does the actual bridge push from there,
// unchanged.
type Reconciler struct {
	Client client.Client
}

var _ reconcile.Reconciler = (*Reconciler)(nil)

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
		logger.Error(enactErr, "failed to enact active scene", "group", group.Name, "activeScene", group.Spec.ActiveScene)
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
// unless ActiveScene names a Scene that's missing or targets a different
// group), and a Go error only for failures that should trigger
// controller-runtime's requeue-with-backoff. A single broken light doesn't
// abort enactment for the rest of the group - per-light failures are
// logged and skipped, with the first one returned so Reconcile still
// requeues.
func (r *Reconciler) enactActiveScene(ctx context.Context, logger logr.Logger, group *lumenetesv1alpha1.Group) (string, error) {
	switch group.Spec.ActiveScene {
	case "":
		return "", nil
	case offScene:
		return "", r.enactOff(ctx, logger, group)
	default:
		return r.enactScene(ctx, logger, group)
	}
}

// enactOff turns off (Spec.On = false only - brightness/color/colorTempK
// are left as they are) every light in group.Spec.Lights, in parallel - a
// group's lights are independent Kubernetes objects with no ordering
// requirement between them, and the shared controller-runtime client is
// safe for concurrent use (it already serves MaxConcurrentReconciles>1
// elsewhere in this binary). One light's failure doesn't stop the rest:
// every goroutine runs to completion regardless of its siblings, and the
// first error is returned so Reconcile still requeues.
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
			if !light.Spec.On {
				return nil
			}
			light.Spec.On = false
			if err := r.Client.Update(ctx, &light); err != nil {
				logger.Error(err, "failed to turn off light", "group", group.Name, "light", name)
				return err
			}
			return nil
		})
	}
	return g.Wait()
}

// enactScene resolves group.Spec.ActiveScene as a Scene name, validates it
// targets this group, and applies each of its Lights entries that's a
// member of group.Spec.Lights onto the target Light's Spec, in parallel
// (see enactOff's doc comment for why concurrent per-light writes are
// safe here). Membership is re-derived directly against group.Spec.Lights
// rather than trusting Scene.Status.InvalidLights, which can be stale
// (Scene's own reconciler may not have caught up yet).
func (r *Reconciler) enactScene(ctx context.Context, logger logr.Logger, group *lumenetesv1alpha1.Group) (string, error) {
	var scene lumenetesv1alpha1.Scene
	if err := r.Client.Get(ctx, client.ObjectKey{Name: group.Spec.ActiveScene}, &scene); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Sprintf("scene %q not found", group.Spec.ActiveScene), nil
		}
		return "", err
	}
	if scene.Spec.Group != group.Name {
		return fmt.Sprintf("scene %q targets group %q, not %q", scene.Name, scene.Spec.Group, group.Name), nil
	}

	members := make(map[string]bool, len(group.Spec.Lights))
	for _, name := range group.Spec.Lights {
		members[name] = true
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
// EnactError for no reason.
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
