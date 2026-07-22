package lightscontroller

import (
	"context"
	"errors"
	"fmt"

	lighthue "github.com/liamawhite/homelab/pkg/lights/hue"
	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	"github.com/liamawhite/lights-controller/internal/bridges"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler is a controller-runtime, watch-driven reconciler for Light: it
// fires on every create/update - including the Poller's and EventConsumer's
// own frequent Status().Update calls - and, separately, whenever the
// manager's cache does a full relist (see cmd/lights-controller's
// --resync-period / ctrl.Options.Cache.SyncPeriod), which is what makes
// drift get re-noticed periodically even with no new spec edit. It diffs
// Spec (desired) against Status (observed) and, unless DryRun, enacts the
// difference against the physical bridge.
//
// Deliberately separate from Poller/EventConsumer even though all three
// live in this package and act on Light: those two keep Status in sync
// with the physical bridge (Poller via periodic full sweep, EventConsumer
// via the real-time CLIP v2 eventstream - see internal/eventstream); this
// reacts to the watch instead, so spec edits are picked up immediately.
// Both Poller and EventConsumer write bridge-derived Status fields
// (name/on/brightness/color/colorTempK/deviceId/etc.) - by design, from
// two equally-authoritative sources (see EventConsumer's mergeLightStatus
// doc comment) - Reconciler only ever writes its own bookkeeping fields
// (LastEnactAttempt/EnactError). Once enactment succeeds, the eventstream
// pushes the bulb's new state to Status on its own within about a second;
// there's nothing left for Reconciler to actively wait for or trigger.
//
// No GenerationChangedPredicate filter is used: Light has a status
// subresource, so metadata.generation never bumps on status-only writes,
// but filtering on it would also silence the periodic resync's
// re-deliveries (a resync relist re-delivers the same object at the same
// generation purely to force reprocessing) - so Poller's/EventConsumer's
// own writes are accepted as harmless extra Reconcile calls instead.
type Reconciler struct {
	Client  client.Client
	Bridges []bridges.Config
	// DryRun, true by default, means Reconcile only ever logs drift instead
	// of enacting it.
	DryRun bool
}

var _ reconcile.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var light lightsv1alpha1.Light
	if err := r.Client.Get(ctx, req.NamespacedName, &light); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Stale status (bridge unreachable last poll) can't be trusted to diff
	// against - wait for the next successful poll to say anything.
	if !light.Status.Reachable {
		return ctrl.Result{}, nil
	}

	diffs := diffLight(light.Spec, light.Status)
	if len(diffs) == 0 {
		if light.Status.EnactError != "" {
			light.Status.EnactError = ""
			if err := r.Client.Status().Update(ctx, &light); err != nil {
				logger.Error(err, "failed to clear enact error", "light", light.Name)
			}
		}
		return ctrl.Result{}, nil
	}

	if r.DryRun {
		logger.Info("light spec differs from observed state",
			"light", light.Name, "dryRun", r.DryRun, "diffs", diffs)
		return ctrl.Result{}, nil
	}

	enactErr := r.enact(ctx, &light, diffs)

	light.Status.LastEnactAttempt = metav1.Now()
	if enactErr != nil {
		light.Status.EnactError = enactErr.Error()
	} else {
		light.Status.EnactError = ""
	}
	if err := r.Client.Status().Update(ctx, &light); err != nil {
		logger.Error(err, "failed to update enact bookkeeping status", "light", light.Name)
	}

	if enactErr != nil {
		// Return the error itself rather than a custom RequeueAfter - now
		// that there's no cooldown to coordinate with, controller-runtime's
		// own default exponential-backoff requeue is exactly the retry
		// pacing a real failure (bridge unreachable, bad PUT, etc.) wants.
		return ctrl.Result{}, enactErr
	}

	return ctrl.Result{}, nil
}

// resolveBridge returns the current IP and app key for bridgeID, or an
// error if the bridge isn't currently known-reachable or isn't paired.
func (r *Reconciler) resolveBridge(ctx context.Context, bridgeID string) (ip, appKey string, err error) {
	var hueBridge lightsv1alpha1.HueBridge
	if err := r.Client.Get(ctx, client.ObjectKey{Name: bridges.ResourceName(bridgeID)}, &hueBridge); err != nil {
		return "", "", fmt.Errorf("failed to get HueBridge %s: %w", bridgeID, err)
	}
	if !hueBridge.Status.Reachable || hueBridge.Status.IP == "" {
		return "", "", fmt.Errorf("bridge %s is not currently reachable", bridgeID)
	}
	cfg, ok := bridges.FindByID(r.Bridges, bridgeID)
	if !ok {
		return "", "", fmt.Errorf("no paired bridge config found for %s", bridgeID)
	}
	return hueBridge.Status.IP, cfg.AppKey, nil
}

// enact attempts to push light's Spec to the bridge for the fields listed
// in diffs, returning a combined error if any call failed. It never
// touches light.Status itself - that's the caller's job (bookkeeping
// fields) and the Poller's job (bridge-derived fields, via confirmAndTrigger).
func (r *Reconciler) enact(ctx context.Context, light *lightsv1alpha1.Light, diffs []fieldDiff) error {
	ip, appKey, err := r.resolveBridge(ctx, light.Status.BridgeID)
	if err != nil {
		return err
	}

	var errs []error
	if hasField(diffs, "on") || hasField(diffs, "brightness") || hasField(diffs, "color") || hasField(diffs, "colorTempK") {
		desired := lighthue.UpdateLightState{
			On:         light.Spec.On,
			Brightness: float64(light.Spec.Brightness),
			Color:      light.Spec.Color,
			ColorTempK: int(light.Spec.ColorTempK),
		}
		if err := lighthue.UpdateLight(ctx, ip, appKey, light.Name, desired); err != nil {
			errs = append(errs, fmt.Errorf("light update: %w", err))
		}
	}
	if hasField(diffs, "name") {
		if light.Status.DeviceID == "" {
			errs = append(errs, errors.New("cannot rename: no device id known for this light"))
		} else if err := lighthue.RenameDevice(ctx, ip, appKey, light.Status.DeviceID, light.Spec.Name); err != nil {
			errs = append(errs, fmt.Errorf("device rename: %w", err))
		}
	}
	return errors.Join(errs...)
}

func hasField(diffs []fieldDiff, field string) bool {
	for _, d := range diffs {
		if d.Field == field {
			return true
		}
	}
	return false
}

// fieldDiff is one field where a Light's Spec (desired) differs from its
// Status (observed).
type fieldDiff struct {
	Field string `json:"field"`
	Want  any    `json:"want"`
	Have  any    `json:"have"`
}

// diffLight returns every field where spec and status disagree. Plain,
// dependency-free function so it's unit-testable without envtest. Every
// LightSpec field is always fully seeded from live state at creation (see
// Poller.upsert), so this is a direct field-by-field comparison with no
// zero-value/"unset" special-casing - except color, which compares via
// hue.ColorsMatch's chromaticity-distance tolerance rather than exact
// string equality, since a commanded hex color and the bridge's later-
// reported one never round-trip to an identical string (see that
// function's doc comment).
func diffLight(spec lightsv1alpha1.LightSpec, status lightsv1alpha1.LightStatus) []fieldDiff {
	var diffs []fieldDiff
	if spec.Name != status.Name {
		diffs = append(diffs, fieldDiff{"name", spec.Name, status.Name})
	}
	if spec.On != status.On {
		diffs = append(diffs, fieldDiff{"on", spec.On, status.On})
	}
	if spec.Brightness != status.Brightness {
		diffs = append(diffs, fieldDiff{"brightness", spec.Brightness, status.Brightness})
	}
	if !lighthue.ColorsMatch(spec.Color, status.Color) {
		diffs = append(diffs, fieldDiff{"color", spec.Color, status.Color})
	}
	if spec.ColorTempK != status.ColorTempK {
		diffs = append(diffs, fieldDiff{"colorTempK", spec.ColorTempK, status.ColorTempK})
	}
	return diffs
}
