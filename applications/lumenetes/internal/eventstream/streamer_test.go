package eventstream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	lighthue "github.com/liamawhite/lumenetes/internal/hue"
	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/bridges"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testLogger() logr.Logger {
	return logr.Discard()
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := lumenetesv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	return scheme
}

func TestResolveIP(t *testing.T) {
	cases := []struct {
		name    string
		bridge  *lumenetesv1alpha1.HueBridge
		wantIP  string
		wantOK  bool
	}{
		{
			name: "reachable bridge",
			bridge: &lumenetesv1alpha1.HueBridge{
				ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("ABC123")},
				Status:     lumenetesv1alpha1.HueBridgeStatus{IP: "10.0.0.5", Reachable: true},
			},
			wantIP: "10.0.0.5",
			wantOK: true,
		},
		{
			name:   "bridge missing",
			bridge: nil,
			wantOK: false,
		},
		{
			name: "bridge unreachable",
			bridge: &lumenetesv1alpha1.HueBridge{
				ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("ABC123")},
				Status:     lumenetesv1alpha1.HueBridgeStatus{IP: "10.0.0.5", Reachable: false},
			},
			wantOK: false,
		},
		{
			name: "bridge reachable but no IP",
			bridge: &lumenetesv1alpha1.HueBridge{
				ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName("ABC123")},
				Status:     lumenetesv1alpha1.HueBridgeStatus{IP: "", Reachable: true},
			},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(newScheme(t))
			if tc.bridge != nil {
				builder = builder.WithObjects(tc.bridge)
			}
			s := &Streamer{Client: builder.Build()}

			ip, ok := s.resolveIP(t.Context(), "ABC123")
			if ok != tc.wantOK || ip != tc.wantIP {
				t.Errorf("resolveIP() = (%q, %v), want (%q, %v)", ip, ok, tc.wantIP, tc.wantOK)
			}
		})
	}
}

func TestNextBackoff(t *testing.T) {
	cases := []struct {
		cur  time.Duration
		want time.Duration
	}{
		{cur: 1 * time.Second, want: 2 * time.Second},
		{cur: 2 * time.Second, want: 4 * time.Second},
		{cur: 15 * time.Second, want: 30 * time.Second},
		{cur: 20 * time.Second, want: maxReconnectBackoff},
		{cur: maxReconnectBackoff, want: maxReconnectBackoff},
	}

	for _, tc := range cases {
		t.Run(tc.cur.String(), func(t *testing.T) {
			if got := nextBackoff(tc.cur); got != tc.want {
				t.Errorf("nextBackoff(%v) = %v, want %v", tc.cur, got, tc.want)
			}
		})
	}
}

func TestPublishButton(t *testing.T) {
	t.Run("delivers to a draining receiver", func(t *testing.T) {
		ch := make(chan lighthue.ButtonEvent, 1)
		s := &Streamer{ButtonEvents: ch}
		ev := lighthue.ButtonEvent{ButtonID: "btn-1", Event: "short_release"}

		s.publishButton(t.Context(), ev)

		select {
		case got := <-ch:
			if got.ButtonID != ev.ButtonID {
				t.Errorf("got %+v, want %+v", got, ev)
			}
		default:
			t.Fatal("expected event on channel, got none")
		}
	})

	t.Run("returns promptly on ctx cancellation with nothing draining", func(t *testing.T) {
		ch := make(chan lighthue.ButtonEvent) // zero-buffer, no reader
		s := &Streamer{ButtonEvents: ch}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		done := make(chan struct{})
		go func() {
			s.publishButton(ctx, lighthue.ButtonEvent{})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("publishButton blocked past ctx cancellation")
		}
	})
}

func TestPublishLight(t *testing.T) {
	t.Run("delivers to a draining receiver", func(t *testing.T) {
		ch := make(chan lighthue.LightEvent, 1)
		s := &Streamer{LightEvents: ch}
		ev := lighthue.LightEvent{LightID: "light-1"}

		s.publishLight(t.Context(), ev)

		select {
		case got := <-ch:
			if got.LightID != ev.LightID {
				t.Errorf("got %+v, want %+v", got, ev)
			}
		default:
			t.Fatal("expected event on channel, got none")
		}
	})

	t.Run("returns promptly on ctx cancellation with nothing draining", func(t *testing.T) {
		ch := make(chan lighthue.LightEvent) // zero-buffer, no reader
		s := &Streamer{LightEvents: ch}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		done := make(chan struct{})
		go func() {
			s.publishLight(ctx, lighthue.LightEvent{})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("publishLight blocked past ctx cancellation")
		}
	})
}

func TestRunBridge_ResolveIPFails(t *testing.T) {
	// No HueBridge seeded, so resolveIP always fails - runBridge must back
	// off and retry without ever attempting a StreamEvents connection, and
	// must return promptly once ctx is canceled during that backoff sleep.
	scheme := newScheme(t)
	s := &Streamer{
		Client:       fake.NewClientBuilder().WithScheme(scheme).Build(),
		ButtonEvents: make(chan lighthue.ButtonEvent, 1),
		LightEvents:  make(chan lighthue.LightEvent, 1),
	}

	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.runBridge(ctx, testLogger(), bridges.Config{ID: "ABC123", AppKey: "key"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("runBridge did not return after ctx cancellation during backoff")
	}
}

func TestRunBridge_StreamsEvents(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("test server's ResponseWriter does not support flushing")
		}
		fmt.Fprint(w, `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"btn-1","type":"button","button":{"button_report":{"updated":"2026-02-06T02:09:14Z","event":"short_release"}}},{"id":"light-1","type":"light","on":{"on":true}}],"id":"e1","type":"update"}]`+"\n\n")
		flusher.Flush()
		<-r.Context().Done()
	})
	srv := httptest.NewTLSServer(handler)
	defer srv.Close()

	scheme := newScheme(t)
	bridgeID := "ABC123"
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName(bridgeID)},
		Status: lumenetesv1alpha1.HueBridgeStatus{
			IP:        strings.TrimPrefix(srv.URL, "https://"),
			Reachable: true,
		},
	}

	buttons := make(chan lighthue.ButtonEvent, 1)
	lights := make(chan lighthue.LightEvent, 1)
	s := &Streamer{
		Client:       fake.NewClientBuilder().WithScheme(scheme).WithObjects(hueBridge).Build(),
		ButtonEvents: buttons,
		LightEvents:  lights,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.runBridge(ctx, testLogger(), bridges.Config{ID: bridgeID, AppKey: "key"})
		close(done)
	}()

	select {
	case ev := <-buttons:
		if ev.ButtonID != "btn-1" {
			t.Errorf("got button event %+v, want ButtonID=btn-1", ev)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for button event")
	}

	select {
	case ev := <-lights:
		if ev.LightID != "light-1" {
			t.Errorf("got light event %+v, want LightID=light-1", ev)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for light event")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("runBridge did not return after ctx cancellation")
	}
}

func TestRunBridge_ReconnectsAfterStreamError(t *testing.T) {
	var hits int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewTLSServer(handler)
	defer srv.Close()

	scheme := newScheme(t)
	bridgeID := "ABC123"
	hueBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName(bridgeID)},
		Status: lumenetesv1alpha1.HueBridgeStatus{
			IP:        strings.TrimPrefix(srv.URL, "https://"),
			Reachable: true,
		},
	}

	s := &Streamer{
		Client:       fake.NewClientBuilder().WithScheme(scheme).WithObjects(hueBridge).Build(),
		ButtonEvents: make(chan lighthue.ButtonEvent, 1),
		LightEvents:  make(chan lighthue.LightEvent, 1),
	}

	// minReconnectBackoff is 1s: attempt 1 happens immediately, then a 1s
	// sleep before attempt 2. Give it comfortable headroom past that.
	ctx, cancel := context.WithTimeout(t.Context(), 2200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.runBridge(ctx, testLogger(), bridges.Config{ID: bridgeID, AppKey: "key"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("runBridge did not return after ctx cancellation")
	}

	if atomic.LoadInt64(&hits) < 2 {
		t.Errorf("got %d connection attempts, want at least 2 (a reconnect after the first error)", hits)
	}
}

func TestStart_ZeroBridgesBlocksUntilCtxDone(t *testing.T) {
	s := &Streamer{
		Client:       fake.NewClientBuilder().WithScheme(newScheme(t)).Build(),
		ButtonEvents: make(chan lighthue.ButtonEvent, 1),
		LightEvents:  make(chan lighthue.LightEvent, 1),
	}

	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := s.Start(ctx); err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
		t.Errorf("Start() returned early after %v, want it to block until ctx.Done()", elapsed)
	}
}

func TestStart_OneBridgeFailureDoesNotBlockAnother(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("test server's ResponseWriter does not support flushing")
		}
		fmt.Fprint(w, `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"light-ok","type":"light","on":{"on":true}}],"id":"e1","type":"update"}]`+"\n\n")
		flusher.Flush()
		<-r.Context().Done()
	})
	srv := httptest.NewTLSServer(handler)
	defer srv.Close()

	scheme := newScheme(t)
	workingID, brokenID := "WORKING", "BROKEN"
	workingBridge := &lumenetesv1alpha1.HueBridge{
		ObjectMeta: metav1.ObjectMeta{Name: bridges.ResourceName(workingID)},
		Status: lumenetesv1alpha1.HueBridgeStatus{
			IP:        strings.TrimPrefix(srv.URL, "https://"),
			Reachable: true,
		},
	}
	// brokenID intentionally has no HueBridge CR at all, so resolveIP for
	// it always fails and its goroutine just backs off forever.

	lights := make(chan lighthue.LightEvent, 1)
	s := &Streamer{
		Client:       fake.NewClientBuilder().WithScheme(scheme).WithObjects(workingBridge).Build(),
		ButtonEvents: make(chan lighthue.ButtonEvent, 1),
		LightEvents:  lights,
		Bridges: []bridges.Config{
			{ID: workingID, AppKey: "key"},
			{ID: brokenID, AppKey: "key"},
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = s.Start(ctx)
		close(done)
	}()

	select {
	case ev := <-lights:
		if ev.LightID != "light-ok" {
			t.Errorf("got %+v, want LightID=light-ok", ev)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("working bridge's event never arrived - broken bridge blocked it")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Start did not return after ctx cancellation")
	}
}
