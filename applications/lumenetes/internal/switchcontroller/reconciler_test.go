package switchcontroller

import (
	"context"
	"fmt"
	"testing"
	"time"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := lumenetesv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	return scheme
}

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&lumenetesv1alpha1.Switch{}, &lumenetesv1alpha1.Light{}).
		WithObjects(objs...).
		Build()
}

// newFakeClientWithInterceptors is newFakeClient plus interceptor.Funcs, for
// forcing otherwise-unreachable failure paths (e.g. a Status().Update that
// fails) that the plain fake client has no way to simulate.
func newFakeClientWithInterceptors(t *testing.T, funcs interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()
	base := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithStatusSubresource(&lumenetesv1alpha1.Switch{}, &lumenetesv1alpha1.Light{}).
		WithObjects(objs...).
		Build()
	return interceptor.NewClient(base, funcs)
}

func getLight(t *testing.T, c client.Client, name string) lumenetesv1alpha1.Light {
	t.Helper()
	var light lumenetesv1alpha1.Light
	if err := c.Get(context.Background(), client.ObjectKey{Name: name}, &light); err != nil {
		t.Fatalf("Get light %s: %v", name, err)
	}
	return light
}

func getSwitch(t *testing.T, c client.Client, name string) lumenetesv1alpha1.Switch {
	t.Helper()
	var sw lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: name}, &sw); err != nil {
		t.Fatalf("Get switch %s: %v", name, err)
	}
	return sw
}

func TestReconcile_SwitchNotFound(t *testing.T) {
	c := newFakeClient(t)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "missing"}}); err != nil {
		t.Errorf("Reconcile() error = %v, want nil for a deleted/missing Switch", err)
	}
}

func TestReconcile_UnreachableSkipsEvenWithFreshEvent(t *testing.T) {
	now := metav1.NewTime(time.Now().Truncate(time.Second))
	sw := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "sw1"},
		Spec: lumenetesv1alpha1.SwitchSpec{
			Bindings: []lumenetesv1alpha1.SwitchBinding{
				{Event: "short_release", Action: lumenetesv1alpha1.SwitchAction{TargetLights: []string{"light1"}, Toggle: true}},
			},
		},
		Status: lumenetesv1alpha1.SwitchStatus{
			Reachable:     false,
			LastEvent:     "short_release",
			LastEventTime: now,
		},
	}
	light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light1"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, sw, light)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "sw1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if got := getLight(t, c, "light1"); got.Spec.On {
		t.Errorf("light Spec.On = true, want untouched: stale status (unreachable) must not be trusted")
	}
}

func TestReconcile_RepeatEventDeliveryIsNoOp(t *testing.T) {
	// LastEventTime == LastHandledEventTime: isNewEvent is false, so this
	// must be a complete no-op, including no Status write at all.
	same := metav1.NewTime(time.Now().Truncate(time.Second))
	sw := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "sw1"},
		Spec: lumenetesv1alpha1.SwitchSpec{
			Bindings: []lumenetesv1alpha1.SwitchBinding{
				{Event: "short_release", Action: lumenetesv1alpha1.SwitchAction{TargetLights: []string{"light1"}, Toggle: true}},
			},
		},
		Status: lumenetesv1alpha1.SwitchStatus{
			Reachable:            true,
			LastEvent:            "short_release",
			LastEventTime:        same,
			LastHandledEventTime: same,
		},
	}
	light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light1"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, sw, light)
	r := &Reconciler{Client: c}

	before := getSwitch(t, c, "sw1")

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "sw1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if got := getLight(t, c, "light1"); got.Spec.On {
		t.Errorf("light Spec.On = true, want untouched: event isn't new")
	}
	after := getSwitch(t, c, "sw1")
	if after.ResourceVersion != before.ResourceVersion {
		t.Errorf("switch ResourceVersion changed (%s -> %s), want no Status write for a repeat event delivery", before.ResourceVersion, after.ResourceVersion)
	}
}

// TestReconcile_NoMatchingBindingStillAdvancesBookkeeping also verifies the
// "pre-loop snapshot" property of the RetryOnConflict write: Reconcile
// computes LastHandledEventTime from the in-memory Switch it already Got at
// the top of the function (sw.Status.LastEventTime), not from re-Getting the
// object after the binding loop - there's nothing in this Switch's own
// Status that the binding loop could have changed, so this is exactly the
// value asserted below.
func TestReconcile_NoMatchingBindingStillAdvancesBookkeeping(t *testing.T) {
	handledAt := metav1.NewTime(time.Now().Truncate(time.Second).Add(-time.Hour))
	eventAt := metav1.NewTime(time.Now().Truncate(time.Second))
	sw := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "sw1"},
		Spec: lumenetesv1alpha1.SwitchSpec{
			Bindings: []lumenetesv1alpha1.SwitchBinding{
				{Event: "long_press", Action: lumenetesv1alpha1.SwitchAction{TargetLights: []string{"light1"}, Toggle: true}},
			},
		},
		Status: lumenetesv1alpha1.SwitchStatus{
			Reachable:            true,
			LastEvent:            "short_release", // doesn't match the only binding's "long_press"
			LastEventTime:        eventAt,
			LastHandledEventTime: handledAt,
		},
	}
	light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light1"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, sw, light)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "sw1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if got := getLight(t, c, "light1"); got.Spec.On {
		t.Errorf("light Spec.On = true, want untouched: no binding matched the event")
	}
	got := getSwitch(t, c, "sw1")
	if !got.Status.LastHandledEventTime.Time.Equal(eventAt.Time) {
		t.Errorf("LastHandledEventTime = %v, want advanced to %v even though no binding matched", got.Status.LastHandledEventTime.Time, eventAt.Time)
	}
}

func TestReconcile_MatchingBindingAppliesToLight(t *testing.T) {
	eventAt := metav1.NewTime(time.Now().Truncate(time.Second))
	sw := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "sw1"},
		Spec: lumenetesv1alpha1.SwitchSpec{
			Bindings: []lumenetesv1alpha1.SwitchBinding{
				{Event: "short_release", Action: lumenetesv1alpha1.SwitchAction{TargetLights: []string{"light1"}, Toggle: true}},
			},
		},
		Status: lumenetesv1alpha1.SwitchStatus{
			Reachable:     true,
			LastEvent:     "short_release",
			LastEventTime: eventAt,
		},
	}
	light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light1"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, sw, light)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "sw1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if got := getLight(t, c, "light1"); !got.Spec.On {
		t.Errorf("light Spec.On = false, want toggled to true by the matching binding")
	}
	got := getSwitch(t, c, "sw1")
	if !got.Status.LastHandledEventTime.Time.Equal(eventAt.Time) {
		t.Errorf("LastHandledEventTime = %v, want %v", got.Status.LastHandledEventTime.Time, eventAt.Time)
	}
}

func TestReconcile_MissingTargetLightDoesNotBlockOthers(t *testing.T) {
	eventAt := metav1.NewTime(time.Now().Truncate(time.Second))
	sw := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "sw1"},
		Spec: lumenetesv1alpha1.SwitchSpec{
			Bindings: []lumenetesv1alpha1.SwitchBinding{
				{Event: "short_release", Action: lumenetesv1alpha1.SwitchAction{TargetLights: []string{"missing-light", "light1"}, Toggle: true}},
			},
		},
		Status: lumenetesv1alpha1.SwitchStatus{
			Reachable:     true,
			LastEvent:     "short_release",
			LastEventTime: eventAt,
		},
	}
	light := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light1"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, sw, light)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "sw1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil: a missing target light must not fail the whole Reconcile", err)
	}

	if got := getLight(t, c, "light1"); !got.Spec.On {
		t.Errorf("light Spec.On = false, want toggled to true despite the other target light being missing")
	}
	got := getSwitch(t, c, "sw1")
	if !got.Status.LastHandledEventTime.Time.Equal(eventAt.Time) {
		t.Errorf("LastHandledEventTime = %v, want advanced to %v even though one target light was missing", got.Status.LastHandledEventTime.Time, eventAt.Time)
	}
}

func TestReconcile_MultipleMatchingBindingsAllApplied(t *testing.T) {
	eventAt := metav1.NewTime(time.Now().Truncate(time.Second))
	sw := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "sw1"},
		Spec: lumenetesv1alpha1.SwitchSpec{
			Bindings: []lumenetesv1alpha1.SwitchBinding{
				{Event: "short_release", Action: lumenetesv1alpha1.SwitchAction{TargetLights: []string{"light1"}, Toggle: true}},
				{Event: "short_release", Action: lumenetesv1alpha1.SwitchAction{TargetLights: []string{"light2"}, On: boolPtr(true)}},
			},
		},
		Status: lumenetesv1alpha1.SwitchStatus{
			Reachable:     true,
			LastEvent:     "short_release",
			LastEventTime: eventAt,
		},
	}
	light1 := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light1"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	light2 := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "light2"}, Spec: lumenetesv1alpha1.LightSpec{On: false}}
	c := newFakeClient(t, sw, light1, light2)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "sw1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if got := getLight(t, c, "light1"); !got.Spec.On {
		t.Errorf("light1 Spec.On = false, want toggled to true by the first binding")
	}
	if got := getLight(t, c, "light2"); !got.Spec.On {
		t.Errorf("light2 Spec.On = false, want set to true by the second binding")
	}
}

// TestReconcile_HandledAtSurvivesOneConflictAndKeepsPreLoopSnapshot forces an
// actual write conflict on the RetryOnConflict path (simulating Streamer
// racing to update the same Switch's Status concurrently) and checks two
// things at once: the retry actually absorbs the conflict and succeeds, and
// the LastHandledEventTime it ultimately persists is the value snapshotted
// before the retry loop began - not something recomputed from whatever the
// retried Get-then-Update saw.
func TestReconcile_HandledAtSurvivesOneConflictAndKeepsPreLoopSnapshot(t *testing.T) {
	eventAt := metav1.NewTime(time.Now().Truncate(time.Second))
	sw := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "sw1"},
		Status: lumenetesv1alpha1.SwitchStatus{
			Reachable:     true,
			LastEvent:     "short_release",
			LastEventTime: eventAt,
		},
	}

	var attempts int
	c := newFakeClientWithInterceptors(t, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, cl client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			attempts++
			if attempts == 1 {
				// Simulate a genuine conflict: some other writer (e.g.
				// EventConsumer) updated this Switch's Status between
				// Reconcile's Get and this Update.
				return apierrors.NewConflict(schema.GroupResource{Group: "lumenetes.io", Resource: "switches"}, obj.GetName(), fmt.Errorf("simulated concurrent write"))
			}
			return cl.Status().Update(ctx, obj, opts...)
		},
	}, sw)
	r := &Reconciler{Client: c}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "sw1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want RetryOnConflict to absorb the simulated conflict and succeed", err)
	}
	if attempts < 2 {
		t.Fatalf("Status().Update was attempted %d time(s), want at least 2 (one simulated conflict, then a retry that succeeds)", attempts)
	}

	got := getSwitch(t, c, "sw1")
	if !got.Status.LastHandledEventTime.Time.Equal(eventAt.Time) {
		t.Errorf("LastHandledEventTime = %v, want %v: the pre-loop snapshot, unaffected by the retried write", got.Status.LastHandledEventTime.Time, eventAt.Time)
	}
}

func boolPtr(b bool) *bool    { return &b }
func int32Ptr(v int32) *int32 { return &v }
func strPtr(s string) *string { return &s }

func TestApplyActionToSpec(t *testing.T) {
	baseline := lumenetesv1alpha1.LightSpec{
		Name: "Kitchen", On: false, Brightness: 50, Color: "#ffedbb", ColorTempK: 2700,
	}
	noDimming := lumenetesv1alpha1.LightSpec{Name: "Fan Light", On: true, Brightness: -1, Color: "", ColorTempK: 0}

	cases := []struct {
		name    string
		current lumenetesv1alpha1.LightSpec
		action  lumenetesv1alpha1.SwitchAction
		want    lumenetesv1alpha1.LightSpec
	}{
		{
			name:    "on true",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{On: boolPtr(true)},
			want:    withOn(baseline, true),
		},
		{
			name:    "on false",
			current: withOn(baseline, true),
			action:  lumenetesv1alpha1.SwitchAction{On: boolPtr(false)},
			want:    baseline,
		},
		{
			name:    "toggle flips true to false",
			current: withOn(baseline, true),
			action:  lumenetesv1alpha1.SwitchAction{Toggle: true},
			want:    baseline,
		},
		{
			name:    "toggle flips false to true",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{Toggle: true},
			want:    withOn(baseline, true),
		},
		{
			name:    "brightness set in range",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{Brightness: int32Ptr(80)},
			want:    withBrightness(baseline, 80),
		},
		{
			name:    "brightness set clamped above 100",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{Brightness: int32Ptr(150)},
			want:    withBrightness(baseline, 100),
		},
		{
			name:    "brightness set clamped below 0",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{Brightness: int32Ptr(-20)},
			want:    withBrightness(baseline, 0),
		},
		{
			name:    "brightness set no-op on non-dimmable light",
			current: noDimming,
			action:  lumenetesv1alpha1.SwitchAction{Brightness: int32Ptr(50)},
			want:    noDimming,
		},
		{
			name:    "brightness delta positive with clamping at 100",
			current: withBrightness(baseline, 95),
			action:  lumenetesv1alpha1.SwitchAction{BrightnessDelta: int32Ptr(10)},
			want:    withBrightness(baseline, 100),
		},
		{
			name:    "brightness delta negative with clamping at 0",
			current: withBrightness(baseline, 5),
			action:  lumenetesv1alpha1.SwitchAction{BrightnessDelta: int32Ptr(-10)},
			want:    withBrightness(baseline, 0),
		},
		{
			name:    "brightness delta no-op on non-dimmable light",
			current: noDimming,
			action:  lumenetesv1alpha1.SwitchAction{BrightnessDelta: int32Ptr(10)},
			want:    noDimming,
		},
		{
			name:    "color set on color-capable light",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{Color: strPtr("#ff0000")},
			want:    withColor(baseline, "#ff0000"),
		},
		{
			name:    "color set no-op on non-color light",
			current: noDimming,
			action:  lumenetesv1alpha1.SwitchAction{Color: strPtr("#ff0000")},
			want:    noDimming,
		},
		{
			name:    "colorTempK set on capable light",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{ColorTempK: int32Ptr(4000)},
			want:    withColorTempK(baseline, 4000),
		},
		{
			name:    "colorTempK set no-op when unsupported",
			current: noDimming,
			action:  lumenetesv1alpha1.SwitchAction{ColorTempK: int32Ptr(4000)},
			want:    noDimming,
		},
		{
			name:    "combined on and brightnessDelta",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{On: boolPtr(true), BrightnessDelta: int32Ptr(10)},
			want:    withBrightness(withOn(baseline, true), 60),
		},
		{
			name:    "nil action is a true no-op",
			current: baseline,
			action:  lumenetesv1alpha1.SwitchAction{},
			want:    baseline,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyActionToSpec(tc.current, tc.action)
			if got != tc.want {
				t.Errorf("applyActionToSpec() = %+v, want %+v", got, tc.want)
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

func TestIsNewEvent(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		lastEvent   time.Time
		lastHandled time.Time
		want        bool
	}{
		{"zero lastEvent", time.Time{}, base, false},
		{"lastEvent after lastHandled", base.Add(time.Second), base, true},
		{"equal timestamps", base, base, false},
		{"lastEvent before lastHandled (stale/out-of-order)", base, base.Add(time.Second), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNewEvent(tc.lastEvent, tc.lastHandled); got != tc.want {
				t.Errorf("isNewEvent() = %v, want %v", got, tc.want)
			}
		})
	}
}
