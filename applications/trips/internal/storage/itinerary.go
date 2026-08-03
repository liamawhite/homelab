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

// ErrItineraryItemNotFound is returned by Delete/Schedule when no itinerary
// item matches the given ID.
var ErrItineraryItemNotFound = errors.New("itinerary item not found")

// ItineraryItem is a row from the itinerary_items table. Location/Latitude/
// Longitude/PlaceID are all empty/zero together until a location is set at
// creation - see Flights' identical convention for sync-derived fields.
type ItineraryItem struct {
	ID          string
	TripID      string
	Title       string
	Location    string
	Latitude    float64
	Longitude   float64
	PlaceID     string
	State       string
	Date        string
	StartMinute int64
	EndMinute   int64
	CreatedAt   time.Time
	// Timezone is an IANA zone name, set once at creation - see
	// itineraryservice's doc comment on AddItineraryItem for why this is
	// rendering-only and never converts Date/StartMinute/EndMinute.
	Timezone string
	// EndDate is empty when State == UNSCHEDULED, otherwise always set -
	// equal to Date for a single-day item, later for one spanning
	// multiple days (StartMinute is on Date, EndMinute is on EndDate).
	EndDate string
}

// ItineraryItems is the repository for the itinerary_items table, backed by
// sqlc-generated queries (see internal/storage/queries.sql and sqlcgen/).
type ItineraryItems struct {
	q *sqlcgen.Queries
}

// NewItineraryItems returns an ItineraryItems repository backed by db.
func NewItineraryItems(db *sql.DB) *ItineraryItems {
	return &ItineraryItems{q: sqlcgen.New(db)}
}

// ByTrip returns every itinerary item attached to tripID, ordered by
// creation time.
func (i *ItineraryItems) ByTrip(ctx context.Context, tripID string) ([]ItineraryItem, error) {
	rows, err := i.q.ListItineraryItemsByTrip(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("listing itinerary items: %w", err)
	}

	items := make([]ItineraryItem, 0, len(rows))
	for _, row := range rows {
		item, err := itineraryItemFromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// Create inserts a new itinerary item with a generated ID, always starting
// in the UNSCHEDULED state (date/minutes empty/zero) - the caller schedules
// it later via Schedule. location/latitude/longitude/placeID may all be
// empty/zero if no location was set.
func (i *ItineraryItems) Create(ctx context.Context, tripID, title, location string, latitude, longitude float64, placeID, timezone string) (ItineraryItem, error) {
	row, err := i.q.CreateItineraryItem(ctx, sqlcgen.CreateItineraryItemParams{
		ID:        uuid.NewString(),
		TripID:    tripID,
		Title:     title,
		Location:  location,
		Latitude:  latitude,
		Longitude: longitude,
		PlaceID:   placeID,
		State:     "UNSCHEDULED",
		Date:      "",
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Timezone:  timezone,
	})
	if err != nil {
		return ItineraryItem{}, fmt.Errorf("creating itinerary item: %w", err)
	}

	return itineraryItemFromRow(row)
}

// Schedule updates an itinerary item's placement (state/date/end_date/
// start_minute/end_minute) and timezone - the single mutation backing every
// drag/resize/unschedule/retimezone interaction. It returns
// ErrItineraryItemNotFound if no such item exists.
func (i *ItineraryItems) Schedule(ctx context.Context, id, state, date, endDate string, startMinute, endMinute int64, timezone string) (ItineraryItem, error) {
	row, err := i.q.ScheduleItineraryItem(ctx, sqlcgen.ScheduleItineraryItemParams{
		State:       state,
		Date:        date,
		StartMinute: startMinute,
		EndMinute:   endMinute,
		Timezone:    timezone,
		EndDate:     endDate,
		ID:          id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ItineraryItem{}, ErrItineraryItemNotFound
		}
		return ItineraryItem{}, fmt.Errorf("scheduling itinerary item: %w", err)
	}

	return itineraryItemFromRow(row)
}

// Delete removes the itinerary item with the given ID. It returns
// ErrItineraryItemNotFound if no such item exists.
func (i *ItineraryItems) Delete(ctx context.Context, id string) error {
	affected, err := i.q.DeleteItineraryItem(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting itinerary item: %w", err)
	}
	if affected == 0 {
		return ErrItineraryItemNotFound
	}

	return nil
}

func itineraryItemFromRow(row sqlcgen.ItineraryItem) (ItineraryItem, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return ItineraryItem{}, fmt.Errorf("parsing itinerary item created_at: %w", err)
	}

	return ItineraryItem{
		ID:          row.ID,
		TripID:      row.TripID,
		Title:       row.Title,
		Location:    row.Location,
		Latitude:    row.Latitude,
		Longitude:   row.Longitude,
		PlaceID:     row.PlaceID,
		State:       row.State,
		Date:        row.Date,
		StartMinute: row.StartMinute,
		EndMinute:   row.EndMinute,
		CreatedAt:   createdAt,
		Timezone:    row.Timezone,
		EndDate:     row.EndDate,
	}, nil
}
