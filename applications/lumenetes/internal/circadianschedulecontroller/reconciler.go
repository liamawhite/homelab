// Package circadianschedulecontroller implements the Reconciler that keeps
// a CircadianSchedule's Status.CurrentBrightness/CurrentColorTempK/
// ValidationError in sync with its Spec and the current time.
// CircadianSchedule does no enactment itself - see internal/groupcontroller
// for that.
package circadianschedulecontroller

import (
	"context"
	"time"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/circadian"
	"github.com/liamawhite/lumenetes/internal/sun"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler is a controller-runtime, watch-driven reconciler for
// CircadianSchedule: it fires on every CircadianSchedule create/update and,
// via the manager's periodic resync relist, recomputes Status even with no
// new edit - the curve's output moves continuously with time, not just
// with Spec changes. No Poller/EventConsumer needed - CircadianSchedule has
// no external (bridge-side) state to sync from, and no enactment
// responsibility (see internal/groupcontroller.Reconciler, which applies
// this schedule's current output onto Light.Spec when a Group selects it
// via Spec.ActiveScene). Validation and interpolation are delegated
// entirely to internal/circadian.Interpolate's own error rather than
// re-checked by hand here, so this controller and groupcontroller can never
// disagree about whether a given Spec is well-formed.
type Reconciler struct {
	Client client.Client
	// Now returns the current time - nil-safe, defaults to time.Now.
	// Overridden in tests for a deterministic "now" relative to the
	// schedule's own Latitude/Longitude-computed sun times.
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

	var schedule lumenetesv1alpha1.CircadianSchedule
	if err := r.Client.Get(ctx, req.NamespacedName, &schedule); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	brightness, colorTempK, validationErr := r.evaluate(&schedule)

	if statusUnchanged(schedule.Status, brightness, colorTempK, validationErr) {
		return ctrl.Result{}, nil
	}

	schedule.Status.CurrentBrightness = brightness
	schedule.Status.CurrentColorTempK = colorTempK
	schedule.Status.ValidationError = validationErr
	schedule.Status.LastSynced = metav1.Now()
	if err := r.Client.Status().Update(ctx, &schedule); err != nil {
		logger.Error(err, "failed to update circadian schedule status", "circadianSchedule", schedule.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// evaluate computes schedule's current output, or a validation error if
// Spec is malformed. Group emptiness is checked here (a CircadianSchedule-
// only concern - Interpolate has no notion of Group); everything about
// Keyframes' well-formedness is Interpolate's own error.
func (r *Reconciler) evaluate(schedule *lumenetesv1alpha1.CircadianSchedule) (brightness, colorTempK *int32, validationErr string) {
	if schedule.Spec.Group == "" {
		return nil, nil, "spec.group is required"
	}
	coords := sun.Coordinates{Latitude: schedule.Spec.Latitude, Longitude: schedule.Spec.Longitude}
	b, c, _, err := circadian.Interpolate(schedule.Spec.Keyframes, coords, r.now())
	if err != nil {
		return nil, nil, err.Error()
	}
	return &b, &c, ""
}

func statusUnchanged(status lumenetesv1alpha1.CircadianScheduleStatus, brightness, colorTempK *int32, validationErr string) bool {
	return status.ValidationError == validationErr &&
		int32PtrEqual(status.CurrentBrightness, brightness) &&
		int32PtrEqual(status.CurrentColorTempK, colorTempK)
}

func int32PtrEqual(a, b *int32) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
