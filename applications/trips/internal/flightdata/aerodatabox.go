// Package flightdata fetches live flight status from AeroDataBox (via
// RapidAPI). Every call here is triggered by an explicit user action (add a
// flight, or click "Sync now") - there is no background poller, since
// AeroDataBox's free tier is a fixed monthly unit quota that a periodic
// poller would burn through regardless of whether anything changed.
package flightdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://aerodatabox.p.rapidapi.com"

// httpClient is reused across calls, same convention as
// applications/lumenetes/internal/hue's package-level client. Cancellation/
// timeout is left to the caller's context rather than a client-wide timeout.
var httpClient = &http.Client{}

// Client fetches flight status from AeroDataBox using an API key issued via
// RapidAPI.
type Client struct {
	apiKey string
	// baseURL is only ever overridden in tests, to point at an
	// httptest.Server stand-in for AeroDataBox instead of the real API.
	baseURL string
}

// NewClient returns a Client authenticating with apiKey.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, baseURL: defaultBaseURL}
}

// Status mirrors trips.v1.FlightStatus without importing the generated
// proto package into this leaf package - callers map between the two.
type Status string

const (
	StatusUnknown   Status = "UNKNOWN"
	StatusScheduled Status = "SCHEDULED"
	StatusActive    Status = "ACTIVE"
	StatusLanded    Status = "LANDED"
	StatusCancelled Status = "CANCELLED"
	StatusDiverted  Status = "DIVERTED"
)

// FlightData is the subset of AeroDataBox's flight-status response this app
// cares about.
type FlightData struct {
	DepartureAirport string
	ArrivalAirport   string
	// DepartureTimezone/ArrivalTimezone are IANA zone names (e.g.
	// "America/Los_Angeles") - AeroDataBox reports every airport's local
	// zone, which is what lets the UI show each leg in its own local time
	// instead of just UTC or the viewer's browser zone.
	DepartureTimezone  string
	ArrivalTimezone    string
	ScheduledDeparture *time.Time
	ScheduledArrival   *time.Time
	ActualDeparture    *time.Time
	ActualArrival      *time.Time
	Status             Status
}

// airportMovement matches AeroDataBox's shared shape for both "departure"
// and "arrival" - each carries the airport reference plus a scheduledTime,
// and (depending on how far out the flight is / how much data AeroDataBox
// has) zero or more of: predictedTime (AeroDataBox's own estimate,
// available well before departure), revisedTime (the airline's official
// updated schedule, e.g. after a delay is announced), and runwayTime (the
// actual wheels-off/wheels-on event, once it's happened).
type airportMovement struct {
	Airport struct {
		IATA     string `json:"iata"`
		TimeZone string `json:"timeZone"`
	} `json:"airport"`
	ScheduledTime struct {
		UTC string `json:"utc"`
	} `json:"scheduledTime"`
	PredictedTime struct {
		UTC string `json:"utc"`
	} `json:"predictedTime"`
	RevisedTime struct {
		UTC string `json:"utc"`
	} `json:"revisedTime"`
	RunwayTime struct {
		UTC string `json:"utc"`
	} `json:"runwayTime"`
}

type flightResult struct {
	Number    string          `json:"number"`
	Status    string          `json:"status"`
	Departure airportMovement `json:"departure"`
	Arrival   airportMovement `json:"arrival"`
}

// FetchFlight looks up flightNumber on date (YYYY-MM-DD) and returns its
// current status. AeroDataBox can return more than one result for a single
// flight number/date (codeshares, rare schedule collisions) - the first
// result is used, matching the common case of a single physical flight per
// number per day.
func (c *Client) FetchFlight(ctx context.Context, flightNumber, date string) (FlightData, error) {
	url := fmt.Sprintf("%s/flights/number/%s/%s", c.baseURL, strings.TrimSpace(flightNumber), date)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return FlightData{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-RapidAPI-Key", c.apiKey)
	req.Header.Set("X-RapidAPI-Host", "aerodatabox.p.rapidapi.com")

	resp, err := httpClient.Do(req)
	if err != nil {
		return FlightData{}, fmt.Errorf("fetching flight: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FlightData{}, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	var results []flightResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return FlightData{}, fmt.Errorf("decoding response: %w", err)
	}
	if len(results) == 0 {
		return FlightData{}, fmt.Errorf("no flight found for %s on %s", flightNumber, date)
	}

	return toFlightData(results[0])
}

func toFlightData(r flightResult) (FlightData, error) {
	scheduledDeparture, err := parseTime(r.Departure.ScheduledTime.UTC)
	if err != nil {
		return FlightData{}, err
	}
	scheduledArrival, err := parseTime(r.Arrival.ScheduledTime.UTC)
	if err != nil {
		return FlightData{}, err
	}

	// "Actual" prefers the runway (wheels off/on) time - the most concrete
	// signal AeroDataBox offers - falling back to the airline's revised
	// schedule, then AeroDataBox's own predicted estimate (commonly the
	// only one of the three present until close to departure), and leaving
	// it unset if none are available yet.
	actualDeparture, err := firstNonEmpty(r.Departure.RunwayTime.UTC, r.Departure.RevisedTime.UTC, r.Departure.PredictedTime.UTC)
	if err != nil {
		return FlightData{}, err
	}
	actualArrival, err := firstNonEmpty(r.Arrival.RunwayTime.UTC, r.Arrival.RevisedTime.UTC, r.Arrival.PredictedTime.UTC)
	if err != nil {
		return FlightData{}, err
	}

	return FlightData{
		DepartureAirport:   r.Departure.Airport.IATA,
		ArrivalAirport:     r.Arrival.Airport.IATA,
		DepartureTimezone:  r.Departure.Airport.TimeZone,
		ArrivalTimezone:    r.Arrival.Airport.TimeZone,
		ScheduledDeparture: scheduledDeparture,
		ScheduledArrival:   scheduledArrival,
		ActualDeparture:    actualDeparture,
		ActualArrival:      actualArrival,
		Status:             toStatus(r.Status),
	}, nil
}

// aeroDataBoxTimeLayout is the "utc" sub-field's actual format - space
// (not "T") between date and time, no seconds, e.g. "2026-07-27 22:05Z".
// Not RFC3339 despite AeroDataBox's own docs implying an ISO 8601 shape -
// verified directly against a live response.
const aeroDataBoxTimeLayout = "2006-01-02 15:04Z07:00"

func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(aeroDataBoxTimeLayout, s)
	if err != nil {
		// Fall back to RFC3339 in case a different endpoint/field ever
		// does use the standard shape - keeps this parser from breaking
		// outright if AeroDataBox is inconsistent across fields.
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("parsing time %q: %w", s, err)
		}
	}
	return &t, nil
}

func firstNonEmpty(values ...string) (*time.Time, error) {
	for _, v := range values {
		if v != "" {
			return parseTime(v)
		}
	}
	return nil, nil
}

func toStatus(s string) Status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "expected", "checkin", "boarding", "gateclosed", "delayed", "scheduled":
		return StatusScheduled
	case "departed", "enroute", "approaching":
		return StatusActive
	case "arrived", "landed":
		return StatusLanded
	case "canceled", "cancelled":
		return StatusCancelled
	case "diverted":
		return StatusDiverted
	default:
		return StatusUnknown
	}
}
