package switchcontroller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/homelab/pkg/lumenetes/hue"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/bridges"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMergedSwitchStatus(t *testing.T) {
	now := metav1.NewTime(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	older := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 1, 11, 30, 0, 0, time.UTC)

	t.Run("polled event older than current keeps current's event fields", func(t *testing.T) {
		current := lumenetesv1alpha1.SwitchStatus{
			LastEvent:            "long_press",
			LastEventTime:        metav1.NewTime(newer),
			LastHandledEventTime: metav1.NewTime(newer),
		}
		polled := lighthue.Switch{Name: "Lounge", LastEvent: "short_release", LastEventTime: older}

		got := mergedSwitchStatus(current, polled, now)

		if got.LastEvent != "long_press" || !got.LastEventTime.Time.Equal(newer) {
			t.Errorf("got LastEvent=%q LastEventTime=%v, want current's untouched (long_press, %v)", got.LastEvent, got.LastEventTime.Time, newer)
		}
		if !got.LastHandledEventTime.Time.Equal(newer) {
			t.Errorf("got LastHandledEventTime=%v, want preserved %v", got.LastHandledEventTime.Time, newer)
		}
		if got.Name != "Lounge" {
			t.Errorf("got Name=%q, want polled value Lounge", got.Name)
		}
	})

	t.Run("polled event newer than current advances event fields but preserves handled", func(t *testing.T) {
		current := lumenetesv1alpha1.SwitchStatus{
			LastEvent:            "short_release",
			LastEventTime:        metav1.NewTime(older),
			LastHandledEventTime: metav1.NewTime(older),
		}
		polled := lighthue.Switch{LastEvent: "long_press", LastEventTime: newer}

		got := mergedSwitchStatus(current, polled, now)

		if got.LastEvent != "long_press" || !got.LastEventTime.Time.Equal(newer) {
			t.Errorf("got LastEvent=%q LastEventTime=%v, want advanced to (long_press, %v)", got.LastEvent, got.LastEventTime.Time, newer)
		}
		if !got.LastHandledEventTime.Time.Equal(older) {
			t.Errorf("got LastHandledEventTime=%v, want preserved %v (not advanced)", got.LastHandledEventTime.Time, older)
		}
	})

	t.Run("polled zero LastEventTime never fired doesn't clobber current", func(t *testing.T) {
		current := lumenetesv1alpha1.SwitchStatus{
			LastEvent:     "short_release",
			LastEventTime: metav1.NewTime(newer),
		}
		polled := lighthue.Switch{LastEvent: "", LastEventTime: time.Time{}}

		got := mergedSwitchStatus(current, polled, now)

		if got.LastEvent != "short_release" || !got.LastEventTime.Time.Equal(newer) {
			t.Errorf("got LastEvent=%q LastEventTime=%v, want current's untouched", got.LastEvent, got.LastEventTime.Time)
		}
	})

	t.Run("always sets Reachable true and LastSynced", func(t *testing.T) {
		got := mergedSwitchStatus(lumenetesv1alpha1.SwitchStatus{}, lighthue.Switch{}, now)
		if !got.Reachable {
			t.Error("got Reachable=false, want true")
		}
		if !got.LastSynced.Time.Equal(now.Time) {
			t.Errorf("got LastSynced=%v, want %v", got.LastSynced.Time, now.Time)
		}
	})
}

const testBridgeID = "AABBCCDDEEFF"

func seedSwitch(name, bridgeID string, reachable bool) *lumenetesv1alpha1.Switch {
	return &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{bridgeIDLabel: bridgeID}},
		Status:     lumenetesv1alpha1.SwitchStatus{Reachable: reachable},
	}
}

func switchesHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/clip/v2/resource/button":
			fmt.Fprint(w, `{"data":[{"id":"btn-1","owner":{"rid":"dev-1"},"metadata":{"control_id":1},"button":{"button_report":{"updated":"2026-01-01T12:00:00Z","event":"short_release"}}}]}`)
		case "/clip/v2/resource/device":
			fmt.Fprint(w, `{"data":[{"id":"dev-1","metadata":{"name":"Lounge Switch"},"product_data":{"product_name":"Hue Dimmer","model_id":"RWL022"}}]}`)
		case "/clip/v2/resource/device_power":
			fmt.Fprint(w, `{"data":[{"owner":{"rid":"dev-1"},"power_state":{"battery_level":80}}]}`)
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	})
}

func TestSyncBridge_MissingHueBridge_MarksUnreachable(t *testing.T) {
	existing := seedSwitch("btn-1", testBridgeID, true)
	c := newFakeClient(t, existing)
	p := &Poller{Client: c}

	p.syncBridge(context.Background(), logr.Discard(), bridges.Config{ID: testBridgeID, AppKey: "key"})

	var got lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: "btn-1"}, &got); err != nil {
		t.Fatalf("Get switch: %v", err)
	}
	if got.Status.Reachable {
		t.Errorf("Status.Reachable = true, want false when the HueBridge CR is missing")
	}
}

func TestSyncBridge_UnreachableHueBridge_MarksUnreachable(t *testing.T) {
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName(testBridgeID)},
		Status:     lumenetesv1alpha1.HueBridgeStatus{Reachable: false},
	}
	existing := seedSwitch("btn-1", testBridgeID, true)
	c := newFakeClient(t, hueBridge, existing)
	p := &Poller{Client: c}

	p.syncBridge(context.Background(), logr.Discard(), bridges.Config{ID: testBridgeID, AppKey: "key"})

	var got lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: "btn-1"}, &got); err != nil {
		t.Fatalf("Get switch: %v", err)
	}
	if got.Status.Reachable {
		t.Errorf("Status.Reachable = true, want false when the HueBridge is not reachable")
	}
}

func TestSyncBridge_FetchSwitchesError_MarksUnreachableAndSkipsGC(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	ip := strings.TrimPrefix(srv.URL, "https://")

	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName(testBridgeID)},
		Status:     lumenetesv1alpha1.HueBridgeStatus{Reachable: true, IP: ip},
	}
	existing := seedSwitch("btn-1", testBridgeID, true)
	c := newFakeClient(t, hueBridge, existing)
	p := &Poller{Client: c}

	p.syncBridge(context.Background(), logr.Discard(), bridges.Config{ID: testBridgeID, AppKey: "key"})

	var got lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: "btn-1"}, &got); err != nil {
		t.Fatalf("Get switch: %v, want the switch to still exist (GC must be skipped on a fetch error)", err)
	}
	if got.Status.Reachable {
		t.Errorf("Status.Reachable = true, want false when FetchSwitches fails")
	}
}

func TestSyncBridge_Success_UpsertsAndGCs(t *testing.T) {
	srv := httptest.NewTLSServer(switchesHandler(t))
	defer srv.Close()
	ip := strings.TrimPrefix(srv.URL, "https://")

	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName(testBridgeID)},
		Status:     lumenetesv1alpha1.HueBridgeStatus{Reachable: true, IP: ip},
	}
	stale := seedSwitch("stale-btn", testBridgeID, true)
	c := newFakeClient(t, hueBridge, stale)
	p := &Poller{Client: c}

	p.syncBridge(context.Background(), logr.Discard(), bridges.Config{ID: testBridgeID, AppKey: "key"})

	var got lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: "btn-1"}, &got); err != nil {
		t.Fatalf("Get switch btn-1: %v, want it created by upsert", err)
	}
	if got.Status.Name != "Lounge Switch" || got.Status.Battery != 80 {
		t.Errorf("got Status=%+v, want Name=Lounge Switch Battery=80", got.Status)
	}

	var stillThere lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: "stale-btn"}, &stillThere); err == nil {
		t.Errorf("stale-btn still exists, want it GC'd since it wasn't in this cycle's fetched switches")
	}
}

func TestUpsert_NewSwitch_SpecStaysZeroValue(t *testing.T) {
	c := newFakeClient(t)
	p := &Poller{Client: c}

	fetched := lighthue.Switch{ID: "btn-1", BridgeID: testBridgeID, Name: "Lounge Switch", ControlID: 1, Battery: 80}
	p.upsert(context.Background(), logr.Discard(), fetched)

	var got lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: "btn-1"}, &got); err != nil {
		t.Fatalf("Get switch: %v", err)
	}
	if len(got.Spec.Bindings) != 0 {
		t.Errorf("Spec.Bindings = %+v, want empty: Switch bindings are pure user config, never seeded from the bridge", got.Spec.Bindings)
	}
	if got.Labels[bridgeIDLabel] != testBridgeID {
		t.Errorf("bridge-id label = %q, want %q", got.Labels[bridgeIDLabel], testBridgeID)
	}
	if got.Status.Name != "Lounge Switch" || !got.Status.Reachable {
		t.Errorf("got Status=%+v, want Name populated and Reachable=true", got.Status)
	}
}

func TestUpsert_ExistingSwitch_NeverRegressesNewerStreamerEvent(t *testing.T) {
	polledOlder := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
	streamedNewer := metav1.NewTime(time.Date(2026, 1, 1, 11, 30, 0, 0, time.UTC))
	existing := &lumenetesv1alpha1.Switch{
		ObjectMeta: metav1.ObjectMeta{Name: "btn-1"},
		Status: lumenetesv1alpha1.SwitchStatus{
			LastEvent:     "long_press",
			LastEventTime: streamedNewer,
		},
	}
	c := newFakeClient(t, existing)
	p := &Poller{Client: c}

	fetched := lighthue.Switch{ID: "btn-1", BridgeID: testBridgeID, LastEvent: "short_release", LastEventTime: polledOlder}
	p.upsert(context.Background(), logr.Discard(), fetched)

	var got lumenetesv1alpha1.Switch
	if err := c.Get(context.Background(), client.ObjectKey{Name: "btn-1"}, &got); err != nil {
		t.Fatalf("Get switch: %v", err)
	}
	if got.Status.LastEvent != "long_press" || !got.Status.LastEventTime.Time.Equal(streamedNewer.Time) {
		t.Errorf("got LastEvent=%q LastEventTime=%v, want the Streamer's newer event preserved (long_press, %v)", got.Status.LastEvent, got.Status.LastEventTime.Time, streamedNewer.Time)
	}
}

func TestGC_DeletesOnlyUnseenSwitchesForTheGivenBridge(t *testing.T) {
	unseen := seedSwitch("unseen", testBridgeID, true)
	seen := seedSwitch("seen", testBridgeID, true)
	otherBridge := seedSwitch("other-bridge-switch", "OTHERBRIDGE", true)
	c := newFakeClient(t, unseen, seen, otherBridge)
	p := &Poller{Client: c}

	p.gc(context.Background(), logr.Discard(), testBridgeID, map[string]bool{"seen": true})

	if err := c.Get(context.Background(), client.ObjectKey{Name: "unseen"}, &lumenetesv1alpha1.Switch{}); err == nil {
		t.Errorf("unseen switch still exists, want it deleted")
	}
	if err := c.Get(context.Background(), client.ObjectKey{Name: "seen"}, &lumenetesv1alpha1.Switch{}); err != nil {
		t.Errorf("seen switch was deleted, want it kept: %v", err)
	}
	if err := c.Get(context.Background(), client.ObjectKey{Name: "other-bridge-switch"}, &lumenetesv1alpha1.Switch{}); err != nil {
		t.Errorf("other-bridge switch was deleted, want it untouched (different bridge): %v", err)
	}
}

func TestMarkUnreachable_OnlyAffectsGivenBridgeAndSkipsAlreadyUnreachable(t *testing.T) {
	reachable := seedSwitch("reachable", testBridgeID, true)
	alreadyUnreachable := seedSwitch("already-unreachable", testBridgeID, false)
	otherBridge := seedSwitch("other-bridge-switch", "OTHERBRIDGE", true)
	c := newFakeClient(t, reachable, alreadyUnreachable, otherBridge)
	p := &Poller{Client: c}

	before := getSwitch(t, c, "already-unreachable")

	p.markUnreachable(context.Background(), logr.Discard(), testBridgeID)

	if got := getSwitch(t, c, "reachable"); got.Status.Reachable {
		t.Errorf("Status.Reachable = true, want flipped to false")
	}
	if got := getSwitch(t, c, "already-unreachable"); got.ResourceVersion != before.ResourceVersion {
		t.Errorf("ResourceVersion changed for an already-unreachable switch, want no redundant Status write")
	}
	if got := getSwitch(t, c, "other-bridge-switch"); !got.Status.Reachable {
		t.Errorf("Status.Reachable = false for a switch on a different bridge, want untouched")
	}
}
