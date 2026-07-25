package scenecontroller

import (
	"context"
	"errors"
	"slices"
	"testing"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func sceneLights(names ...string) []lumenetesv1alpha1.SceneLightState {
	states := make([]lumenetesv1alpha1.SceneLightState, len(names))
	for i, name := range names {
		states[i] = lumenetesv1alpha1.SceneLightState{Name: name}
	}
	return states
}

func TestInvalidLights(t *testing.T) {
	cases := []struct {
		name           string
		sceneLights    []lumenetesv1alpha1.SceneLightState
		groupMembers   map[string]bool
		existingLights map[string]bool
		want           []string
	}{
		{
			name: "empty spec",
			want: nil,
		},
		{
			name:           "all valid",
			sceneLights:    sceneLights("a", "b"),
			groupMembers:   map[string]bool{"a": true, "b": true},
			existingLights: map[string]bool{"a": true, "b": true},
			want:           nil,
		},
		{
			name:           "not a group member",
			sceneLights:    sceneLights("a", "b"),
			groupMembers:   map[string]bool{"a": true},
			existingLights: map[string]bool{"a": true, "b": true},
			want:           []string{"b"},
		},
		{
			name:           "light doesn't exist",
			sceneLights:    sceneLights("a", "b"),
			groupMembers:   map[string]bool{"a": true, "b": true},
			existingLights: map[string]bool{"a": true},
			want:           []string{"b"},
		},
		{
			name:           "group not found - every light invalid",
			sceneLights:    sceneLights("a", "b"),
			groupMembers:   nil,
			existingLights: map[string]bool{"a": true, "b": true},
			want:           []string{"a", "b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := invalidLights(tc.sceneLights, tc.groupMembers, tc.existingLights)
			if !slices.Equal(got, tc.want) {
				t.Errorf("invalidLights(%v, %v, %v) = %v, want %v", tc.sceneLights, tc.groupMembers, tc.existingLights, got, tc.want)
			}
		})
	}
}

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

func TestReconcile(t *testing.T) {
	t.Run("scene not found", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
		r := &Reconciler{Client: c}

		res, err := r.Reconcile(t.Context(), reconcileRequest("missing"))
		if err != nil {
			t.Fatalf("Reconcile() error = %v, want nil", err)
		}
		if res != (ctrl.Result{}) {
			t.Errorf("Reconcile() result = %v, want zero value", res)
		}
	})

	t.Run("group exists - membership derived correctly", func(t *testing.T) {
		scene := &lumenetesv1alpha1.Scene{
			ObjectMeta: metav1.ObjectMeta{Name: "movie-night"},
			Spec: lumenetesv1alpha1.SceneSpec{
				Group:  "living-room",
				Lights: sceneLights("lamp", "tv-backlight"),
			},
		}
		group := &lumenetesv1alpha1.Group{
			ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
			Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"lamp", "tv-backlight"}},
		}
		lamp := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "lamp"}}
		tvBacklight := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "tv-backlight"}}

		c := fake.NewClientBuilder().
			WithScheme(newTestScheme(t)).
			WithStatusSubresource(&lumenetesv1alpha1.Scene{}).
			WithObjects(scene, group, lamp, tvBacklight).
			Build()
		r := &Reconciler{Client: c}

		if _, err := r.Reconcile(t.Context(), reconcileRequest("movie-night")); err != nil {
			t.Fatalf("Reconcile() error = %v, want nil", err)
		}

		var got lumenetesv1alpha1.Scene
		if err := c.Get(t.Context(), client.ObjectKey{Name: "movie-night"}, &got); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.Status.InvalidLights != nil {
			t.Errorf("Status.InvalidLights = %v, want nil", got.Status.InvalidLights)
		}
	})

	t.Run("group doesn't exist - every light reported invalid", func(t *testing.T) {
		scene := &lumenetesv1alpha1.Scene{
			ObjectMeta: metav1.ObjectMeta{Name: "movie-night"},
			Spec: lumenetesv1alpha1.SceneSpec{
				Group:  "no-such-group",
				Lights: sceneLights("lamp", "tv-backlight"),
			},
		}
		lamp := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "lamp"}}
		tvBacklight := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "tv-backlight"}}

		c := fake.NewClientBuilder().
			WithScheme(newTestScheme(t)).
			WithStatusSubresource(&lumenetesv1alpha1.Scene{}).
			WithObjects(scene, lamp, tvBacklight).
			Build()
		r := &Reconciler{Client: c}

		if _, err := r.Reconcile(t.Context(), reconcileRequest("movie-night")); err != nil {
			t.Fatalf("Reconcile() error = %v, want nil", err)
		}

		var got lumenetesv1alpha1.Scene
		if err := c.Get(t.Context(), client.ObjectKey{Name: "movie-night"}, &got); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		want := []string{"lamp", "tv-backlight"}
		if !slices.Equal(got.Status.InvalidLights, want) {
			t.Errorf("Status.InvalidLights = %v, want %v", got.Status.InvalidLights, want)
		}
	})

	t.Run("group Get fails with non-NotFound error - Reconcile returns error, no status update", func(t *testing.T) {
		scene := &lumenetesv1alpha1.Scene{
			ObjectMeta: metav1.ObjectMeta{Name: "movie-night"},
			Spec: lumenetesv1alpha1.SceneSpec{
				Group:  "living-room",
				Lights: sceneLights("lamp"),
			},
		}
		lamp := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "lamp"}}

		wantErr := errors.New("boom")
		c := fake.NewClientBuilder().
			WithScheme(newTestScheme(t)).
			WithStatusSubresource(&lumenetesv1alpha1.Scene{}).
			WithObjects(scene, lamp).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*lumenetesv1alpha1.Group); ok {
						return wantErr
					}
					return c.Get(ctx, key, obj, opts...)
				},
			}).
			Build()
		r := &Reconciler{Client: c}

		if _, err := r.Reconcile(t.Context(), reconcileRequest("movie-night")); !errors.Is(err, wantErr) {
			t.Fatalf("Reconcile() error = %v, want %v", err, wantErr)
		}

		var got lumenetesv1alpha1.Scene
		if err := c.Get(t.Context(), client.ObjectKey{Name: "movie-night"}, &got); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.Status.InvalidLights != nil || !got.Status.LastSynced.IsZero() {
			t.Errorf("Status = %+v, want untouched", got.Status)
		}
	})

	t.Run("light doesn't exist - reported invalid even though it's a group member", func(t *testing.T) {
		scene := &lumenetesv1alpha1.Scene{
			ObjectMeta: metav1.ObjectMeta{Name: "movie-night"},
			Spec: lumenetesv1alpha1.SceneSpec{
				Group:  "living-room",
				Lights: sceneLights("lamp", "ghost-light"),
			},
		}
		group := &lumenetesv1alpha1.Group{
			ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
			Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"lamp", "ghost-light"}},
		}
		lamp := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "lamp"}}

		c := fake.NewClientBuilder().
			WithScheme(newTestScheme(t)).
			WithStatusSubresource(&lumenetesv1alpha1.Scene{}).
			WithObjects(scene, group, lamp).
			Build()
		r := &Reconciler{Client: c}

		if _, err := r.Reconcile(t.Context(), reconcileRequest("movie-night")); err != nil {
			t.Fatalf("Reconcile() error = %v, want nil", err)
		}

		var got lumenetesv1alpha1.Scene
		if err := c.Get(t.Context(), client.ObjectKey{Name: "movie-night"}, &got); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		want := []string{"ghost-light"}
		if !slices.Equal(got.Status.InvalidLights, want) {
			t.Errorf("Status.InvalidLights = %v, want %v", got.Status.InvalidLights, want)
		}
	})

	t.Run("light Get fails with non-NotFound error - Reconcile returns error, short-circuits loop", func(t *testing.T) {
		scene := &lumenetesv1alpha1.Scene{
			ObjectMeta: metav1.ObjectMeta{Name: "movie-night"},
			Spec: lumenetesv1alpha1.SceneSpec{
				Group:  "living-room",
				Lights: sceneLights("lamp", "tv-backlight"),
			},
		}
		group := &lumenetesv1alpha1.Group{
			ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
			Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"lamp", "tv-backlight"}},
		}
		lamp := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "lamp"}}
		tvBacklight := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "tv-backlight"}}

		wantErr := errors.New("boom")
		c := fake.NewClientBuilder().
			WithScheme(newTestScheme(t)).
			WithStatusSubresource(&lumenetesv1alpha1.Scene{}).
			WithObjects(scene, group, lamp, tvBacklight).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*lumenetesv1alpha1.Light); ok && key.Name == "lamp" {
						return wantErr
					}
					return c.Get(ctx, key, obj, opts...)
				},
			}).
			Build()
		r := &Reconciler{Client: c}

		if _, err := r.Reconcile(t.Context(), reconcileRequest("movie-night")); !errors.Is(err, wantErr) {
			t.Fatalf("Reconcile() error = %v, want %v", err, wantErr)
		}

		var got lumenetesv1alpha1.Scene
		if err := c.Get(t.Context(), client.ObjectKey{Name: "movie-night"}, &got); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.Status.InvalidLights != nil {
			t.Errorf("Status.InvalidLights = %v, want untouched (nil)", got.Status.InvalidLights)
		}
	})

	t.Run("computed invalid list unchanged - no status update", func(t *testing.T) {
		scene := &lumenetesv1alpha1.Scene{
			ObjectMeta: metav1.ObjectMeta{Name: "movie-night"},
			Spec: lumenetesv1alpha1.SceneSpec{
				Group:  "living-room",
				Lights: sceneLights("lamp", "ghost-light"),
			},
			Status: lumenetesv1alpha1.SceneStatus{
				InvalidLights: []string{"ghost-light"},
			},
		}
		group := &lumenetesv1alpha1.Group{
			ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
			Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"lamp", "ghost-light"}},
		}
		lamp := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "lamp"}}

		c := fake.NewClientBuilder().
			WithScheme(newTestScheme(t)).
			WithStatusSubresource(&lumenetesv1alpha1.Scene{}).
			WithObjects(scene, group, lamp).
			Build()
		r := &Reconciler{Client: c}

		var before lumenetesv1alpha1.Scene
		if err := c.Get(t.Context(), client.ObjectKey{Name: "movie-night"}, &before); err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if _, err := r.Reconcile(t.Context(), reconcileRequest("movie-night")); err != nil {
			t.Fatalf("Reconcile() error = %v, want nil", err)
		}

		var after lumenetesv1alpha1.Scene
		if err := c.Get(t.Context(), client.ObjectKey{Name: "movie-night"}, &after); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if after.ResourceVersion != before.ResourceVersion {
			t.Errorf("ResourceVersion changed from %s to %s, want no-op (unchanged invalid list)", before.ResourceVersion, after.ResourceVersion)
		}
	})

	t.Run("computed invalid list differs - status updated", func(t *testing.T) {
		scene := &lumenetesv1alpha1.Scene{
			ObjectMeta: metav1.ObjectMeta{Name: "movie-night"},
			Spec: lumenetesv1alpha1.SceneSpec{
				Group:  "living-room",
				Lights: sceneLights("lamp", "ghost-light"),
			},
			Status: lumenetesv1alpha1.SceneStatus{
				InvalidLights: nil,
			},
		}
		group := &lumenetesv1alpha1.Group{
			ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
			Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"lamp", "ghost-light"}},
		}
		lamp := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "lamp"}}

		c := fake.NewClientBuilder().
			WithScheme(newTestScheme(t)).
			WithStatusSubresource(&lumenetesv1alpha1.Scene{}).
			WithObjects(scene, group, lamp).
			Build()
		r := &Reconciler{Client: c}

		if _, err := r.Reconcile(t.Context(), reconcileRequest("movie-night")); err != nil {
			t.Fatalf("Reconcile() error = %v, want nil", err)
		}

		var got lumenetesv1alpha1.Scene
		if err := c.Get(t.Context(), client.ObjectKey{Name: "movie-night"}, &got); err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		want := []string{"ghost-light"}
		if !slices.Equal(got.Status.InvalidLights, want) {
			t.Errorf("Status.InvalidLights = %v, want %v", got.Status.InvalidLights, want)
		}
		if got.Status.LastSynced.IsZero() {
			t.Errorf("Status.LastSynced not set")
		}
	})
}
