// Package groupcontroller implements the Reconciler that keeps a Group's
// Status.MissingLights in sync with which of its Spec.Lights currently
// resolve to a real Light CR.
package groupcontroller

import (
	"context"
	"slices"

	lightsv1alpha1 "github.com/liamawhite/lights-controller/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler is a controller-runtime, watch-driven reconciler for Group: it
// fires on every Group create/update and, via the manager's periodic resync
// relist (see cmd/lights-controller's --resync-period), re-checks each
// Spec.Lights entry against the live set of Light CRs even with no new Group
// edit - which is what catches a referenced Light being deleted later. No
// Poller/EventConsumer needed - Group has no external (bridge-side) state to
// sync from, unlike Light/Switch.
type Reconciler struct {
	Client client.Client
}

var _ reconcile.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var group lightsv1alpha1.Group
	if err := r.Client.Get(ctx, req.NamespacedName, &group); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	existing := make(map[string]bool, len(group.Spec.Lights))
	for _, name := range group.Spec.Lights {
		var light lightsv1alpha1.Light
		if err := r.Client.Get(ctx, client.ObjectKey{Name: name}, &light); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			continue
		}
		existing[name] = true
	}

	missing := missingLights(group.Spec.Lights, existing)
	if slices.Equal(group.Status.MissingLights, missing) {
		return ctrl.Result{}, nil
	}

	group.Status.MissingLights = missing
	group.Status.LastSynced = metav1.Now()
	if err := r.Client.Status().Update(ctx, &group); err != nil {
		logger.Error(err, "failed to update group status", "group", group.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
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
