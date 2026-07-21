package lightscontroller

import (
	"context"
	"errors"
	"fmt"
	"time"

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
// fires on every create/update - including the Poller's own frequent
// Status().Update calls - and, separately, whenever the manager's cache does
// a full relist (see cmd/lights-controller's --resync-period /
// ctrl.Options.Cache.SyncPeriod), which is what makes drift get re-noticed
// periodically even with no new spec edit. It diffs Spec (desired) against
// Status (observed) and, unless DryRun, enacts the difference against the
// physical bridge.
//
// Deliberately separate from Poller even though both live in this package
// and both act on Light: Poller is a plain ticker keeping Status in sync
// with the physical bridge (bridges can't push events, so that has to stay
// poll-based); this reacts to the watch instead, so spec edits are picked
// up immediately rather than waiting for the next poll. Poller remains the
// sole writer of bridge-derived Status fields (name/on/brightness/color/
// colorTempK/deviceId/etc.) - Reconciler only ever writes its own
// bookkeeping fields (LastEnactAttempt/EnactError) and nudges Poller (via
// TriggerSync) to do the real refresh sooner than its next tick.
//
// No GenerationChangedPredicate filter is used: Light has a status
// subresource, so metadata.generation never bumps on the Poller's
// status-only writes, but filtering on it would also silence the periodic
// resync's re-deliveries (a resync relist re-delivers the same object at
// the same generation purely to force reprocessing) - so the Poller's own
// writes are accepted as harmless extra Reconcile calls instead.
type Reconciler struct {
	Client  client.Client
	Bridges []bridges.Config
	// Cooldown bounds how often Reconcile will attempt enactment for a
	// given Light, regardless of how many times it fires in between - the
	// debounce mechanism. Ignored when DryRun.
	Cooldown time.Duration
	// TriggerSync, if non-nil, is used to ask Poller for an out-of-band
	// refresh of a Light right after a successful enactment attempt (see
	// confirmAndTrigger) instead of waiting for Poller's next regular tick.
	TriggerSync chan<- LightRef
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

	if remaining := remainingCooldown(light.Status.LastEnactAttempt.Time, r.Cooldown, time.Now()); remaining > 0 {
		return ctrl.Result{RequeueAfter: remaining}, nil
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
		logger.Error(enactErr, "failed to enact light spec", "light", light.Name)
		return ctrl.Result{RequeueAfter: r.Cooldown}, nil
	}

	r.confirmAndTrigger(ctx, &light)
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

// confirmAndTrigger waits up to 10s, polling the bridge directly and
// read-only, for light's live state to match Spec, then nudges Poller
// (via TriggerSync) to do the authoritative Status refresh - regardless of
// whether convergence was actually confirmed within that window. This
// never writes Status itself; Poller stays the sole writer of
// bridge-derived fields.
func (r *Reconciler) confirmAndTrigger(ctx context.Context, light *lightsv1alpha1.Light) {
	const confirmTimeout = 10 * time.Second
	const confirmInterval = 1 * time.Second

	confirmCtx, cancel := context.WithTimeout(ctx, confirmTimeout)
	defer cancel()

	if ip, appKey, err := r.resolveBridge(confirmCtx, light.Status.BridgeID); err == nil {
		ticker := time.NewTicker(confirmInterval)
		defer ticker.Stop()
	waitLoop:
		for {
			fetched, err := lighthue.FetchLight(confirmCtx, ip, appKey, light.Name)
			if err == nil && lightMatchesSpec(fetched, light.Spec) {
				break waitLoop
			}
			select {
			case <-confirmCtx.Done():
				break waitLoop
			case <-ticker.C:
			}
		}
	}

	select {
	case r.TriggerSync <- LightRef{BridgeID: light.Status.BridgeID, LightID: light.Name}:
	default:
	}
}

func lightMatchesSpec(l lighthue.Light, spec lightsv1alpha1.LightSpec) bool {
	return l.On == spec.On &&
		int32(l.Brightness) == spec.Brightness &&
		l.Color == spec.Color &&
		int32(l.ColorTempK) == spec.ColorTempK
}

// remainingCooldown returns how much longer to wait before another
// enactment attempt is allowed, given lastAttempt (zero if none has been
// made yet), cooldown, and the current time. Zero or negative means an
// attempt is allowed now.
func remainingCooldown(lastAttempt time.Time, cooldown time.Duration, now time.Time) time.Duration {
	if lastAttempt.IsZero() {
		return 0
	}
	return cooldown - now.Sub(lastAttempt)
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
// zero-value/"unset" special-casing.
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
	if spec.Color != status.Color {
		diffs = append(diffs, fieldDiff{"color", spec.Color, status.Color})
	}
	if spec.ColorTempK != status.ColorTempK {
		diffs = append(diffs, fieldDiff{"colorTempK", spec.ColorTempK, status.ColorTempK})
	}
	return diffs
}
