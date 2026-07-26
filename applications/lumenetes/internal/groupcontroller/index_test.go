package groupcontroller

import (
	"context"
	"errors"
	"slices"
	"testing"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func newIndexedFakeClient(t *testing.T, objs ...client.Object) client.WithWatch {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithIndex(&lumenetesv1alpha1.Group{}, LightsIndexKey, func(obj client.Object) []string {
			return obj.(*lumenetesv1alpha1.Group).Spec.Lights
		}).
		WithObjects(objs...).
		Build()
}

func TestMapLightToGroups(t *testing.T) {
	// group-a and group-b both reference light-2 and are Reactive - both
	// must be returned. group-c references light-3 but is CircadianSchedule
	// - must NOT be returned (the regression case: an earlier version
	// enqueued every referencing Group regardless of Kind, which fed a
	// live flickering incident for a CircadianSchedule-active Group - see
	// MapLightToGroups' doc comment). group-d references light-3 too, and
	// is unmanaged (nil ActiveScene) - also must not be returned.
	groupA := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "group-a"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"light-1", "light-2"}, ActiveScene: reactiveRef()},
	}
	groupB := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "group-b"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"light-2"}, ActiveScene: reactiveRef()},
	}
	groupC := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "group-c"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"light-3"}, ActiveScene: circadianRef("some-schedule")},
	}
	groupD := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "group-d"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"light-3"}},
	}
	c := newIndexedFakeClient(t, groupA, groupB, groupC, groupD)
	mapFn := MapLightToGroups(c)

	t.Run("light referenced by two Reactive groups", func(t *testing.T) {
		light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light-2"}}
		requests := mapFn(t.Context(), light)
		var names []string
		for _, req := range requests {
			names = append(names, req.Name)
		}
		slices.Sort(names)
		if want := []string{"group-a", "group-b"}; !slices.Equal(names, want) {
			t.Errorf("MapLightToGroups(light-2) = %v, want %v", names, want)
		}
	})

	t.Run("light referenced only by non-Reactive groups is filtered out entirely", func(t *testing.T) {
		light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light-3"}}
		if requests := mapFn(t.Context(), light); len(requests) != 0 {
			t.Errorf("MapLightToGroups(light-3) = %v, want empty (referencing groups are CircadianSchedule/unmanaged, not Reactive)", requests)
		}
	})

	t.Run("light referenced by no groups", func(t *testing.T) {
		light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light-unknown"}}
		if requests := mapFn(t.Context(), light); len(requests) != 0 {
			t.Errorf("MapLightToGroups(light-unknown) = %v, want empty", requests)
		}
	})

	t.Run("wrong object type returns nil", func(t *testing.T) {
		if requests := mapFn(t.Context(), &lumenetesv1alpha1.Group{}); requests != nil {
			t.Errorf("MapLightToGroups(Group) = %v, want nil", requests)
		}
	})

	t.Run("List failure returns nil, not a panic", func(t *testing.T) {
		wantErr := errors.New("boom")
		failing := interceptor.NewClient(c, interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				return wantErr
			},
		})
		light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light-2"}}
		if requests := MapLightToGroups(failing)(t.Context(), light); requests != nil {
			t.Errorf("MapLightToGroups with a failing List = %v, want nil", requests)
		}
	})
}
