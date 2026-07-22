package hue

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ButtonEvent is a single button-press event observed on the bridge's
// eventstream.
type ButtonEvent struct {
	ButtonID string // button resource RID - matches Switch.ID
	Event    string // e.g. "short_release", "long_press"
	Time     time.Time
}

// LightEvent is a (possibly partial) light state update observed on the
// bridge's eventstream - the bridge only reports fields that actually
// changed, so a nil field here means "not reported in this update," not
// "cleared" - callers must merge onto existing state field-by-field, never
// overwrite wholesale.
type LightEvent struct {
	LightID    string
	On         *bool
	Brightness *float64 // percentage 0-100
	Color      *string  // "#rrggbb", derived from xy if present
	ColorTempK *int     // Kelvin, derived from mirek if present and valid
	Time       time.Time
}

// sseEnvelope is one dispatched eventstream frame's "data:" payload - an
// array of these, per the CLIP v2 eventstream schema.
type sseEnvelope struct {
	CreationTime string            `json:"creationtime"`
	Type         string            `json:"type"`
	Data         []json.RawMessage `json:"data"`
}

// resourceTypePeek is decoded first for every item in an envelope's Data -
// the stream is global and unfiltered (light/grouped_light/button/etc. all
// interleaved), so this is how uninteresting items get discarded cheaply.
type resourceTypePeek struct {
	Type string `json:"type"`
}

// buttonReportResource is the button-specific shape decoded only for items
// whose resourceTypePeek.Type == "button".
type buttonReportResource struct {
	ID     string `json:"id"`
	Button struct {
		ButtonReport struct {
			Updated string `json:"updated"`
			Event   string `json:"event"`
		} `json:"button_report"`
	} `json:"button"`
}

// lightReportResource is the light-specific shape decoded only for items
// whose resourceTypePeek.Type == "light". Every field is a pointer so a
// partial update (e.g. only "on" changed) can be told apart from an
// explicit zero value - the same optionality UpdateLightState's PUT body
// uses, just for reading instead of writing.
type lightReportResource struct {
	ID string `json:"id"`
	On *struct {
		On bool `json:"on"`
	} `json:"on"`
	Dimming *struct {
		Brightness float64 `json:"brightness"`
	} `json:"dimming"`
	Color *struct {
		XY struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"xy"`
	} `json:"color"`
	ColorTemperature *struct {
		Mirek      int  `json:"mirek"`
		MirekValid bool `json:"mirek_valid"`
	} `json:"color_temperature"`
}

// StreamEvents opens the bridge at ip's CLIP v2 SSE eventstream and calls
// onButton/onLight once per matching event, blocking until ctx is canceled
// or the stream ends. Either callback may be nil if the caller doesn't
// care about that event type. Returns nil only when ctx.Done() caused the
// end (a deliberate stop); any other termination (EOF, read error, non-200)
// returns a non-nil error so the caller (internal/eventstream.Streamer)
// knows to reconnect.
func StreamEvents(ctx context.Context, ip, appKey string, onButton func(ButtonEvent), onLight func(LightEvent)) error {
	url := fmt.Sprintf("https://%s/eventstream/clip/v2", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to build request for %s: %w", url, err)
	}
	req.Header.Set("hue-application-key", appKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := hueClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("failed to reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	if err := parseSSE(resp.Body, onButton, onLight); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("eventstream from %s ended: %w", url, err)
	}
	return nil
}

// parseSSE reads r as an SSE stream, dispatching one callback per matching
// event, until r ends. Split out from StreamEvents so tests can feed a
// canned io.Reader without a real bridge/HTTP round trip.
func parseSSE(r io.Reader, onButton func(ButtonEvent), onLight func(LightEvent)) error {
	scanner := bufio.NewScanner(r)
	// A single dispatched envelope can bundle every resource in a batch -
	// the default 64KB token cap risks truncating a large frame.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var dataLines []string
	flush := func() {
		if len(dataLines) == 0 {
			return
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		dispatchSSEPayload(payload, onButton, onLight)
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flush() // blank line terminates the SSE frame
		case strings.HasPrefix(line, ":"):
			// comment/keepalive (e.g. ": hi") - ignore
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		default:
			// id:/event:/retry: fields - not needed for v1, ignore
		}
	}
	flush() // stream may end mid-frame with no trailing blank line
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

// dispatchSSEPayload decodes one SSE frame's joined data: payload and
// calls onButton/onLight for every matching event found in it. A malformed
// payload is dropped, not treated as a fatal stream error - one bad frame
// shouldn't kill the whole connection.
func dispatchSSEPayload(payload string, onButton func(ButtonEvent), onLight func(LightEvent)) {
	var envelopes []sseEnvelope
	if err := json.Unmarshal([]byte(payload), &envelopes); err != nil {
		return
	}
	for _, env := range envelopes {
		creationTime, _ := time.Parse(time.RFC3339, env.CreationTime)
		for _, raw := range env.Data {
			var peek resourceTypePeek
			if err := json.Unmarshal(raw, &peek); err != nil {
				continue
			}
			switch peek.Type {
			case "button":
				dispatchButtonEvent(raw, creationTime, onButton)
			case "light":
				dispatchLightEvent(raw, creationTime, onLight)
			}
		}
	}
}

func dispatchButtonEvent(raw json.RawMessage, creationTime time.Time, onButton func(ButtonEvent)) {
	if onButton == nil {
		return
	}
	var res buttonReportResource
	if err := json.Unmarshal(raw, &res); err != nil || res.Button.ButtonReport.Event == "" {
		return
	}
	t := creationTime
	if updated, err := time.Parse(time.RFC3339, res.Button.ButtonReport.Updated); err == nil {
		t = updated
	}
	onButton(ButtonEvent{ButtonID: res.ID, Event: res.Button.ButtonReport.Event, Time: t})
}

func dispatchLightEvent(raw json.RawMessage, creationTime time.Time, onLight func(LightEvent)) {
	if onLight == nil {
		return
	}
	var res lightReportResource
	if err := json.Unmarshal(raw, &res); err != nil || res.ID == "" {
		return
	}

	ev := LightEvent{LightID: res.ID, Time: creationTime}
	if res.On != nil {
		on := res.On.On
		ev.On = &on
	}
	if res.Dimming != nil {
		brightness := res.Dimming.Brightness
		ev.Brightness = &brightness
	}
	if res.Color != nil {
		if hex := xyToHex(res.Color.XY.X, res.Color.XY.Y); hex != "" {
			ev.Color = &hex
		}
	}
	if res.ColorTemperature != nil && res.ColorTemperature.MirekValid {
		k := mirekToKelvin(res.ColorTemperature.Mirek)
		ev.ColorTempK = &k
	}
	onLight(ev)
}
