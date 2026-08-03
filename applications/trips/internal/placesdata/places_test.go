package placesdata

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{apiKey: "test-key", baseURL: server.URL}
}

const sampleAutocompleteResponse = `{
  "suggestions": [
    {
      "placePrediction": {
        "placeId": "ChIJLU7jZClu5kcR4PcOOO6p3I0",
        "text": {"text": "Eiffel Tower, Paris, France"}
      }
    }
  ]
}`

func TestSearchLocations_ParsesResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("X-Goog-Api-Key"); got != "test-key" {
			t.Errorf("X-Goog-Api-Key header = %q, want %q", got, "test-key")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		var decoded autocompleteRequest
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if decoded.Input != "Eiffel Tower" {
			t.Errorf("input = %q, want %q", decoded.Input, "Eiffel Tower")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleAutocompleteResponse))
	})

	candidates, err := client.SearchLocations(context.Background(), "Eiffel Tower")
	if err != nil {
		t.Fatalf("SearchLocations() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].PlaceID != "ChIJLU7jZClu5kcR4PcOOO6p3I0" {
		t.Errorf("PlaceID = %q, want %q", candidates[0].PlaceID, "ChIJLU7jZClu5kcR4PcOOO6p3I0")
	}
	if candidates[0].Description != "Eiffel Tower, Paris, France" {
		t.Errorf("Description = %q, want %q", candidates[0].Description, "Eiffel Tower, Paris, France")
	}
}

func TestSearchLocations_NoSuggestions(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})

	candidates, err := client.SearchLocations(context.Background(), "asdkfjhaslkdjfh")
	if err != nil {
		t.Fatalf("SearchLocations() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("len(candidates) = %d, want 0", len(candidates))
	}
}

func TestSearchLocations_NonOKStatus(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	if _, err := client.SearchLocations(context.Background(), "Eiffel Tower"); err == nil {
		t.Fatal("SearchLocations() error = nil, want an error for a non-200 response")
	}
}

const samplePlaceDetailsResponse = `{
  "formattedAddress": "Champ de Mars, 5 Av. Anatole France, 75007 Paris, France",
  "location": {"latitude": 48.8583701, "longitude": 2.2944813}
}`

func TestGetLocation_ParsesResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if got := r.Header.Get("X-Goog-Api-Key"); got != "test-key" {
			t.Errorf("X-Goog-Api-Key header = %q, want %q", got, "test-key")
		}
		if got := r.Header.Get("X-Goog-FieldMask"); got != "formattedAddress,location" {
			t.Errorf("X-Goog-FieldMask header = %q, want %q", got, "formattedAddress,location")
		}
		if !strings.HasSuffix(r.URL.Path, "/ChIJLU7jZClu5kcR4PcOOO6p3I0") {
			t.Errorf("path = %q, want suffix %q", r.URL.Path, "/ChIJLU7jZClu5kcR4PcOOO6p3I0")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(samplePlaceDetailsResponse))
	})

	loc, err := client.GetLocation(context.Background(), "ChIJLU7jZClu5kcR4PcOOO6p3I0")
	if err != nil {
		t.Fatalf("GetLocation() error = %v", err)
	}
	if loc.FormattedAddress != "Champ de Mars, 5 Av. Anatole France, 75007 Paris, France" {
		t.Errorf("FormattedAddress = %q, want %q", loc.FormattedAddress, "Champ de Mars, 5 Av. Anatole France, 75007 Paris, France")
	}
	if loc.Latitude != 48.8583701 {
		t.Errorf("Latitude = %v, want %v", loc.Latitude, 48.8583701)
	}
	if loc.Longitude != 2.2944813 {
		t.Errorf("Longitude = %v, want %v", loc.Longitude, 2.2944813)
	}
}

func TestGetLocation_NonOKStatus(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if _, err := client.GetLocation(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("GetLocation() error = nil, want an error for a non-200 response")
	}
}
