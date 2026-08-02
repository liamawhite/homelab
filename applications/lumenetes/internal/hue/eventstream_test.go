package hue

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseSSE_ButtonEvent(t *testing.T) {
	frame := `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"1e3d9a73-7069-40ed-a889-051763348737","type":"light","on":{"on":false}},{"id":"abc-button-1","type":"button","button":{"button_report":{"updated":"2026-02-06T02:09:14Z","event":"short_release"}}}],"id":"dbcbe13e-5528-4f1e-a174-bd2da08c965d","type":"update"}]

`
	var got []ButtonEvent
	err := parseSSE(strings.NewReader(frame), func(ev ButtonEvent) { got = append(got, ev) }, nil)
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(got), got)
	}
	if got[0].ButtonID != "abc-button-1" || got[0].Event != "short_release" {
		t.Errorf("got %+v, want ButtonID=abc-button-1 Event=short_release", got[0])
	}
	wantTime, _ := time.Parse(time.RFC3339, "2026-02-06T02:09:14Z")
	if !got[0].Time.Equal(wantTime) {
		t.Errorf("got Time=%v, want %v", got[0].Time, wantTime)
	}
}

func TestParseSSE_CommentLinesIgnored(t *testing.T) {
	stream := ": hi\n\n" +
		`data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"btn-1","type":"button","button":{"button_report":{"updated":"2026-02-06T02:09:14Z","event":"long_press"}}}],"id":"e1","type":"update"}]` +
		"\n\n: hi\n\n"

	var got []ButtonEvent
	err := parseSSE(strings.NewReader(stream), func(ev ButtonEvent) { got = append(got, ev) }, nil)
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(got), got)
	}
	if got[0].ButtonID != "btn-1" || got[0].Event != "long_press" {
		t.Errorf("got %+v, want ButtonID=btn-1 Event=long_press", got[0])
	}
}

func TestParseSSE_MultiLineData(t *testing.T) {
	// SSE spec: consecutive data: lines before a blank line are joined
	// with "\n" into one payload.
	stream := "data: [{\"creationtime\":\"2026-02-06T02:09:13Z\",\"data\":[{\"id\":\"btn-2\",\"type\":\"button\",\n" +
		"data: \"button\":{\"button_report\":{\"updated\":\"2026-02-06T02:09:14Z\",\"event\":\"double_short_release\"}}}],\"id\":\"e2\",\"type\":\"update\"}]\n\n"

	var got []ButtonEvent
	err := parseSSE(strings.NewReader(stream), func(ev ButtonEvent) { got = append(got, ev) }, nil)
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(got) != 1 || got[0].Event != "double_short_release" {
		t.Fatalf("got %+v, want one double_short_release event", got)
	}
}

func TestParseSSE_MalformedFrameSkipped(t *testing.T) {
	stream := "data: {not valid json\n\n" +
		`data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"btn-3","type":"button","button":{"button_report":{"updated":"2026-02-06T02:09:14Z","event":"initial_press"}}}],"id":"e3","type":"update"}]` +
		"\n\n"

	var got []ButtonEvent
	err := parseSSE(strings.NewReader(stream), func(ev ButtonEvent) { got = append(got, ev) }, nil)
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(got) != 1 || got[0].ButtonID != "btn-3" {
		t.Fatalf("got %+v, want the one valid frame to still dispatch", got)
	}
}

func TestParseSSE_ReturnsErrorAtEOF(t *testing.T) {
	// A plain io.EOF from the reader after valid data is exactly what a
	// closed connection looks like - parseSSE must surface a non-nil
	// error so callers know to reconnect, while still having called the
	// handler for events seen before the end.
	stream := `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"btn-4","type":"button","button":{"button_report":{"updated":"2026-02-06T02:09:14Z","event":"long_release"}}}],"id":"e4","type":"update"}]` + "\n\n"

	var got []ButtonEvent
	err := parseSSE(strings.NewReader(stream), func(ev ButtonEvent) { got = append(got, ev) }, nil)
	if err == nil {
		t.Fatal("parseSSE() error = nil, want non-nil (EOF signals reconnect)")
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1 (events before EOF still dispatched)", len(got))
	}
}

func TestParseSSE_LightEvent_OnOnly(t *testing.T) {
	frame := `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"light-1","type":"light","on":{"on":true}}],"id":"e5","type":"update"}]` + "\n\n"

	var got []LightEvent
	err := parseSSE(strings.NewReader(frame), nil, func(ev LightEvent) { got = append(got, ev) })
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(got), got)
	}
	ev := got[0]
	if ev.LightID != "light-1" {
		t.Errorf("got LightID=%q, want light-1", ev.LightID)
	}
	if ev.On == nil || *ev.On != true {
		t.Errorf("got On=%v, want pointer to true", ev.On)
	}
	// Only "on" was reported - everything else must stay nil, not zero.
	if ev.Brightness != nil {
		t.Errorf("got Brightness=%v, want nil (not reported in this update)", ev.Brightness)
	}
	if ev.Color != nil {
		t.Errorf("got Color=%v, want nil (not reported in this update)", ev.Color)
	}
	if ev.ColorTempK != nil {
		t.Errorf("got ColorTempK=%v, want nil (not reported in this update)", ev.ColorTempK)
	}
}

func TestParseSSE_LightEvent_DimmingOnly(t *testing.T) {
	frame := `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"light-2","type":"light","dimming":{"brightness":42.5}}],"id":"e6","type":"update"}]` + "\n\n"

	var got []LightEvent
	err := parseSSE(strings.NewReader(frame), nil, func(ev LightEvent) { got = append(got, ev) })
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1: %+v", len(got), got)
	}
	ev := got[0]
	if ev.Brightness == nil || *ev.Brightness != 42.5 {
		t.Errorf("got Brightness=%v, want pointer to 42.5", ev.Brightness)
	}
	if ev.On != nil {
		t.Errorf("got On=%v, want nil (not reported in this update)", ev.On)
	}
}

func TestParseSSE_LightEvent_ColorTemperature(t *testing.T) {
	frame := `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"light-3","type":"light","color_temperature":{"mirek":250,"mirek_valid":true}}],"id":"e7","type":"update"}]` + "\n\n"

	var got []LightEvent
	err := parseSSE(strings.NewReader(frame), nil, func(ev LightEvent) { got = append(got, ev) })
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(got) != 1 || got[0].ColorTempK == nil {
		t.Fatalf("got %+v, want one event with ColorTempK set", got)
	}
	if *got[0].ColorTempK != 4000 {
		t.Errorf("got ColorTempK=%d, want 4000 (1_000_000/250)", *got[0].ColorTempK)
	}
}

func TestParseSSE_LightEvent_MirekInvalidIgnored(t *testing.T) {
	frame := `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"light-4","type":"light","color_temperature":{"mirek":250,"mirek_valid":false}}],"id":"e8","type":"update"}]` + "\n\n"

	var got []LightEvent
	err := parseSSE(strings.NewReader(frame), nil, func(ev LightEvent) { got = append(got, ev) })
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(got) != 1 || got[0].ColorTempK != nil {
		t.Fatalf("got %+v, want ColorTempK nil when mirek_valid is false", got)
	}
}

func TestParseSSE_ButtonAndLightBothDispatch(t *testing.T) {
	frame := `data: [{"creationtime":"2026-02-06T02:09:13Z","data":[{"id":"btn-5","type":"button","button":{"button_report":{"updated":"2026-02-06T02:09:14Z","event":"initial_press"}}},{"id":"light-5","type":"light","on":{"on":false}},{"id":"grouped-1","type":"grouped_light","on":{"on":true}}],"id":"e9","type":"update"}]` + "\n\n"

	var buttons []ButtonEvent
	var lights []LightEvent
	err := parseSSE(strings.NewReader(frame),
		func(ev ButtonEvent) { buttons = append(buttons, ev) },
		func(ev LightEvent) { lights = append(lights, ev) },
	)
	if err != io.EOF {
		t.Fatalf("parseSSE() error = %v, want io.EOF", err)
	}
	if len(buttons) != 1 || buttons[0].ButtonID != "btn-5" {
		t.Errorf("got buttons=%+v, want one btn-5 event", buttons)
	}
	if len(lights) != 1 || lights[0].LightID != "light-5" {
		t.Errorf("got lights=%+v, want one light-5 event (grouped_light must be ignored)", lights)
	}
}

func TestStreamEvents_ContextCancelReturnsNil(t *testing.T) {
	// Serve a slow trickle of SSE keepalives that never closes on its own,
	// so the only way the stream ends is via context cancellation.
	// hueClient already sets InsecureSkipVerify, so it works against this
	// self-signed httptest server with no client swapping needed.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("test server's ResponseWriter does not support flushing")
		}
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				fmt.Fprint(w, ": hi\n\n")
				flusher.Flush()
			}
		}
	})
	srv := httptest.NewTLSServer(handler)
	defer srv.Close()

	ip := strings.TrimPrefix(srv.URL, "https://")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := StreamEvents(ctx, ip, "test-app-key", func(ButtonEvent) {}, func(LightEvent) {})
	if err != nil {
		t.Errorf("StreamEvents() error = %v, want nil after context cancellation", err)
	}
}
