package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/liamawhite/shopping/internal/storage/sqlcgen"
)

// ErrItemNotFound is returned by Delete when no item matches the given ID.
var ErrItemNotFound = errors.New("item not found")

// Item is a row from the items table.
type Item struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Items is the repository for the items table, backed by sqlc-generated
// queries (see internal/storage/queries.sql and sqlcgen/) - this type just
// adapts sqlcgen's raw string created_at column to a time.Time on the way
// out, and generates IDs/timestamps on the way in, so callers never deal
// with the storage-level representation.
type Items struct {
	q *sqlcgen.Queries
}

// NewItems returns an Items repository backed by db.
func NewItems(db *sql.DB) *Items {
	return &Items{q: sqlcgen.New(db)}
}

// List returns every item, oldest first.
func (i *Items) List(ctx context.Context) ([]Item, error) {
	rows, err := i.q.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing items: %w", err)
	}

	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		item, err := fromRow(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// Create inserts a new item with a generated ID and returns it.
func (i *Items) Create(ctx context.Context, name string) (Item, error) {
	row, err := i.q.CreateItem(ctx, sqlcgen.CreateItemParams{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Item{}, fmt.Errorf("creating item: %w", err)
	}

	return fromRow(row)
}

// Delete removes the item with the given ID. It returns ErrItemNotFound if
// no such item exists.
func (i *Items) Delete(ctx context.Context, id string) error {
	affected, err := i.q.DeleteItem(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting item: %w", err)
	}
	if affected == 0 {
		return ErrItemNotFound
	}

	return nil
}

func fromRow(row sqlcgen.Item) (Item, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return Item{}, fmt.Errorf("parsing item created_at: %w", err)
	}

	return Item{ID: row.ID, Name: row.Name, CreatedAt: createdAt}, nil
}
