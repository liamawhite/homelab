package switchcontroller

import (
	"context"
	"time"

	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler is a controller-runtime, watch-driven reconciler for Switch:
// it fires whenever Streamer patches a Switch's Status (a new button
// event) and, if the event is genuinely new, applies every matching
// binding's Action to its target Lights' Spec - a plain Kubernetes write,
// exactly as if a user had run `kubectl edit`. It has no bridge/hue
// dependency at all: internal/lightscontroller.Reconciler does all the
// actual bridge enactment from there, unchanged.
type Reconciler struct {
	Client client.Client
}

var _ reconcile.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var sw lightsv1alpha1.Switch
	if err := r.Client.Get(ctx, req.NamespacedName, &sw); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Stale status (bridge unreachable last poll) can't be trusted - wait
	// for the next successful poll to say anything.
	if !sw.Status.Reachable {
		return ctrl.Result{}, nil
	}

	if !isNewEvent(sw.Status.LastEventTime.Time, sw.Status.LastHandledEventTime.Time) {
		return ctrl.Result{}, nil
	}

	for _, binding := range sw.Spec.Bindings {
		if binding.Event != sw.Status.LastEvent {
			continue
		}
		for _, lightName := range binding.Action.TargetLights {
			if err := r.applyToLight(ctx, lightName, binding.Action); err != nil {
				logger.Error(err, "failed to apply switch action to light",
					"switch", sw.Name, "light", lightName, "event", sw.Status.LastEvent)
				continue
			}
			logger.Info("applied switch action to light",
				"switch", sw.Name, "light", lightName, "event", sw.Status.LastEvent)
		}
	}

	// Retry with a fresh Get, rather than reusing the in-memory sw from
	// above - this is the mitigation for a real race: Streamer and this
	// Reconciler both do read-then-Status().Update against the same
	// object from different goroutines. If this write simply errored on
	// conflict, controller-runtime's default requeue would re-run this
	// entire Reconcile - including re-applying bindings a second time
	// (a visible glitch for Toggle, a real double-step for
	// BrightnessDelta, which is computed against Spec). Retrying just
	// this bookkeeping write narrows the race to "the write contends,"
	// not "an action gets applied twice."
	handledAt := sw.Status.LastEventTime
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var latest lightsv1alpha1.Switch
		if err := r.Client.Get(ctx, req.NamespacedName, &latest); err != nil {
			return err
		}
		latest.Status.LastHandledEventTime = handledAt
		return r.Client.Status().Update(ctx, &latest)
	})
	return ctrl.Result{}, err
}

// applyToLight applies action to the named Light's Spec - a plain spec
// write, exactly equivalent to a user editing the Light.
func (r *Reconciler) applyToLight(ctx context.Context, lightName string, action lightsv1alpha1.SwitchAction) error {
	var light lightsv1alpha1.Light
	if err := r.Client.Get(ctx, client.ObjectKey{Name: lightName}, &light); err != nil {
		return err
	}
	light.Spec = applyActionToSpec(light.Spec, action)
	return r.Client.Update(ctx, &light)
}

// isNewEvent reports whether lastEvent represents a genuinely new button
// event that hasn't been handled yet - a correctness requirement (not a
// stylistic cooldown choice): distinguishing a brand-new event from a
// repeat Reconcile delivery of an already-handled one.
func isNewEvent(lastEvent, lastHandled time.Time) bool {
	if lastEvent.IsZero() {
		return false
	}
	return lastEvent.After(lastHandled)
}

const minBrightness, maxBrightness int32 = 0, 100

// applyActionToSpec computes current's next LightSpec after action. Toggle
// and BrightnessDelta are computed against current (the light's last
// commanded desired state), not its observed Status, so rapid repeated
// "repeat" events (continuous dimming while a button is held) compound
// responsively without waiting for a bridge round-trip between each step.
//
// Brightness/BrightnessDelta/Color/ColorTempK are all no-ops if the target
// light doesn't support that capability (checked via the same sentinel
// convention used throughout this codebase: Brightness==-1, Color=="",
// ColorTempK==0) - without this, a misconfigured binding pointing a
// brightness/color action at an incapable light would durably desync that
// Light's Spec from Status and spin lightscontroller.Reconciler's
// enact-cooldown loop with a permanent EnactError for no reason.
func applyActionToSpec(current lightsv1alpha1.LightSpec, action lightsv1alpha1.SwitchAction) lightsv1alpha1.LightSpec {
	next := current

	if action.On != nil {
		next.On = *action.On
	}
	if action.Toggle {
		next.On = !next.On
	}
	if action.Brightness != nil && current.Brightness != -1 {
		next.Brightness = clampBrightness(*action.Brightness)
	}
	if action.BrightnessDelta != nil && current.Brightness != -1 {
		next.Brightness = clampBrightness(current.Brightness + *action.BrightnessDelta)
	}
	if action.Color != nil && current.Color != "" {
		next.Color = *action.Color
	}
	if action.ColorTempK != nil && current.ColorTempK != 0 {
		next.ColorTempK = *action.ColorTempK
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
