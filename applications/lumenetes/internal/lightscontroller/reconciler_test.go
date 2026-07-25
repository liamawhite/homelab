package lightscontroller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/bridges"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := lumenetesv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	return scheme
}

func TestDiffLight(t *testing.T) {
	synced := lumenetesv1alpha1.LightSpec{Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700}
	syncedStatus := lumenetesv1alpha1.LightStatus{Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700}

	cases := []struct {
		name   string
		spec   lumenetesv1alpha1.LightSpec
		status lumenetesv1alpha1.LightStatus
		want   []string
	}{
		{
			name:   "no drift",
			spec:   synced,
			status: syncedStatus,
			want:   nil,
		},
		{
			name:   "name differs",
			spec:   lumenetesv1alpha1.LightSpec{Name: "Lounge"},
			status: lumenetesv1alpha1.LightStatus{Name: "Kitchen"},
			want:   []string{"name"},
		},
		{
			name:   "on differs",
			spec:   lumenetesv1alpha1.LightSpec{On: true},
			status: lumenetesv1alpha1.LightStatus{On: false},
			want:   []string{"on"},
		},
		{
			name:   "brightness differs",
			spec:   lumenetesv1alpha1.LightSpec{Brightness: 80},
			status: lumenetesv1alpha1.LightStatus{Brightness: 50},
			want:   []string{"brightness"},
		},
		{
			name:   "color differs",
			spec:   lumenetesv1alpha1.LightSpec{Color: "#ff0000"},
			status: lumenetesv1alpha1.LightStatus{Color: "#00ff00"},
			want:   []string{"color"},
		},
		{
			name:   "colorTempK differs",
			spec:   lumenetesv1alpha1.LightSpec{ColorTempK: 4000},
			status: lumenetesv1alpha1.LightStatus{ColorTempK: 2700},
			want:   []string{"colorTempK"},
		},
		{
			name:   "multi-field drift",
			spec:   lumenetesv1alpha1.LightSpec{Name: "Lounge", On: false, Brightness: 50, Color: "#ffffff", ColorTempK: 2700},
			status: syncedStatus,
			want:   []string{"name", "on"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := diffLight(tc.spec, tc.status)
			var gotFields []string
			for _, d := range got {
				gotFields = append(gotFields, d.Field)
			}
			if !slices.Equal(gotFields, tc.want) {
				t.Errorf("diffLight() fields = %v, want %v", gotFields, tc.want)
			}
		})
	}
}

func TestHasField(t *testing.T) {
	diffs := []fieldDiff{{Field: "on"}, {Field: "color"}}
	if !hasField(diffs, "on") {
		t.Error("hasField(diffs, \"on\") = false, want true")
	}
	if hasField(diffs, "brightness") {
		t.Error("hasField(diffs, \"brightness\") = true, want false")
	}
	if hasField(nil, "on") {
		t.Error("hasField(nil, \"on\") = true, want false")
	}
}

func TestBridgesFindByID(t *testing.T) {
	cfgs := []bridges.Config{{ID: "abc", AppKey: "key1"}, {ID: "def", AppKey: "key2"}}

	got, ok := bridges.FindByID(cfgs, "def")
	if !ok || got.AppKey != "key2" {
		t.Errorf("FindByID(cfgs, \"def\") = %+v, %v, want key2, true", got, ok)
	}
	if _, ok := bridges.FindByID(cfgs, "missing"); ok {
		t.Error("FindByID(cfgs, \"missing\") = true, want false")
	}
}

func TestReconcile_LightNotFound(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()
	r := &Reconciler{Client: fakeClient}

	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "missing"}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}
	if res != (ctrl.Result{}) {
		t.Errorf("Reconcile() result = %+v, want zero value", res)
	}
}

func TestReconcile_StatusUnreachable_ShortCircuits(t *testing.T) {
	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true},
		Status:     lumenetesv1alpha1.LightStatus{On: false, Reachable: false},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.EnactError != "" || !got.Status.LastEnactAttempt.IsZero() {
		t.Errorf("got Status = %+v, want no enactment attempted", got.Status)
	}
}

func TestReconcile_NoDiff_NoWrite(t *testing.T) {
	synced := lumenetesv1alpha1.LightSpec{Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700}
	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       synced,
		Status:     lumenetesv1alpha1.LightStatus{Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700, Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient}

	var before lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &before); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var after lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &after); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if before.ResourceVersion != after.ResourceVersion {
		t.Errorf("resourceVersion changed from %s to %s, want no write when there's no diff", before.ResourceVersion, after.ResourceVersion)
	}
}

func TestReconcile_NoDiff_ClearsStaleEnactError(t *testing.T) {
	synced := lumenetesv1alpha1.LightSpec{Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700}
	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       synced,
		Status: lumenetesv1alpha1.LightStatus{
			Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700,
			Reachable: true, EnactError: "stale error from a previous attempt",
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.EnactError != "" {
		t.Errorf("got EnactError = %q, want cleared", got.Status.EnactError)
	}
}

func TestReconcile_DryRun_NoBridgeCallNoStatusWrite(t *testing.T) {
	var hit atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	ip := strings.TrimPrefix(srv.URL, "https://")

	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true, Brightness: -1},
		Status:     lumenetesv1alpha1.LightStatus{On: false, Brightness: -1, Reachable: true, BridgeID: "BRIDGE1"},
	}
	// A fully-resolvable bridge is wired in on purpose - if DryRun ever
	// regressed into calling enact, this would be reachable, and hit would
	// flip true.
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: ip, Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}, DryRun: true}

	var before lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &before); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.EnactError != "" || !got.Status.LastEnactAttempt.IsZero() {
		t.Errorf("got Status = %+v, want no enactment attempted in dry-run", got.Status)
	}
	if got.ResourceVersion != before.ResourceVersion {
		t.Errorf("resourceVersion changed from %s to %s, want no status write in dry-run", before.ResourceVersion, got.ResourceVersion)
	}
	if hit.Load() {
		t.Error("bridge endpoint was called during a dry run, want no call at all")
	}
}

func TestReconcile_StateDiff_BrightnessColorColorTempK(t *testing.T) {
	var mu sync.Mutex
	var putBody map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/clip/v2/resource/light/light-1", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
			t.Errorf("failed to decode PUT body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	ip := strings.TrimPrefix(srv.URL, "https://")

	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true, Brightness: 80, Color: "#ff0000", ColorTempK: 4000},
		Status: lumenetesv1alpha1.LightStatus{
			On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700, Reachable: true, BridgeID: "BRIDGE1",
		},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: ip, Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	mu.Lock()
	defer mu.Unlock()
	dimming, _ := putBody["dimming"].(map[string]any)
	if dimming == nil || dimming["brightness"] != float64(80) {
		t.Errorf("PUT body dimming = %v, want brightness 80", dimming)
	}
	if _, present := putBody["color"]; !present {
		t.Errorf("PUT body missing color, want it present for a non-empty spec.Color")
	}
	colorTemperature, _ := putBody["color_temperature"].(map[string]any)
	if colorTemperature == nil || colorTemperature["mirek"] != float64(250) {
		t.Errorf("PUT body color_temperature = %v, want mirek 250 (1_000_000/4000, the documented Kelvin<->mirek formula)", colorTemperature)
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.EnactError != "" {
		t.Errorf("got EnactError = %q, want cleared on success", got.Status.EnactError)
	}
}

func TestReconcile_ResolveBridgeFails_SetsEnactError(t *testing.T) {
	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true, Brightness: -1},
		Status:     lumenetesv1alpha1.LightStatus{On: false, Brightness: -1, Reachable: true, BridgeID: "BRIDGE1"},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient} // no HueBridge CR at all

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}})
	if err == nil {
		t.Fatal("Reconcile() error = nil, want an error to trigger requeue")
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.EnactError == "" {
		t.Error("got EnactError = \"\", want it set to the resolveBridge failure")
	}
	if got.Status.LastEnactAttempt.IsZero() {
		t.Error("got LastEnactAttempt zero, want it set")
	}
}

func TestReconcile_ResolveBridgeFails_HueBridgeUnreachable(t *testing.T) {
	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true, Brightness: -1},
		Status:     lumenetesv1alpha1.LightStatus{On: false, Brightness: -1, Reachable: true, BridgeID: "BRIDGE1"},
	}
	// HueBridge CR exists but is marked unreachable - a different failure
	// mode from the CR being missing entirely (see
	// TestReconcile_ResolveBridgeFails_SetsEnactError).
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{Reachable: false},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}})
	if err == nil {
		t.Fatal("Reconcile() error = nil, want an error")
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !strings.Contains(got.Status.EnactError, "not currently reachable") {
		t.Errorf("got EnactError = %q, want it to mention the bridge is not currently reachable", got.Status.EnactError)
	}
}

func TestReconcile_ResolveBridgeFails_NoPairedConfig(t *testing.T) {
	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true, Brightness: -1},
		Status:     lumenetesv1alpha1.LightStatus{On: false, Brightness: -1, Reachable: true, BridgeID: "BRIDGE1"},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: "203.0.113.1", Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient} // Bridges is nil - no paired config for BRIDGE1

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}})
	if err == nil {
		t.Fatal("Reconcile() error = nil, want an error")
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !strings.Contains(got.Status.EnactError, "no paired bridge config") {
		t.Errorf("got EnactError = %q, want it to mention no paired bridge config", got.Status.EnactError)
	}
}

func TestReconcile_StateDiff_CallsUpdateLight(t *testing.T) {
	var mu sync.Mutex
	var putBody map[string]any
	var putCalled bool

	mux := http.NewServeMux()
	mux.HandleFunc("/clip/v2/resource/light/light-1", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		putCalled = true
		if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
			t.Errorf("failed to decode PUT body: %v", err)
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	ip := strings.TrimPrefix(srv.URL, "https://")

	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{On: true, Brightness: -1},
		Status:     lumenetesv1alpha1.LightStatus{On: false, Brightness: -1, Reachable: true, BridgeID: "BRIDGE1"},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: ip, Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !putCalled {
		t.Fatal("UpdateLight PUT was never made")
	}
	onField, _ := putBody["on"].(map[string]any)
	if onField["on"] != true {
		t.Errorf("PUT body on.on = %v, want true", onField["on"])
	}
	if _, present := putBody["dimming"]; present {
		t.Errorf("PUT body unexpectedly included dimming: %v", putBody)
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.EnactError != "" {
		t.Errorf("got EnactError = %q, want cleared on success", got.Status.EnactError)
	}
}

func TestReconcile_Rename_NoDeviceID(t *testing.T) {
	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{Name: "New Name", On: false, Brightness: -1},
		Status: lumenetesv1alpha1.LightStatus{
			Name: "Old Name", On: false, Brightness: -1, Reachable: true, BridgeID: "BRIDGE1", DeviceID: "",
		},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: "203.0.113.1", Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}})
	if err == nil {
		t.Fatal("Reconcile() error = nil, want an error since rename can't be enacted without a device id")
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !strings.Contains(got.Status.EnactError, "no device id known") {
		t.Errorf("got EnactError = %q, want it to mention no device id known", got.Status.EnactError)
	}
}

func TestReconcile_Rename_CallsRenameDevice(t *testing.T) {
	var mu sync.Mutex
	var putBody map[string]any
	var putPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/clip/v2/resource/device/dev-1", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		putPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
			t.Errorf("failed to decode PUT body: %v", err)
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	ip := strings.TrimPrefix(srv.URL, "https://")

	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{Name: "New Name", On: false, Brightness: -1},
		Status: lumenetesv1alpha1.LightStatus{
			Name: "Old Name", On: false, Brightness: -1, Reachable: true, BridgeID: "BRIDGE1", DeviceID: "dev-1",
		},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: ip, Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}}); err != nil {
		t.Fatalf("Reconcile() error = %v, want nil", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if putPath != "/clip/v2/resource/device/dev-1" {
		t.Fatalf("RenameDevice PUT path = %q, want /clip/v2/resource/device/dev-1", putPath)
	}
	metadata, _ := putBody["metadata"].(map[string]any)
	if metadata["name"] != "New Name" {
		t.Errorf("PUT body metadata.name = %v, want New Name", metadata["name"])
	}
}

func TestReconcile_CombinedDiff_PartialFailureStillErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/clip/v2/resource/light/light-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/clip/v2/resource/device/dev-1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"description":"boom"}]}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	ip := strings.TrimPrefix(srv.URL, "https://")

	light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "light-1"},
		Spec:       lumenetesv1alpha1.LightSpec{Name: "New Name", On: true, Brightness: -1},
		Status: lumenetesv1alpha1.LightStatus{
			Name: "Old Name", On: false, Brightness: -1, Reachable: true, BridgeID: "BRIDGE1", DeviceID: "dev-1",
		},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: ip, Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(light, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	r := &Reconciler{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}}

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: "light-1"}})
	if err == nil {
		t.Fatal("Reconcile() error = nil, want the rename failure to surface even though the state update succeeded")
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !strings.Contains(got.Status.EnactError, "device rename") {
		t.Errorf("got EnactError = %q, want it to mention device rename", got.Status.EnactError)
	}
	if strings.Contains(got.Status.EnactError, "light update") {
		t.Errorf("got EnactError = %q, want it to NOT mention light update since that call succeeded", got.Status.EnactError)
	}
}
