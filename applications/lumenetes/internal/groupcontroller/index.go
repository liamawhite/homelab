package groupcontroller

import (
	"context"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// LightsIndexKey indexes Group by each entry in Spec.Lights, so "which
// Groups reference this Light" is an O(1) cache lookup instead of a full
// List+scan - used by MapLightToGroups for near-instant Group
// reconciliation on Light change. Registered once, in
// cmd/lumenetes-controller/main.go, before either controller starts.
const LightsIndexKey = "spec.lights"

// RegisterIndexes registers LightsIndexKey against the manager's cache.
func RegisterIndexes(ctx context.Context, mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, &lumenetesv1alpha1.Group{}, LightsIndexKey, func(obj client.Object) []string {
		return obj.(*lumenetesv1alpha1.Group).Spec.Lights
	})
}

// MapLightToGroups returns an EnqueueRequestsFromMapFunc handler that, for
// any Light change, enqueues a reconcile request for each Reactive-mode
// Group referencing it.
//
// Deliberately filtered to Reactive-kind Groups only - an earlier version
// of this function enqueued every referencing Group regardless of Kind, on
// the assumption that Reconcile is a cheap no-op for a Group not in
// Reactive mode. That assumption holds for Off/Scene (static targets - once
// converged, genuinely nothing left to write) but not CircadianSchedule:
// its target is a continuous function of time, so re-evaluating it is
// almost never a no-op. Confirmed live: leaving this unfiltered created a
// real feedback loop for a CircadianSchedule-active Group - every bridge
// echo of an enacted change re-triggered this mapping, which re-triggered
// a Group reconcile, which recomputed circadian.Interpolate for a fresh
// "now" (genuinely different within seconds, since these curves can move
// several Kelvin/brightness points per minute) and enacted again,
// generating a rapid, visually-flickering stream of real-but-transient
// bridge writes far faster than --resync-period's already-adequate 1m
// cadence intends for a "gradual" curve. Off/Scene/CircadianSchedule don't
// need sub-minute reactivity to a Light change at all - only Reactive mode
// does (to close its race with lightscontroller.Reconciler - see
// LightSpec.Reactive's doc comment).
func MapLightToGroups(c client.Client) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		light, ok := obj.(*lumenetesv1alpha1.Light)
		if !ok {
			return nil
		}
		var groups lumenetesv1alpha1.GroupList
		if err := c.List(ctx, &groups, client.MatchingFields{LightsIndexKey: light.Name}); err != nil {
			return nil
		}
		var requests []reconcile.Request
		for _, group := range groups.Items {
			ref := group.Spec.ActiveScene
			if ref == nil || ref.Kind != lumenetesv1alpha1.ActiveSceneKindReactive {
				continue
			}
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: group.Name}})
		}
		return requests
	}
}
