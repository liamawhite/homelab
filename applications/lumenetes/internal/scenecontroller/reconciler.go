// Package scenecontroller implements the Reconciler that keeps a Scene's
// Status.InvalidLights in sync with its Spec.Group's membership. Scene
// does no enactment itself - see internal/groupcontroller for that.
package scenecontroller

import (
	"context"
	"slices"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler is a controller-runtime, watch-driven reconciler for Scene: it
// fires on every Scene create/update and, via the manager's periodic
// resync relist, re-checks each Spec.Lights entry against the live Group/
// Light CRs even with no new Scene edit - which is what catches a
// referenced Light or Group being deleted later. No Poller/EventConsumer
// needed - Scene has no external (bridge-side) state to sync from, and no
// enactment responsibility (see internal/groupcontroller.Reconciler).
type Reconciler struct {
	Client client.Client
}

var _ reconcile.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var scene lumenetesv1alpha1.Scene
	if err := r.Client.Get(ctx, req.NamespacedName, &scene); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// groupMembers stays nil (never matches, so every light is reported
	// invalid) if the referenced Group doesn't exist - a Scene targeting
	// a nonexistent Group is a validation failure, not a hard Reconcile
	// error.
	var groupMembers map[string]bool
	var group lumenetesv1alpha1.Group
	if err := r.Client.Get(ctx, client.ObjectKey{Name: scene.Spec.Group}, &group); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	} else {
		groupMembers = make(map[string]bool, len(group.Spec.Lights))
		for _, name := range group.Spec.Lights {
			groupMembers[name] = true
		}
	}

	existingLights := make(map[string]bool, len(scene.Spec.Lights))
	for _, ls := range scene.Spec.Lights {
		var light lumenetesv1alpha1.Light
		if err := r.Client.Get(ctx, client.ObjectKey{Name: ls.Name}, &light); err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			continue
		}
		existingLights[ls.Name] = true
	}

	invalid := invalidLights(scene.Spec.Lights, groupMembers, existingLights)
	if slices.Equal(scene.Status.InvalidLights, invalid) {
		return ctrl.Result{}, nil
	}

	scene.Status.InvalidLights = invalid
	scene.Status.LastSynced = metav1.Now()
	if err := r.Client.Status().Update(ctx, &scene); err != nil {
		logger.Error(err, "failed to update scene status", "scene", scene.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// invalidLights returns the subset of sceneLights whose Name isn't both a
// member of groupMembers and present in existingLights, preserving
// sceneLights' order. Plain, dependency-free function so it's unit-testable
// without envtest. A nil groupMembers (referenced Group not found) means
// every entry is reported invalid.
func invalidLights(sceneLights []lumenetesv1alpha1.SceneLightState, groupMembers map[string]bool, existingLights map[string]bool) []string {
	var invalid []string
	for _, ls := range sceneLights {
		if !groupMembers[ls.Name] || !existingLights[ls.Name] {
			invalid = append(invalid, ls.Name)
		}
	}
	return invalid
}
