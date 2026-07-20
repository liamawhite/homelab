package lightscontroller

import (
	"context"

	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler is a controller-runtime, watch-driven reconciler for Light: it
// fires on every create/update - including the Poller's own frequent
// Status().Update calls - and, separately, whenever the manager's cache does
// a full relist (see cmd/lights-controller's --resync-period /
// ctrl.Options.Cache.SyncPeriod), which is what makes the dry-run log below
// reappear periodically even with no new spec edit. It never writes
// anything today: it only diffs Spec (desired) against Status (observed)
// and logs drift.
//
// Deliberately separate from Poller even though both live in this package
// and both act on Light: Poller is a plain ticker keeping Status in sync
// with the physical bridge (bridges can't push events, so that has to stay
// poll-based); this reacts to the watch instead, so spec edits are picked
// up immediately rather than waiting for the next poll.
//
// No GenerationChangedPredicate filter is used: Light has a status
// subresource, so metadata.generation never bumps on the Poller's
// status-only writes, but filtering on it would also silence the periodic
// resync's re-deliveries (a resync relist re-delivers the same object at
// the same generation purely to force reprocessing) - so the Poller's own
// writes are accepted as harmless extra Reconcile calls instead.
type Reconciler struct {
	Client client.Client
	// DryRun, true by default, means Reconcile only ever logs drift.
	// There's no enactment implementation at all yet in this phase - false
	// behaves identically to true today - but the field/flag is plumbed
	// end to end so a future phase can branch on it here without touching
	// main.go or any call sites.
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
		return ctrl.Result{}, nil
	}

	logger.Info("light spec differs from observed state",
		"light", light.Name, "dryRun", r.DryRun, "diffs", diffs)

	// Future enactment hooks in here: when DryRun is false, this is where
	// the differing fields get PUT to the bridge (via pkg/lights/hue)
	// instead of only logged. Not implemented in this phase.

	return ctrl.Result{}, nil
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
