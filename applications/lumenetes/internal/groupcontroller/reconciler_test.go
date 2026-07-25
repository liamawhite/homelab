package groupcontroller

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestMissingLights(t *testing.T) {
	cases := []struct {
		name     string
		spec     []string
		existing map[string]bool
		want     []string
	}{
		{
			name: "empty spec",
			spec: nil,
			want: nil,
		},
		{
			name:     "all present",
			spec:     []string{"a", "b"},
			existing: map[string]bool{"a": true, "b": true},
			want:     nil,
		},
		{
			name:     "some missing",
			spec:     []string{"a", "b", "c"},
			existing: map[string]bool{"a": true, "c": true},
			want:     []string{"b"},
		},
		{
			name:     "all missing",
			spec:     []string{"a", "b"},
			existing: map[string]bool{},
			want:     []string{"a", "b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := missingLights(tc.spec, tc.existing)
			if !slices.Equal(got, tc.want) {
				t.Errorf("missingLights(%v, %v) = %v, want %v", tc.spec, tc.existing, got, tc.want)
			}
		})
	}
}

func boolPtr(b bool) *bool    { return &b }
func int32Ptr(v int32) *int32 { return &v }
func strPtr(s string) *string { return &s }

func TestApplySceneStateToSpec(t *testing.T) {
	baseline := lumenetesv1alpha1.LightSpec{
		Name: "Kitchen", On: false, Brightness: 50, Color: "#ffedbb", ColorTempK: 2700,
	}
	noDimming := lumenetesv1alpha1.LightSpec{Name: "Fan Light", On: true, Brightness: -1, Color: "", ColorTempK: 0}

	cases := []struct {
		name    string
		current lumenetesv1alpha1.LightSpec
		state   lumenetesv1alpha1.SceneLightState
		want    lumenetesv1alpha1.LightSpec
	}{
		{
			name:    "on true",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{On: boolPtr(true)},
			want:    withOn(baseline, true),
		},
		{
			name:    "on false",
			current: withOn(baseline, true),
			state:   lumenetesv1alpha1.SceneLightState{On: boolPtr(false)},
			want:    baseline,
		},
		{
			name:    "brightness set in range",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{Brightness: int32Ptr(80)},
			want:    withBrightness(baseline, 80),
		},
		{
			name:    "brightness set clamped above 100",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{Brightness: int32Ptr(150)},
			want:    withBrightness(baseline, 100),
		},
		{
			name:    "brightness set clamped below 0",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{Brightness: int32Ptr(-20)},
			want:    withBrightness(baseline, 0),
		},
		{
			name:    "brightness set no-op on non-dimmable light",
			current: noDimming,
			state:   lumenetesv1alpha1.SceneLightState{Brightness: int32Ptr(50)},
			want:    noDimming,
		},
		{
			name:    "color set on color-capable light",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{Color: strPtr("#ff0000")},
			want:    withColor(baseline, "#ff0000"),
		},
		{
			name:    "color set no-op on non-color light",
			current: noDimming,
			state:   lumenetesv1alpha1.SceneLightState{Color: strPtr("#ff0000")},
			want:    noDimming,
		},
		{
			name:    "colorTempK set on capable light",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{ColorTempK: int32Ptr(4000)},
			want:    withColorTempK(baseline, 4000),
		},
		{
			name:    "colorTempK set no-op when unsupported",
			current: noDimming,
			state:   lumenetesv1alpha1.SceneLightState{ColorTempK: int32Ptr(4000)},
			want:    noDimming,
		},
		{
			name:    "combined on and brightness",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{On: boolPtr(true), Brightness: int32Ptr(20)},
			want:    withBrightness(withOn(baseline, true), 20),
		},
		{
			name:    "empty state is a true no-op",
			current: baseline,
			state:   lumenetesv1alpha1.SceneLightState{},
			want:    baseline,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applySceneStateToSpec(tc.current, tc.state)
			if got != tc.want {
				t.Errorf("applySceneStateToSpec() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func withOn(s lumenetesv1alpha1.LightSpec, on bool) lumenetesv1alpha1.LightSpec {
	s.On = on
	return s
}

func withBrightness(s lumenetesv1alpha1.LightSpec, b int32) lumenetesv1alpha1.LightSpec {
	s.Brightness = b
	return s
}

func withColor(s lumenetesv1alpha1.LightSpec, c string) lumenetesv1alpha1.LightSpec {
	s.Color = c
	return s
}

func withColorTempK(s lumenetesv1alpha1.LightSpec, k int32) lumenetesv1alpha1.LightSpec {
	s.ColorTempK = k
	return s
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := lumenetesv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	return scheme
}

func newFakeClient(t *testing.T, objs ...client.Object) client.WithWatch {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithObjects(objs...).
		WithStatusSubresource(&lumenetesv1alpha1.Group{}, &lumenetesv1alpha1.Scene{}, &lumenetesv1alpha1.Light{}).
		Build()
}

func getLight(t *testing.T, c client.Client, name string) lumenetesv1alpha1.Light {
	t.Helper()
	var light lumenetesv1alpha1.Light
	if err := c.Get(t.Context(), client.ObjectKey{Name: name}, &light); err != nil {
		t.Fatalf("get light %q: %v", name, err)
	}
	return light
}

func TestReconcile_GroupNotFound(t *testing.T) {
	c := newFakeClient(t)
	r := &Reconciler{Client: c}

	res, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "missing"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	if res != (ctrl.Result{}) {
		t.Errorf("Reconcile() result = %+v, want empty", res)
	}
}

func TestReconcile_AllLightsExist(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a", "b"}},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	lightB := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "b"}}
	c := newFakeClient(t, group, lightA, lightB)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	if got.Status.MissingLights != nil {
		t.Errorf("Status.MissingLights = %v, want nil", got.Status.MissingLights)
	}
	if got.Status.LightCount != 2 {
		t.Errorf("Status.LightCount = %d, want 2", got.Status.LightCount)
	}
}

func TestReconcile_SomeLightsMissing(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a", "b", "c"}},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	lightC := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "c"}}
	c := newFakeClient(t, group, lightA, lightC)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	if !slices.Equal(got.Status.MissingLights, []string{"b"}) {
		t.Errorf("Status.MissingLights = %v, want [b]", got.Status.MissingLights)
	}
}

func TestReconcile_ActiveSceneEmpty(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{ActiveScene: ""},
		Status:     lumenetesv1alpha1.GroupStatus{ActiveSceneError: "stale error from a previous scene"},
	}
	c := newFakeClient(t, group)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	if got.Status.ActiveSceneError != "" {
		t.Errorf("Status.ActiveSceneError = %q, want empty", got.Status.ActiveSceneError)
	}
}

func TestReconcile_ActiveSceneOff(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a", "b"}, ActiveScene: "off"},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: lumenetesv1alpha1.LightSpec{On: true}}
	lightB := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "b"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, group, lightA, lightB)
	r := &Reconciler{Client: c}

	beforeB := getLight(t, c, "b")

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	afterA := getLight(t, c, "a")
	if afterA.Spec.On {
		t.Errorf("light a Spec.On = true, want false")
	}
	afterB := getLight(t, c, "b")
	if afterB.Spec.On {
		t.Errorf("light b Spec.On = true, want false")
	}
	if afterB.ResourceVersion != beforeB.ResourceVersion {
		t.Errorf("light b was updated (resourceVersion %s -> %s) though already off", beforeB.ResourceVersion, afterB.ResourceVersion)
	}
}

func TestReconcile_ActiveSceneNotFound(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: "missing-scene"},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, group, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	want := `scene "missing-scene" not found`
	if got.Status.ActiveSceneError != want {
		t.Errorf("Status.ActiveSceneError = %q, want %q", got.Status.ActiveSceneError, want)
	}
	if getLight(t, c, "a").Spec.On {
		t.Errorf("light a Spec.On = true, want untouched (false)")
	}
}

func TestReconcile_ActiveSceneWrongGroup(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: "movie"},
	}
	scene := &lumenetesv1alpha1.Scene{
		ObjectMeta: metav1.ObjectMeta{Name: "movie"},
		Spec:       lumenetesv1alpha1.SceneSpec{Group: "bedroom"},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	c := newFakeClient(t, group, scene, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	want := `scene "movie" targets group "bedroom", not "living-room"`
	if got.Status.ActiveSceneError != want {
		t.Errorf("Status.ActiveSceneError = %q, want %q", got.Status.ActiveSceneError, want)
	}
}

func TestReconcile_ActiveSceneValid(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a", "b"}, ActiveScene: "movie"},
	}
	scene := &lumenetesv1alpha1.Scene{
		ObjectMeta: metav1.ObjectMeta{Name: "movie"},
		Spec: lumenetesv1alpha1.SceneSpec{
			Group: "living-room",
			Lights: []lumenetesv1alpha1.SceneLightState{
				{Name: "a", On: boolPtr(true)},
				// "c" is not a member of the group's Spec.Lights - must be skipped.
				{Name: "c", On: boolPtr(true)},
			},
		},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	lightB := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "b"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	lightC := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "c"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, group, scene, lightA, lightB, lightC)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if !getLight(t, c, "a").Spec.On {
		t.Errorf("light a Spec.On = false, want true (scene member)")
	}
	if getLight(t, c, "b").Spec.On {
		t.Errorf("light b Spec.On = true, want false (scene doesn't target it)")
	}
	if getLight(t, c, "c").Spec.On {
		t.Errorf("light c Spec.On = true, want untouched (not a group member despite being in the scene)")
	}
}

// TestReconcile_EnactScene_PartialFailure verifies that one target light
// failing to enact doesn't stop a sibling light in the same scene from
// being applied, and that Reconcile still returns an error (to trigger
// requeue). The injected Get failure only fires on the *second* Get for
// "bad" - the first (Reconcile's own Spec.Lights existence check) must
// succeed, or Reconcile would short-circuit before ever reaching
// enactScene, defeating the point of this test.
func TestReconcile_EnactScene_PartialFailure(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"good", "bad"}, ActiveScene: "movie"},
	}
	scene := &lumenetesv1alpha1.Scene{
		ObjectMeta: metav1.ObjectMeta{Name: "movie"},
		Spec: lumenetesv1alpha1.SceneSpec{
			Group: "living-room",
			Lights: []lumenetesv1alpha1.SceneLightState{
				{Name: "good", On: boolPtr(true)},
				{Name: "bad", On: boolPtr(true)},
			},
		},
	}
	lightGood := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "good"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	lightBad := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "bad"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	base := newFakeClient(t, group, scene, lightGood, lightBad)

	var mu sync.Mutex
	badGetCount := 0
	c := interceptor.NewClient(base, interceptor.Funcs{
		Get: func(ctx context.Context, inner client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if key.Name == "bad" {
				mu.Lock()
				badGetCount++
				n := badGetCount
				mu.Unlock()
				if n > 1 {
					return errors.New("injected: bridge-side get failure")
				}
			}
			return inner.Get(ctx, key, obj, opts...)
		},
	})
	r := &Reconciler{Client: c}

	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}})
	if err == nil {
		t.Fatalf("Reconcile() error = nil, want non-nil (bad light's enactment should fail)")
	}

	if !getLight(t, base, "good").Spec.On {
		t.Errorf("light good Spec.On = false, want true (sibling failure must not block it)")
	}
	if getLight(t, base, "bad").Spec.On {
		t.Errorf("light bad Spec.On = true, want untouched (its Get failed before Update)")
	}
}
