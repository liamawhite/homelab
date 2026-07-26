package circadianschedulecontroller

import (
	"context"
	"errors"
	"testing"
	"time"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/circadian"
	"github.com/liamawhite/lumenetes/internal/sun"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var equator = sun.Coordinates{Latitude: 0, Longitude: 0}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := lumenetesv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return scheme
}

func reconcileRequest(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: client.ObjectKey{Name: name}}
}

func fourKeyframes() []lumenetesv1alpha1.CircadianKeyframe {
	return []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 15, ColorTempK: 2200},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarNoon, Brightness: 100, ColorTempK: 6500},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunset, Brightness: 70, ColorTempK: 3500},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarMidnight, Brightness: 5, ColorTempK: 2000},
	}
}

func TestReconcile_NotFound(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	r := &Reconciler{Client: c}

	res, err := r.Reconcile(t.Context(), reconcileRequest("missing"))
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	if res != (ctrl.Result{}) {
		t.Errorf("Reconcile() result = %v, want zero value", res)
	}
}

func TestReconcile_EmptyGroup(t *testing.T) {
	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "no-group"},
		Spec:       lumenetesv1alpha1.CircadianScheduleSpec{Keyframes: fourKeyframes()},
	}
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&lumenetesv1alpha1.CircadianSchedule{}).
		WithObjects(schedule).
		Build()
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), reconcileRequest("no-group")); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.CircadianSchedule
	if err := c.Get(t.Context(), client.ObjectKey{Name: "no-group"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.ValidationError == "" {
		t.Error("Status.ValidationError = \"\", want non-empty for missing spec.group")
	}
	if got.Status.CurrentBrightness != nil || got.Status.CurrentColorTempK != nil {
		t.Errorf("Status.Current* = (%v, %v), want (nil, nil)", got.Status.CurrentBrightness, got.Status.CurrentColorTempK)
	}
}

func TestReconcile_TooFewKeyframesSurfacesInterpolateError(t *testing.T) {
	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "single-keyframe"},
		Spec: lumenetesv1alpha1.CircadianScheduleSpec{
			Group:     "living-space",
			Keyframes: []lumenetesv1alpha1.CircadianKeyframe{{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 10, ColorTempK: 2000}},
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&lumenetesv1alpha1.CircadianSchedule{}).
		WithObjects(schedule).
		Build()
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), reconcileRequest("single-keyframe")); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.CircadianSchedule
	if err := c.Get(t.Context(), client.ObjectKey{Name: "single-keyframe"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.ValidationError == "" {
		t.Error("Status.ValidationError = \"\", want the surfaced Interpolate error")
	}
}

func TestReconcile_ValidScheduleComputesCurrentValues(t *testing.T) {
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	now := times.SolarNoon
	wantB, wantC, err := circadian.Interpolate(fourKeyframes(), equator, now)
	if err != nil {
		t.Fatalf("circadian.Interpolate: %v", err)
	}

	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "living-space-circadian"},
		Spec:       lumenetesv1alpha1.CircadianScheduleSpec{Group: "living-space", Keyframes: fourKeyframes()},
	}
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&lumenetesv1alpha1.CircadianSchedule{}).
		WithObjects(schedule).
		Build()
	r := &Reconciler{Client: c, Now: func() time.Time { return now }}

	if _, err := r.Reconcile(t.Context(), reconcileRequest("living-space-circadian")); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.CircadianSchedule
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-space-circadian"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.ValidationError != "" {
		t.Errorf("Status.ValidationError = %q, want empty", got.Status.ValidationError)
	}
	if got.Status.CurrentBrightness == nil || *got.Status.CurrentBrightness != wantB {
		t.Errorf("Status.CurrentBrightness = %v, want %d", got.Status.CurrentBrightness, wantB)
	}
	if got.Status.CurrentColorTempK == nil || *got.Status.CurrentColorTempK != wantC {
		t.Errorf("Status.CurrentColorTempK = %v, want %d", got.Status.CurrentColorTempK, wantC)
	}
	if got.Status.LastSynced.IsZero() {
		t.Error("Status.LastSynced not set")
	}
}

func TestReconcile_UnchangedValuesSkipStatusUpdate(t *testing.T) {
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	now := times.SolarNoon
	wantB, wantC, err := circadian.Interpolate(fourKeyframes(), equator, now)
	if err != nil {
		t.Fatalf("circadian.Interpolate: %v", err)
	}

	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "living-space-circadian"},
		Spec:       lumenetesv1alpha1.CircadianScheduleSpec{Group: "living-space", Keyframes: fourKeyframes()},
		Status: lumenetesv1alpha1.CircadianScheduleStatus{
			CurrentBrightness: &wantB,
			CurrentColorTempK: &wantC,
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&lumenetesv1alpha1.CircadianSchedule{}).
		WithObjects(schedule).
		Build()
	r := &Reconciler{Client: c, Now: func() time.Time { return now }}

	var before lumenetesv1alpha1.CircadianSchedule
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-space-circadian"}, &before); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if _, err := r.Reconcile(t.Context(), reconcileRequest("living-space-circadian")); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var after lumenetesv1alpha1.CircadianSchedule
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-space-circadian"}, &after); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if after.ResourceVersion != before.ResourceVersion {
		t.Errorf("ResourceVersion changed from %s to %s, want no-op (unchanged values)", before.ResourceVersion, after.ResourceVersion)
	}
}

func TestReconcile_GetFailsWithNonNotFoundError(t *testing.T) {
	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "living-space-circadian"},
		Spec:       lumenetesv1alpha1.CircadianScheduleSpec{Group: "living-space", Keyframes: fourKeyframes()},
	}
	wantErr := errors.New("boom")
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&lumenetesv1alpha1.CircadianSchedule{}).
		WithObjects(schedule).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*lumenetesv1alpha1.CircadianSchedule); ok {
					return wantErr
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), reconcileRequest("living-space-circadian")); !errors.Is(err, wantErr) {
		t.Fatalf("Reconcile() error = %v, want %v", err, wantErr)
	}
}
