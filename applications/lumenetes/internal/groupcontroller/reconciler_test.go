package groupcontroller

import (
	"context"
	"errors"
	"slices"
	"sync"
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

func sceneRef(name string) *lumenetesv1alpha1.ActiveSceneRef {
	return &lumenetesv1alpha1.ActiveSceneRef{Kind: lumenetesv1alpha1.ActiveSceneKindScene, Name: name}
}

func circadianRef(name string) *lumenetesv1alpha1.ActiveSceneRef {
	return &lumenetesv1alpha1.ActiveSceneRef{Kind: lumenetesv1alpha1.ActiveSceneKindCircadianSchedule, Name: name}
}

func offRef() *lumenetesv1alpha1.ActiveSceneRef {
	return &lumenetesv1alpha1.ActiveSceneRef{Kind: lumenetesv1alpha1.ActiveSceneKindOff}
}

func reactiveRef() *lumenetesv1alpha1.ActiveSceneRef {
	return &lumenetesv1alpha1.ActiveSceneRef{Kind: lumenetesv1alpha1.ActiveSceneKindReactive}
}

func fourKeyframes() []lumenetesv1alpha1.CircadianKeyframe {
	return []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 15, ColorTempK: 2200},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarNoon, Brightness: 100, ColorTempK: 6500},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunset, Brightness: 70, ColorTempK: 3500},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarMidnight, Brightness: 5, ColorTempK: 2000},
	}
}

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
		WithStatusSubresource(&lumenetesv1alpha1.Group{}, &lumenetesv1alpha1.Scene{}, &lumenetesv1alpha1.Light{}, &lumenetesv1alpha1.CircadianSchedule{}).
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
		Spec:       lumenetesv1alpha1.GroupSpec{ActiveScene: nil},
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
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a", "b"}, ActiveScene: offRef()},
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
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: sceneRef("missing-scene")},
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
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: sceneRef("movie")},
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
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a", "b"}, ActiveScene: sceneRef("movie")},
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
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"good", "bad"}, ActiveScene: sceneRef("movie")},
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

func TestReconcile_ActiveCircadianScheduleValid(t *testing.T) {
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	sunTimes, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	now := sunTimes.SolarNoon
	wantBrightness, wantColorTempK, _, err := circadian.Interpolate(fourKeyframes(), equator, now)
	if err != nil {
		t.Fatalf("circadian.Interpolate: %v", err)
	}

	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a", "b"}, ActiveScene: circadianRef("living-room-circadian")},
	}
	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room-circadian"},
		Spec:       lumenetesv1alpha1.CircadianScheduleSpec{Group: "living-room", Keyframes: fourKeyframes()},
	}
	// "b" has no colorTempK support - the sentinel skip must still apply
	// via the synthetic SceneLightState, exactly as it would for a Scene.
	// "a" starts with a stale Color (as if seeded long ago at Light
	// creation) - enactCircadianSchedule must relinquish it to "", or
	// lightscontroller.Reconciler would fight the bridge's real
	// color-temperature rendering with this stale value (see
	// enactCircadianSchedule's doc comment).
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: lumenetesv1alpha1.LightSpec{On: true, Brightness: 1, Color: "#ffd483", ColorTempK: 1}}
	lightB := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "b"}, Spec: lumenetesv1alpha1.LightSpec{On: true, Brightness: 1, ColorTempK: 0}}
	c := newFakeClient(t, group, schedule, lightA, lightB)
	r := &Reconciler{Client: c, Now: func() time.Time { return now }}

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

	afterA := getLight(t, c, "a")
	if afterA.Spec.Brightness != wantBrightness {
		t.Errorf("light a Spec.Brightness = %d, want %d", afterA.Spec.Brightness, wantBrightness)
	}
	if afterA.Spec.ColorTempK != wantColorTempK {
		t.Errorf("light a Spec.ColorTempK = %d, want %d", afterA.Spec.ColorTempK, wantColorTempK)
	}
	if afterA.Spec.Color != "" {
		t.Errorf("light a Spec.Color = %q, want relinquished (\"\") - a circadian curve never manages Color", afterA.Spec.Color)
	}
	if !afterA.Spec.On {
		t.Errorf("light a Spec.On = false, want untouched (true) - fourKeyframes() never sets On")
	}

	afterB := getLight(t, c, "b")
	if afterB.Spec.Brightness != wantBrightness {
		t.Errorf("light b Spec.Brightness = %d, want %d", afterB.Spec.Brightness, wantBrightness)
	}
	if afterB.Spec.ColorTempK != 0 {
		t.Errorf("light b Spec.ColorTempK = %d, want untouched (0, colorTempK unsupported)", afterB.Spec.ColorTempK)
	}
}

func TestReconcile_ActiveCircadianScheduleSetsOn(t *testing.T) {
	// A schedule whose current keyframe explicitly sets On must flip every
	// target light's Spec.On accordingly - unlike Brightness/ColorTempK,
	// this isn't gated by any per-light capability sentinel.
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	sunTimes, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	keyframes := []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarMidnight, Brightness: 0, ColorTempK: 2200, On: lumenetesv1alpha1.CircadianOnStateOff},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 40, ColorTempK: 3000, On: lumenetesv1alpha1.CircadianOnStateOn},
	}

	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "external-office"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: circadianRef("external-office-circadian")},
	}
	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "external-office-circadian"},
		Spec:       lumenetesv1alpha1.CircadianScheduleSpec{Group: "external-office", Keyframes: keyframes},
	}
	// Status.On starts true (matching Spec) - applyCircadianLightState
	// gates On on the light's own last-observed Status.On, not its Spec,
	// so the fixture must say what's really "on" right now.
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: lumenetesv1alpha1.LightSpec{On: true, Brightness: 100, ColorTempK: 1}, Status: lumenetesv1alpha1.LightStatus{On: true}}
	c := newFakeClient(t, group, schedule, lightA)

	beforeSunrise := sunTimes.Sunrise.Add(-2 * time.Hour)
	r := &Reconciler{Client: c, Now: func() time.Time { return beforeSunrise }}
	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "external-office"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	got := getLight(t, c, "a")
	if got.Spec.On {
		t.Errorf("light a Spec.On = true 2h before sunrise, want false")
	}

	// Simulate lightscontroller pushing that Spec.On=false to the bridge
	// and the poller reading it back into Status - the real-world
	// convergence applyCircadianLightState's Status.On gate relies on to
	// ever notice the next actual crossing.
	got.Status.On = got.Spec.On
	if err := c.Status().Update(t.Context(), &got); err != nil {
		t.Fatalf("update light status: %v", err)
	}

	r.Now = func() time.Time { return sunTimes.Sunrise }
	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "external-office"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if got := getLight(t, c, "a"); !got.Spec.On {
		t.Errorf("light a Spec.On = false at sunrise, want true")
	}
}

func TestReconcile_ActiveCircadianScheduleOnLeavesManualOverride(t *testing.T) {
	// A manual Spec.On override (e.g. kubectl patch) mid-span, with no
	// keyframe crossing since the light's Status last converged, must
	// stick - applyCircadianLightState only (re)pushes On when it
	// disagrees with the light's own Status.On, not merely whenever the
	// schedule reconciles, or the override would get fought back on the
	// very next resync even though the schedule's resolved level hasn't
	// actually changed.
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	sunTimes, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	keyframes := []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarMidnight, Brightness: 0, ColorTempK: 2200, On: lumenetesv1alpha1.CircadianOnStateOff},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 40, ColorTempK: 3000, On: lumenetesv1alpha1.CircadianOnStateOn},
	}
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "external-office"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: circadianRef("external-office-circadian")},
	}
	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "external-office-circadian"},
		Spec:       lumenetesv1alpha1.CircadianScheduleSpec{Group: "external-office", Keyframes: keyframes},
	}
	// Mid-day steady state: the schedule resolves "on", and the light
	// really is on (Status.On true) - as if the sunrise transition was
	// already enacted and converged some time ago.
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: lumenetesv1alpha1.LightSpec{On: true, Brightness: 100, ColorTempK: 1}, Status: lumenetesv1alpha1.LightStatus{On: true}}
	c := newFakeClient(t, group, schedule, lightA)

	midDay := sunTimes.Sunrise.Add(2 * time.Hour)
	r := &Reconciler{Client: c, Now: func() time.Time { return midDay }}

	// A user manually turns the light off - Status.On hasn't caught up
	// yet (no poller in this test), matching the real-world moment right
	// after the edit lands and before lightscontroller/the poller
	// round-trip it back into Status.
	light := getLight(t, c, "a")
	light.Spec.On = false
	if err := c.Update(t.Context(), &light); err != nil {
		t.Fatalf("update light: %v", err)
	}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "external-office"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if got := getLight(t, c, "a"); got.Spec.On {
		t.Errorf("light a Spec.On = true after reconcile, want the manual override (false) to stick since Status.On still agrees with the schedule's resolved level")
	}
}

func TestReconcile_ActiveCircadianScheduleWrongGroup(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: circadianRef("bedroom-circadian")},
	}
	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "bedroom-circadian"},
		Spec:       lumenetesv1alpha1.CircadianScheduleSpec{Group: "bedroom", Keyframes: fourKeyframes()},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	c := newFakeClient(t, group, schedule, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	want := `circadian schedule "bedroom-circadian" targets group "bedroom", not "living-room"`
	if got.Status.ActiveSceneError != want {
		t.Errorf("Status.ActiveSceneError = %q, want %q", got.Status.ActiveSceneError, want)
	}
}

func TestReconcile_ActiveCircadianScheduleInterpolateError(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: circadianRef("broken-circadian")},
	}
	schedule := &lumenetesv1alpha1.CircadianSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "broken-circadian"},
		Spec: lumenetesv1alpha1.CircadianScheduleSpec{
			Group:     "living-room",
			Keyframes: []lumenetesv1alpha1.CircadianKeyframe{{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 10, ColorTempK: 2000}},
		},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, group, schedule, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	if got.Status.ActiveSceneError == "" {
		t.Error("Status.ActiveSceneError = \"\", want the surfaced Interpolate error")
	}
	if getLight(t, c, "a").Spec.On {
		t.Errorf("light a Spec.On = true, want untouched")
	}
}

func TestReconcile_ActiveSceneNeitherSceneNorCircadianSchedule(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: circadianRef("missing-schedule")},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	c := newFakeClient(t, group, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	want := `circadian schedule "missing-schedule" not found`
	if got.Status.ActiveSceneError != want {
		t.Errorf("Status.ActiveSceneError = %q, want %q", got.Status.ActiveSceneError, want)
	}
}

func TestReconcile_ActiveSceneUnknownKind(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: &lumenetesv1alpha1.ActiveSceneRef{Kind: "Bogus", Name: "whatever"}},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	c := newFakeClient(t, group, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &got); err != nil {
		t.Fatalf("get group: %v", err)
	}
	want := `unknown activeScene kind "Bogus"`
	if got.Status.ActiveSceneError != want {
		t.Errorf("Status.ActiveSceneError = %q, want %q", got.Status.ActiveSceneError, want)
	}
}

func TestReconcile_ActiveReactive_MirrorsStatusToSpec(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: reactiveRef()},
	}
	lightA := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Spec:       lumenetesv1alpha1.LightSpec{Name: "Kitchen", On: false, Brightness: 10, Color: "#000000", ColorTempK: 2000},
		Status: lumenetesv1alpha1.LightStatus{
			Reachable: true, On: true, Brightness: 80, Color: "#ffffff", ColorTempK: 5000,
		},
	}
	c := newFakeClient(t, group, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	got := getLight(t, c, "a")
	if got.Spec.On != true || got.Spec.Brightness != 80 || got.Spec.Color != "#ffffff" || got.Spec.ColorTempK != 5000 {
		t.Errorf("Spec = %+v, want mirrored from Status", got.Spec)
	}
	if !got.Spec.Reactive {
		t.Error("Spec.Reactive = false, want true - this is what internal/lightscontroller.Reconciler checks before enacting")
	}
	if got.Spec.Name != "Kitchen" {
		t.Errorf("Spec.Name = %q, want untouched (%q) - renaming isn't part of reactive mirroring", got.Spec.Name, "Kitchen")
	}

	var gotGroup lumenetesv1alpha1.Group
	if err := c.Get(t.Context(), client.ObjectKey{Name: "living-room"}, &gotGroup); err != nil {
		t.Fatalf("get group: %v", err)
	}
	if gotGroup.Status.ActiveSceneError != "" {
		t.Errorf("Status.ActiveSceneError = %q, want empty", gotGroup.Status.ActiveSceneError)
	}
}

func TestReconcile_ActiveReactive_UnreachableSkipped(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: reactiveRef()},
	}
	lightA := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Spec:       lumenetesv1alpha1.LightSpec{On: false, Brightness: 10},
		Status:     lumenetesv1alpha1.LightStatus{Reachable: false, On: true, Brightness: 80},
	}
	c := newFakeClient(t, group, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	got := getLight(t, c, "a")
	if got.Spec.On != false || got.Spec.Brightness != 10 {
		t.Errorf("Spec = %+v, want untouched (unreachable Status is stale)", got.Spec)
	}
}

func TestReconcile_ActiveReactive_NoOpSkipsUpdate(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: reactiveRef()},
	}
	lightA := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true, Brightness: 80, Color: "#ffffff", ColorTempK: 5000, Reactive: true},
		Status:     lumenetesv1alpha1.LightStatus{Reachable: true, On: true, Brightness: 80, Color: "#ffffff", ColorTempK: 5000},
	}
	c := newFakeClient(t, group, lightA)
	r := &Reconciler{Client: c}

	before := getLight(t, c, "a")

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	after := getLight(t, c, "a")
	if after.ResourceVersion != before.ResourceVersion {
		t.Errorf("ResourceVersion changed from %s to %s, want no-op (Spec already matches Status and Reactive already true)", before.ResourceVersion, after.ResourceVersion)
	}
}

func TestReconcile_ActiveReactive_NonMemberUntouched(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: reactiveRef()},
	}
	lightA := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "a"}, Status: lumenetesv1alpha1.LightStatus{Reachable: true, On: true}}
	// "b" is not a member of group.Spec.Lights - must be left alone even
	// though it exists.
	lightB := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "b"},
		Spec:       lumenetesv1alpha1.LightSpec{On: false},
		Status:     lumenetesv1alpha1.LightStatus{Reachable: true, On: true},
	}
	c := newFakeClient(t, group, lightA, lightB)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	if getLight(t, c, "b").Spec.On {
		t.Errorf("light b Spec.On = true, want untouched (false) - not a group member")
	}
}

// TestReconcile_TransitionOutOfReactive_ClearsReactiveFlag covers the
// regression LightSpec.Reactive's doc comment warns about: every
// non-Reactive enactment path must clear a light's Spec.Reactive itself,
// or a light that was once in a Reactive-mode Group would stay invisible
// to internal/lightscontroller.Reconciler forever, even after this Group
// moves it to Off. Off is exercised directly here; enactScene and
// enactCircadianSchedule share the same clearing via applySceneStateToSpec
// (see that function's doc comment), not re-tested per Kind.
func TestReconcile_TransitionOutOfReactive_ClearsReactiveFlag(t *testing.T) {
	group := &lumenetesv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "living-room"},
		Spec:       lumenetesv1alpha1.GroupSpec{Lights: []string{"a"}, ActiveScene: offRef()},
	}
	lightA := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "a"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true, Reactive: true},
	}
	c := newFakeClient(t, group, lightA)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "living-room"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	got := getLight(t, c, "a")
	if got.Spec.On {
		t.Errorf("light a Spec.On = true, want false (Off)")
	}
	if got.Spec.Reactive {
		t.Error("light a Spec.Reactive = true, want false - stuck reactive flag would hide this light from lightscontroller forever")
	}
}
