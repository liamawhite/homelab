package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/liamawhite/reminders/internal/storage/sqlcgen"
)

// ErrOneOffItemNotFound is returned by Delete when no item matches the
// given ID.
var ErrOneOffItemNotFound = errors.New("one-off item not found")

// OneOffItem is a row from the one_off_items table.
type OneOffItem struct {
	ID        string
	Title     string
	CreatedAt time.Time
}

// OneOffItems is the repository for the one_off_items table, backed by
// sqlc-generated queries (see internal/storage/queries.sql and sqlcgen/) -
// this type just adapts sqlcgen's raw string created_at column to a
// time.Time on the way out, and generates IDs/timestamps on the way in, so
// callers never deal with the storage-level representation.
type OneOffItems struct {
	q *sqlcgen.Queries
}

// NewOneOffItems returns an OneOffItems repository backed by db.
func NewOneOffItems(db *sql.DB) *OneOffItems {
	return &OneOffItems{q: sqlcgen.New(db)}
}

// List returns every one-off item, oldest first.
func (i *OneOffItems) List(ctx context.Context) ([]OneOffItem, error) {
	rows, err := i.q.ListOneOffItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing one-off items: %w", err)
	}

	items := make([]OneOffItem, 0, len(rows))
	for _, row := range rows {
		item, err := fromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// Create inserts a new one-off item with a generated ID and returns it.
func (i *OneOffItems) Create(ctx context.Context, title string) (OneOffItem, error) {
	row, err := i.q.CreateOneOffItem(ctx, sqlcgen.CreateOneOffItemParams{
		ID:        uuid.NewString(),
		Title:     title,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return OneOffItem{}, fmt.Errorf("creating one-off item: %w", err)
	}

	return fromRow(row)
}

// Delete removes the one-off item with the given ID. It returns
// ErrOneOffItemNotFound if no such item exists.
func (i *OneOffItems) Delete(ctx context.Context, id string) error {
	affected, err := i.q.DeleteOneOffItem(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting one-off item: %w", err)
	}
	if affected == 0 {
		return ErrOneOffItemNotFound
	}

	return nil
}

func fromRow(row sqlcgen.OneOffItem) (OneOffItem, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return OneOffItem{}, fmt.Errorf("parsing one-off item created_at: %w", err)
	}

	return OneOffItem{ID: row.ID, Title: row.Title, CreatedAt: createdAt}, nil
}
