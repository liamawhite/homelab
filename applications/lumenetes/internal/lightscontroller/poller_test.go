package lightscontroller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/homelab/pkg/lumenetes/hue"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/bridges"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// testLightResource mirrors the unexported lightResource shape in
// pkg/lumenetes/hue/lights.go closely enough to drive FetchLights against a
// local httptest server.
type testLightResource struct {
	ID    string `json:"id"`
	Owner struct {
		RID   string `json:"rid"`
		RType string `json:"rtype"`
	} `json:"owner"`
	Metadata struct {
		Name      string `json:"name"`
		Archetype string `json:"archetype"`
	} `json:"metadata"`
	On struct {
		On bool `json:"on"`
	} `json:"on"`
}

func newLightsServer(t *testing.T, lights []testLightResource) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/clip/v2/resource/light", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": lights})
	})
	mux.HandleFunc("/clip/v2/resource/device", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func serverIP(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "https://")
}

func TestPoller_SyncBridge_MissingHueBridge(t *testing.T) {
	existing := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "stale-light", Labels: map[string]string{bridgeIDLabel: "BRIDGE1"}},
		Status:     lumenetesv1alpha1.LightStatus{Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(existing).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	p := &Poller{Client: fakeClient}

	p.syncBridge(context.Background(), logr.Discard(), bridges.Config{ID: "BRIDGE1"})

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "stale-light"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.Reachable {
		t.Error("got Reachable = true, want false since the HueBridge CR is missing")
	}
}

func TestPoller_SyncBridge_UnreachableHueBridge(t *testing.T) {
	existing := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "stale-light", Labels: map[string]string{bridgeIDLabel: "BRIDGE1"}},
		Status:     lumenetesv1alpha1.LightStatus{Reachable: true},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{Reachable: false},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(existing, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	p := &Poller{Client: fakeClient}

	p.syncBridge(context.Background(), logr.Discard(), bridges.Config{ID: "BRIDGE1"})

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "stale-light"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status.Reachable {
		t.Error("got Reachable = true, want false since the HueBridge is not reachable")
	}
}

func TestPoller_SyncBridge_FetchLightsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/clip/v2/resource/light", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	existing := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "stale-light", Labels: map[string]string{bridgeIDLabel: "BRIDGE1"}},
		Status:     lumenetesv1alpha1.LightStatus{Reachable: true},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: serverIP(srv), Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(existing, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	p := &Poller{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}}

	p.syncBridge(context.Background(), logr.Discard(), bridges.Config{ID: "BRIDGE1", AppKey: "key1"})

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "stale-light"}, &got); err != nil {
		t.Fatalf("Get() error = %v, want the light to still exist (not GC'd on fetch failure)", err)
	}
	if got.Status.Reachable {
		t.Error("got Reachable = true, want false after a fetch failure")
	}
}

func TestPoller_SyncBridge_Success(t *testing.T) {
	lightA := testLightResource{ID: "light-a"}
	lightA.Metadata.Name = "Light A"
	lightB := testLightResource{ID: "light-b"}
	lightB.Metadata.Name = "Light B"
	srv := newLightsServer(t, []testLightResource{lightA, lightB})

	stale := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "stale-light", Labels: map[string]string{bridgeIDLabel: "BRIDGE1"}},
	}
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("BRIDGE1")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: serverIP(srv), Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(stale, hueBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	p := &Poller{Client: fakeClient, Bridges: []bridges.Config{{ID: "BRIDGE1", AppKey: "key1"}}}

	p.syncBridge(context.Background(), logr.Discard(), bridges.Config{ID: "BRIDGE1", AppKey: "key1"})

	for _, name := range []string{"light-a", "light-b"} {
		var got lumenetesv1alpha1.Light
		if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: name}, &got); err != nil {
			t.Errorf("Get(%s) error = %v, want it created", name, err)
		}
	}

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "stale-light"}, &got); !apierrors.IsNotFound(err) {
		t.Errorf("Get(stale-light) error = %v, want NotFound (should have been GC'd)", err)
	}
}

func TestPoller_Upsert_NewLight(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	p := &Poller{Client: fakeClient}

	fetched := lighthue.Light{
		ID: "light-1", Name: "Kitchen", BridgeID: "BRIDGE1", DeviceID: "dev-1",
		On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700,
		FixtureType: "table shade", Product: "Hue White", Model: "LCT007",
	}
	p.upsert(context.Background(), logr.Discard(), fetched)

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	wantSpec := lumenetesv1alpha1.LightSpec{Name: "Kitchen", On: true, Brightness: 50, Color: "#ffffff", ColorTempK: 2700}
	if got.Spec != wantSpec {
		t.Errorf("got Spec = %+v, want %+v", got.Spec, wantSpec)
	}
	if got.Status.Name != "Kitchen" || !got.Status.On || got.Status.Brightness != 50 || !got.Status.Reachable {
		t.Errorf("got Status = %+v, want it seeded from fetched values with Reachable=true", got.Status)
	}
	if got.Labels[bridgeIDLabel] != "BRIDGE1" {
		t.Errorf("got label %s = %q, want BRIDGE1", bridgeIDLabel, got.Labels[bridgeIDLabel])
	}
}

func TestPoller_Upsert_ExistingLight_SpecUntouched(t *testing.T) {
	originalSpec := lumenetesv1alpha1.LightSpec{Name: "Old Name", On: true, Brightness: 10, Color: "#000000", ColorTempK: 2000}
	existing := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "light-1",
			CreationTimestamp: metav1.Now(),
			Labels:            map[string]string{bridgeIDLabel: "BRIDGE1"},
		},
		Spec: originalSpec,
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(existing).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	p := &Poller{Client: fakeClient}

	fetched := lighthue.Light{
		ID: "light-1", Name: "New Name", BridgeID: "BRIDGE1",
		On: false, Brightness: 99, Color: "#ffffff", ColorTempK: 5000,
	}
	p.upsert(context.Background(), logr.Discard(), fetched)

	var got lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "light-1"}, &got); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Spec != originalSpec {
		t.Errorf("got Spec = %+v, want unchanged from seed %+v (seed-once guard)", got.Spec, originalSpec)
	}
	if got.Status.Name != "New Name" || got.Status.On != false || got.Status.Brightness != 99 || !got.Status.Reachable {
		t.Errorf("got Status = %+v, want it updated to the freshly-fetched values", got.Status)
	}
}

func TestPoller_GC(t *testing.T) {
	keep := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "keep-me", Labels: map[string]string{bridgeIDLabel: "BRIDGE1"}}}
	remove := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "delete-me", Labels: map[string]string{bridgeIDLabel: "BRIDGE1"}}}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(keep, remove).Build()
	p := &Poller{Client: fakeClient}

	p.gc(context.Background(), logr.Discard(), "BRIDGE1", map[string]bool{"keep-me": true})

	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "keep-me"}, &lumenetesv1alpha1.Light{}); err != nil {
		t.Errorf("Get(keep-me) error = %v, want it to still exist", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "delete-me"}, &lumenetesv1alpha1.Light{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(delete-me) error = %v, want NotFound", err)
	}

	// Second gc call with the same (now-empty) bridge should be a no-op,
	// tolerating the fact that "delete-me" is already gone.
	p.gc(context.Background(), logr.Discard(), "BRIDGE1", map[string]bool{"keep-me": true})
}

func TestPoller_GC_ToleratesNotFoundOnDelete(t *testing.T) {
	stale := &lumenetesv1alpha1.Light{ObjectMeta: metav1.ObjectMeta{Name: "already-gone", Labels: map[string]string{bridgeIDLabel: "BRIDGE1"}}}
	fakeClient := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithObjects(stale).
		WithInterceptorFuncs(interceptor.Funcs{
			// Force a real NotFound from the Delete call itself (rather
			// than just having nothing left to delete), so gc's
			// apierrors.IsNotFound tolerance is actually exercised.
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				return apierrors.NewNotFound(lumenetesv1alpha1.GroupVersion.WithResource("lights").GroupResource(), obj.GetName())
			},
		}).
		Build()
	p := &Poller{Client: fakeClient}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("gc panicked on a NotFound delete: %v", r)
		}
	}()
	p.gc(context.Background(), logr.Discard(), "BRIDGE1", map[string]bool{})
}

func TestPoller_MarkUnreachable(t *testing.T) {
	bridge1Light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "bridge1-light", Labels: map[string]string{bridgeIDLabel: "BRIDGE1"}},
		Status:     lumenetesv1alpha1.LightStatus{Reachable: true, Name: "keep-metadata"},
	}
	bridge2Light := &lumenetesv1alpha1.Light{
		ObjectMeta: metav1.ObjectMeta{Name: "bridge2-light", Labels: map[string]string{bridgeIDLabel: "BRIDGE2"}},
		Status:     lumenetesv1alpha1.LightStatus{Reachable: true},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(bridge1Light, bridge2Light).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	p := &Poller{Client: fakeClient}

	p.markUnreachable(context.Background(), logr.Discard(), "BRIDGE1")

	var got1 lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "bridge1-light"}, &got1); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got1.Status.Reachable || got1.Status.Name != "keep-metadata" {
		t.Errorf("got Status = %+v, want Reachable=false with other fields untouched", got1.Status)
	}

	var got2 lumenetesv1alpha1.Light
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "bridge2-light"}, &got2); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !got2.Status.Reachable {
		t.Error("got bridge2-light Reachable = false, want untouched (different bridge)")
	}

	// Idempotent: already-unreachable light should not get a redundant write.
	var before lumenetesv1alpha1.Light
	_ = fakeClient.Get(context.Background(), client.ObjectKey{Name: "bridge1-light"}, &before)
	p.markUnreachable(context.Background(), logr.Discard(), "BRIDGE1")
	var after lumenetesv1alpha1.Light
	_ = fakeClient.Get(context.Background(), client.ObjectKey{Name: "bridge1-light"}, &after)
	if before.ResourceVersion != after.ResourceVersion {
		t.Errorf("resourceVersion changed on an already-unreachable light, want no redundant write")
	}
}

func TestPoller_Sync_OneBadBridgeDoesNotBlockAnother(t *testing.T) {
	good := testLightResource{ID: "good-light"}
	good.Metadata.Name = "Good Light"
	srv := newLightsServer(t, []testLightResource{good})

	goodBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("GOOD")},
		Status:     lumenetesv1alpha1.HueBridgeStatus{IP: serverIP(srv), Reachable: true},
	}
	// BAD's HueBridge CR simply doesn't exist.
	fakeClient := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(goodBridge).WithStatusSubresource(&lumenetesv1alpha1.Light{}).Build()
	p := &Poller{
		Client: fakeClient,
		Bridges: []bridges.Config{
			{ID: "BAD", AppKey: "key-bad"},
			{ID: "GOOD", AppKey: "key-good"},
		},
	}

	p.sync(context.Background(), logr.Discard())

	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "good-light"}, &lumenetesv1alpha1.Light{}); err != nil {
		t.Errorf("Get(good-light) error = %v, want it created despite BAD's failure", err)
	}
}
