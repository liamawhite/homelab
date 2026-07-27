package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/liamawhite/trips/internal/storage/sqlcgen"
)

// ErrFlightNotFound is returned by Delete when no flight matches the given ID.
var ErrFlightNotFound = errors.New("flight not found")

// Flight is a row from the flights table. Fields populated by a sync (the
// airports, times, and status) are pointers/empty until the first
// successful sync - see FlightSyncResult.
type Flight struct {
	ID                 string
	TripID             string
	FlightNumber       string
	FlightDate         string
	DepartureAirport   string
	ArrivalAirport     string
	DepartureTimezone  string
	ArrivalTimezone    string
	ScheduledDeparture *time.Time
	ScheduledArrival   *time.Time
	ActualDeparture    *time.Time
	ActualArrival      *time.Time
	Status             string
	LastSyncedAt       *time.Time
	SyncError          string
	CreatedAt          time.Time
}

// FlightSyncResult carries the fields a successful external fetch updates.
type FlightSyncResult struct {
	DepartureAirport   string
	ArrivalAirport     string
	DepartureTimezone  string
	ArrivalTimezone    string
	ScheduledDeparture *time.Time
	ScheduledArrival   *time.Time
	ActualDeparture    *time.Time
	ActualArrival      *time.Time
	Status             string
}

// Flights is the repository for the flights table, backed by sqlc-generated
// queries (see internal/storage/queries.sql and sqlcgen/).
type Flights struct {
	q *sqlcgen.Queries
}

// NewFlights returns a Flights repository backed by db.
func NewFlights(db *sql.DB) *Flights {
	return &Flights{q: sqlcgen.New(db)}
}

// ByTrip returns every flight attached to tripID, ordered by flight date.
func (f *Flights) ByTrip(ctx context.Context, tripID string) ([]Flight, error) {
	rows, err := f.q.ListFlightsByTrip(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("listing flights: %w", err)
	}

	flights := make([]Flight, 0, len(rows))
	for _, row := range rows {
		// ListFlightsByTripRow has the exact same field layout as
		// sqlcgen.Flight (sqlc generates a per-query row type even when the
		// selected columns match the table 1:1) - a direct conversion is
		// valid Go and avoids a second field-by-field mapping function.
		flight, err := flightFromRow(sqlcgen.Flight(row))
		if err != nil {
			return nil, err
		}
		flights = append(flights, flight)
	}

	return flights, nil
}

// Get returns the flight with the given ID. It returns ErrFlightNotFound if
// no such flight exists.
func (f *Flights) Get(ctx context.Context, id string) (Flight, error) {
	row, err := f.q.GetFlight(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Flight{}, ErrFlightNotFound
		}
		return Flight{}, fmt.Errorf("getting flight: %w", err)
	}

	return flightFromRow(sqlcgen.Flight(row))
}

// Create inserts a new flight with a generated ID. Every sync-derived field
// starts empty/unset - the caller is expected to follow up with a
// UpdateSyncResult/UpdateSyncError call once it has (attempted to) fetch
// live data.
func (f *Flights) Create(ctx context.Context, tripID, flightNumber, date string) (Flight, error) {
	row, err := f.q.CreateFlight(ctx, sqlcgen.CreateFlightParams{
		ID:           uuid.NewString(),
		TripID:       tripID,
		FlightNumber: flightNumber,
		FlightDate:   date,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Flight{}, fmt.Errorf("creating flight: %w", err)
	}

	return flightFromRow(sqlcgen.Flight(row))
}

// Delete removes the flight with the given ID. It returns ErrFlightNotFound
// if no such flight exists.
func (f *Flights) Delete(ctx context.Context, id string) error {
	affected, err := f.q.DeleteFlight(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting flight: %w", err)
	}
	if affected == 0 {
		return ErrFlightNotFound
	}

	return nil
}

// UpdateSyncResult records a successful sync's fetched fields, overwriting
// whatever was previously stored.
func (f *Flights) UpdateSyncResult(ctx context.Context, id string, result FlightSyncResult) (Flight, error) {
	now := time.Now().UTC()
	row, err := f.q.UpdateFlightSyncResult(ctx, sqlcgen.UpdateFlightSyncResultParams{
		DepartureAirport:   result.DepartureAirport,
		ArrivalAirport:     result.ArrivalAirport,
		DepartureTimezone:  result.DepartureTimezone,
		ArrivalTimezone:    result.ArrivalTimezone,
		ScheduledDeparture: timeToNullString(result.ScheduledDeparture),
		ScheduledArrival:   timeToNullString(result.ScheduledArrival),
		ActualDeparture:    timeToNullString(result.ActualDeparture),
		ActualArrival:      timeToNullString(result.ActualArrival),
		Status:             result.Status,
		LastSyncedAt:       timeToNullString(&now),
		SyncError:          "",
		ID:                 id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Flight{}, ErrFlightNotFound
		}
		return Flight{}, fmt.Errorf("updating flight sync result: %w", err)
	}

	return flightFromRow(sqlcgen.Flight(row))
}

// UpdateSyncError records a failed sync attempt - it deliberately leaves
// every previously-fetched field untouched (a transient API failure should
// never destroy known-good data), only recording the failure and the
// attempt time so the UI can show "last synced" / surface the error.
func (f *Flights) UpdateSyncError(ctx context.Context, id, syncErr string) (Flight, error) {
	now := time.Now().UTC()
	row, err := f.q.UpdateFlightSyncError(ctx, sqlcgen.UpdateFlightSyncErrorParams{
		LastSyncedAt: timeToNullString(&now),
		SyncError:    syncErr,
		ID:           id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Flight{}, ErrFlightNotFound
		}
		return Flight{}, fmt.Errorf("updating flight sync error: %w", err)
	}

	return flightFromRow(sqlcgen.Flight(row))
}

func timeToNullString(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339Nano), Valid: true}
}

func nullStringToTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil, fmt.Errorf("parsing time %q: %w", s.String, err)
	}
	return &t, nil
}

func flightFromRow(row sqlcgen.Flight) (Flight, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return Flight{}, fmt.Errorf("parsing flight created_at: %w", err)
	}

	scheduledDeparture, err := nullStringToTime(row.ScheduledDeparture)
	if err != nil {
		return Flight{}, err
	}
	scheduledArrival, err := nullStringToTime(row.ScheduledArrival)
	if err != nil {
		return Flight{}, err
	}
	actualDeparture, err := nullStringToTime(row.ActualDeparture)
	if err != nil {
		return Flight{}, err
	}
	actualArrival, err := nullStringToTime(row.ActualArrival)
	if err != nil {
		return Flight{}, err
	}
	lastSyncedAt, err := nullStringToTime(row.LastSyncedAt)
	if err != nil {
		return Flight{}, err
	}

	return Flight{
		ID:                 row.ID,
		TripID:             row.TripID,
		FlightNumber:       row.FlightNumber,
		FlightDate:         row.FlightDate,
		DepartureAirport:   row.DepartureAirport,
		ArrivalAirport:     row.ArrivalAirport,
		DepartureTimezone:  row.DepartureTimezone,
		ArrivalTimezone:    row.ArrivalTimezone,
		ScheduledDeparture: scheduledDeparture,
		ScheduledArrival:   scheduledArrival,
		ActualDeparture:    actualDeparture,
		ActualArrival:      actualArrival,
		Status:             row.Status,
		LastSyncedAt:       lastSyncedAt,
		SyncError:          row.SyncError,
		CreatedAt:          createdAt,
	}, nil
}
