// Package placesdata resolves locations via the Google Places API (New).
// Every call here is triggered by an explicit user action (typing into the
// itinerary location search, or submitting a new item with a location
// picked from it) - there's no background job involved.
package placesdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const defaultBaseURL = "https://places.googleapis.com"

// httpClient is reused across calls, same convention as
// applications/trips/internal/flightdata's package-level client.
// Cancellation/timeout is left to the caller's context rather than a
// client-wide timeout.
var httpClient = &http.Client{}

// Client resolves locations via the Google Places API (New) using an API
// key issued via Google Cloud.
type Client struct {
	apiKey string
	// baseURL is only ever overridden in tests, to point at an
	// httptest.Server stand-in for Places instead of the real API.
	baseURL string
}

// NewClient returns a Client authenticating with apiKey.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, baseURL: defaultBaseURL}
}

// NewClientForTest returns a Client pointed at baseURL instead of the real
// Places API - exported so other packages' tests (e.g. itineraryservice)
// can exercise the "places configured" path against an httptest.Server
// without this package needing to expose baseURL directly.
func NewClientForTest(baseURL string) *Client {
	return &Client{apiKey: "test-key", baseURL: baseURL}
}

// Candidate is one location suggestion returned by SearchLocations.
type Candidate struct {
	PlaceID     string
	Description string
}

// Location is a resolved location's address and coordinates, returned by
// GetLocation.
type Location struct {
	FormattedAddress string
	Latitude         float64
	Longitude        float64
}

type autocompleteRequest struct {
	Input string `json:"input"`
}

type autocompleteResponse struct {
	Suggestions []struct {
		PlacePrediction struct {
			PlaceID string `json:"placeId"`
			Text    struct {
				Text string `json:"text"`
			} `json:"text"`
		} `json:"placePrediction"`
	} `json:"suggestions"`
}

// SearchLocations returns candidate locations matching query, for a
// type-ahead search UI. Results carry only a place ID and display text -
// call GetLocation to resolve a chosen candidate's coordinates.
func (c *Client) SearchLocations(ctx context.Context, query string) ([]Candidate, error) {
	body, err := json.Marshal(autocompleteRequest{Input: query})
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/places:autocomplete", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searching locations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("places autocomplete returned status %d", resp.StatusCode)
	}

	var decoded autocompleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	candidates := make([]Candidate, 0, len(decoded.Suggestions))
	for _, s := range decoded.Suggestions {
		candidates = append(candidates, Candidate{
			PlaceID:     s.PlacePrediction.PlaceID,
			Description: s.PlacePrediction.Text.Text,
		})
	}

	return candidates, nil
}

type placeDetailsResponse struct {
	FormattedAddress string `json:"formattedAddress"`
	Location         struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
}

// GetLocation resolves placeID (as returned by SearchLocations) into its
// formatted address and coordinates.
func (c *Client) GetLocation(ctx context.Context, placeID string) (Location, error) {
	url := fmt.Sprintf("%s/v1/places/%s", c.baseURL, strings.TrimSpace(placeID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Location{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", "formattedAddress,location")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Location{}, fmt.Errorf("fetching location: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Location{}, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	var decoded placeDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return Location{}, fmt.Errorf("decoding response: %w", err)
	}

	return Location{
		FormattedAddress: decoded.FormattedAddress,
		Latitude:         decoded.Location.Latitude,
		Longitude:        decoded.Location.Longitude,
	}, nil
}
