package flightdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Shape and time format ("2026-08-01 15:04Z" - space-separated, no
// seconds) verified directly against a live AeroDataBox response, not
// assumed from docs.
const sampleResponse = `[
  {
    "number": "UA 523",
    "status": "EnRoute",
    "departure": {
      "airport": {"iata": "SFO", "timeZone": "America/Los_Angeles"},
      "scheduledTime": {"utc": "2026-08-01 15:04Z"},
      "revisedTime": {"utc": "2026-08-01 15:10Z"},
      "runwayTime": {"utc": "2026-08-01 15:12Z"}
    },
    "arrival": {
      "airport": {"iata": "JFK", "timeZone": "America/New_York"},
      "scheduledTime": {"utc": "2026-08-01 23:30Z"},
      "predictedTime": {"utc": "2026-08-01 23:35Z"}
    }
  }
]`

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{apiKey: "test-key", baseURL: server.URL}
}

func TestFetchFlight_ParsesResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-RapidAPI-Key"); got != "test-key" {
			t.Errorf("X-RapidAPI-Key header = %q, want %q", got, "test-key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleResponse))
	})

	data, err := client.FetchFlight(context.Background(), "UA523", "2026-08-01")
	if err != nil {
		t.Fatalf("FetchFlight() error = %v", err)
	}

	if data.DepartureAirport != "SFO" {
		t.Errorf("DepartureAirport = %q, want %q", data.DepartureAirport, "SFO")
	}
	if data.ArrivalAirport != "JFK" {
		t.Errorf("ArrivalAirport = %q, want %q", data.ArrivalAirport, "JFK")
	}
	if data.DepartureTimezone != "America/Los_Angeles" {
		t.Errorf("DepartureTimezone = %q, want %q", data.DepartureTimezone, "America/Los_Angeles")
	}
	if data.ArrivalTimezone != "America/New_York" {
		t.Errorf("ArrivalTimezone = %q, want %q", data.ArrivalTimezone, "America/New_York")
	}
	if data.ScheduledDeparture == nil {
		t.Fatal("ScheduledDeparture is nil")
	}
	if data.ActualDeparture == nil {
		t.Fatal("ActualDeparture is nil, want the runwayTime value")
	}
	// Arrival has only predictedTime (no runway/revised), so actual should
	// fall back to that.
	if data.ActualArrival == nil {
		t.Fatal("ActualArrival is nil, want the predictedTime value")
	}
	if data.Status != StatusActive {
		t.Errorf("Status = %q, want %q", data.Status, StatusActive)
	}
}

func TestFetchFlight_NoResults(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	if _, err := client.FetchFlight(context.Background(), "UA523", "2026-08-01"); err == nil {
		t.Fatal("FetchFlight() error = nil, want an error for an empty result set")
	}
}

func TestFetchFlight_NonOKStatus(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	if _, err := client.FetchFlight(context.Background(), "UA523", "2026-08-01"); err == nil {
		t.Fatal("FetchFlight() error = nil, want an error for a non-200 response")
	}
}
