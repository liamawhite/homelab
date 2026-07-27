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

// ErrTripNotFound is returned by Delete when no trip matches the given ID.
var ErrTripNotFound = errors.New("trip not found")

// Trip is a row from the trips table.
type Trip struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Trips is the repository for the trips table, backed by sqlc-generated
// queries (see internal/storage/queries.sql and sqlcgen/) - this type just
// adapts sqlcgen's raw string created_at column to a time.Time on the way
// out, and generates IDs/timestamps on the way in, so callers never deal
// with the storage-level representation.
type Trips struct {
	q *sqlcgen.Queries
}

// NewTrips returns a Trips repository backed by db.
func NewTrips(db *sql.DB) *Trips {
	return &Trips{q: sqlcgen.New(db)}
}

// List returns every trip, oldest first.
func (t *Trips) List(ctx context.Context) ([]Trip, error) {
	rows, err := t.q.ListTrips(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing trips: %w", err)
	}

	trips := make([]Trip, 0, len(rows))
	for _, row := range rows {
		trip, err := fromRow(row)
		if err != nil {
			return nil, err
		}
		trips = append(trips, trip)
	}

	return trips, nil
}

// Create inserts a new trip with a generated ID and returns it.
func (t *Trips) Create(ctx context.Context, name string) (Trip, error) {
	row, err := t.q.CreateTrip(ctx, sqlcgen.CreateTripParams{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Trip{}, fmt.Errorf("creating trip: %w", err)
	}

	return fromRow(row)
}

// Delete removes the trip with the given ID. It returns ErrTripNotFound if
// no such trip exists.
func (t *Trips) Delete(ctx context.Context, id string) error {
	affected, err := t.q.DeleteTrip(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting trip: %w", err)
	}
	if affected == 0 {
		return ErrTripNotFound
	}

	return nil
}

func fromRow(row sqlcgen.Trip) (Trip, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return Trip{}, fmt.Errorf("parsing trip created_at: %w", err)
	}

	return Trip{ID: row.ID, Name: row.Name, CreatedAt: createdAt}, nil
}
