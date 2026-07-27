// Package flightsync fetches one flight's live status and persists it.
// It's the single place that bridges internal/flightdata (the external API
// client) and internal/storage (the flights table) - both internal/
// flightservice's on-demand RPCs (AddFlight's initial sync, SyncFlight's
// manual "sync now") call this instead of duplicating fetch-then-persist
// logic.
package flightsync

import (
	"context"
	"time"

	"github.com/liamawhite/trips/internal/flightdata"
	"github.com/liamawhite/trips/internal/storage"
)

// writeTimeout bounds the storage write-back independently of ctx - ctx may
// carry a caller-supplied deadline meant for the external fetch (e.g.
// AddFlight's best-effort initial sync), and a slow/near-expired fetch
// shouldn't also risk failing the local SQLite write that records the
// result (or, on failure, even just the error message).
const writeTimeout = 5 * time.Second

// SyncOne fetches flight's current status from client and persists it via
// flights. On a fetch failure, the flight's previously-stored data is left
// untouched (see storage.Flights.UpdateSyncError's doc comment) - only the
// last-synced timestamp and error message are recorded. The returned Flight
// reflects that recorded state (including sync_error) even on failure, so
// callers can still show the caller-visible flight/error without a second
// fetch; the fetch error itself is also returned for callers that need to
// react to it (e.g. choose an RPC error code).
func SyncOne(ctx context.Context, client *flightdata.Client, flights *storage.Flights, flight storage.Flight) (storage.Flight, error) {
	data, err := client.FetchFlight(ctx, flight.FlightNumber, flight.FlightDate)

	writeCtx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	if err != nil {
		updated, updateErr := flights.UpdateSyncError(writeCtx, flight.ID, err.Error())
		if updateErr != nil {
			return storage.Flight{}, updateErr
		}
		return updated, err
	}

	return flights.UpdateSyncResult(writeCtx, flight.ID, storage.FlightSyncResult{
		DepartureAirport:   data.DepartureAirport,
		ArrivalAirport:     data.ArrivalAirport,
		DepartureTimezone:  data.DepartureTimezone,
		ArrivalTimezone:    data.ArrivalTimezone,
		ScheduledDeparture: data.ScheduledDeparture,
		ScheduledArrival:   data.ScheduledArrival,
		ActualDeparture:    data.ActualDeparture,
		ActualArrival:      data.ActualArrival,
		Status:             string(data.Status),
	})
}
