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

// ErrAccommodationNotFound is returned by Delete when no accommodation
// matches the given ID.
var ErrAccommodationNotFound = errors.New("accommodation not found")

// Accommodation is a row from the accommodations table. Unlike Flight,
// there's no external source to sync from - every field is exactly what
// the user entered.
type Accommodation struct {
	ID        string
	TripID    string
	Name      string
	Location  string
	CheckIn   string
	CheckOut  string
	CreatedAt time.Time
}

// Accommodations is the repository for the accommodations table, backed by
// sqlc-generated queries (see internal/storage/queries.sql and sqlcgen/).
type Accommodations struct {
	q *sqlcgen.Queries
}

// NewAccommodations returns an Accommodations repository backed by db.
func NewAccommodations(db *sql.DB) *Accommodations {
	return &Accommodations{q: sqlcgen.New(db)}
}

// ByTrip returns every accommodation attached to tripID, ordered by
// check-in date.
func (a *Accommodations) ByTrip(ctx context.Context, tripID string) ([]Accommodation, error) {
	rows, err := a.q.ListAccommodationsByTrip(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("listing accommodations: %w", err)
	}

	accommodations := make([]Accommodation, 0, len(rows))
	for _, row := range rows {
		// ListAccommodationsByTripRow has the same field layout as
		// sqlcgen.Accommodation (sqlc still generates a distinct per-query
		// row type even though the columns match 1:1) - see
		// storage/flights.go's identical comment for why this conversion
		// is valid Go and requires matching field order.
		accommodation, err := accommodationFromRow(sqlcgen.Accommodation(row))
		if err != nil {
			return nil, err
		}
		accommodations = append(accommodations, accommodation)
	}

	return accommodations, nil
}

// Create inserts a new accommodation with a generated ID and returns it.
func (a *Accommodations) Create(ctx context.Context, tripID, name, location, checkIn, checkOut string) (Accommodation, error) {
	row, err := a.q.CreateAccommodation(ctx, sqlcgen.CreateAccommodationParams{
		ID:        uuid.NewString(),
		TripID:    tripID,
		Name:      name,
		Location:  location,
		CheckIn:   checkIn,
		CheckOut:  checkOut,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Accommodation{}, fmt.Errorf("creating accommodation: %w", err)
	}

	return accommodationFromRow(sqlcgen.Accommodation(row))
}

// Delete removes the accommodation with the given ID. It returns
// ErrAccommodationNotFound if no such accommodation exists.
func (a *Accommodations) Delete(ctx context.Context, id string) error {
	affected, err := a.q.DeleteAccommodation(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting accommodation: %w", err)
	}
	if affected == 0 {
		return ErrAccommodationNotFound
	}

	return nil
}

func accommodationFromRow(row sqlcgen.Accommodation) (Accommodation, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return Accommodation{}, fmt.Errorf("parsing accommodation created_at: %w", err)
	}

	return Accommodation{
		ID:        row.ID,
		TripID:    row.TripID,
		Name:      row.Name,
		Location:  row.Location,
		CheckIn:   row.CheckIn,
		CheckOut:  row.CheckOut,
		CreatedAt: createdAt,
	}, nil
}
