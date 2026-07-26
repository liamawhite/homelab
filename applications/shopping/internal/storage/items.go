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

// ItemStatus is a TEXT enum (SQLite has no native enum type) - add new
// values here and to the CHECK constraint in
// 0005_add_items_status.sql together.
type ItemStatus string

const (
	ItemStatusTodo ItemStatus = "todo"
	ItemStatusDone ItemStatus = "done"
)

// Item is a row from the items table, joined with its label's name so
// callers never need a second lookup to display it. CompletedAt is non-nil
// if and only if Status is ItemStatusDone - enforced by a CHECK constraint
// on the table itself, not just this package.
type Item struct {
	ID          string
	Name        string
	LabelID     string
	LabelName   string
	Status      ItemStatus
	CompletedAt *time.Time
	CreatedAt   time.Time
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

// List returns every item, in no particular order - display order is a
// client concern (see web/src/lib/items.ts's useItems).
func (i *Items) List(ctx context.Context) ([]Item, error) {
	rows, err := i.q.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing items: %w", err)
	}

	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		item, err := newItem(row.ID, row.Name, row.LabelID, row.LabelName, row.Status, row.CompletedAt, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

// Create inserts a new item with a generated ID under label (caller has
// already resolved and validated label, so its name is passed straight
// through here rather than looked up again). New items always start
// todo/uncompleted.
func (i *Items) Create(ctx context.Context, name string, label Label) (Item, error) {
	row, err := i.q.CreateItem(ctx, sqlcgen.CreateItemParams{
		ID:        uuid.NewString(),
		Name:      name,
		LabelID:   label.ID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Item{}, fmt.Errorf("creating item: %w", err)
	}

	return newItem(row.ID, row.Name, row.LabelID, label.Name, row.Status, row.CompletedAt, row.CreatedAt)
}

// SetStatus updates an item's status, setting or clearing CompletedAt to
// match automatically - callers never supply a timestamp directly, so a
// done item can't end up without one (or a todo item with one). The
// returned Item's LabelName is left blank - this query doesn't join
// labels, so callers that need it (itemservice, for the API response)
// look it up separately. Returns ErrItemNotFound if no such item exists.
func (i *Items) SetStatus(ctx context.Context, id string, status ItemStatus) (Item, error) {
	var completedAt sql.NullString
	if status == ItemStatusDone {
		completedAt = sql.NullString{String: time.Now().UTC().Format(time.RFC3339Nano), Valid: true}
	}

	row, err := i.q.SetItemStatus(ctx, sqlcgen.SetItemStatusParams{
		Status:      string(status),
		CompletedAt: completedAt,
		ID:          id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, ErrItemNotFound
		}
		return Item{}, fmt.Errorf("setting item status: %w", err)
	}

	return newItem(row.ID, row.Name, row.LabelID, "", row.Status, row.CompletedAt, row.CreatedAt)
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

func newItem(id, name, labelID, labelName, status string, completedAt sql.NullString, createdAt string) (Item, error) {
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Item{}, fmt.Errorf("parsing item created_at: %w", err)
	}

	var completedAtPtr *time.Time
	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return Item{}, fmt.Errorf("parsing item completed_at: %w", err)
		}
		completedAtPtr = &t
	}

	return Item{
		ID:          id,
		Name:        name,
		LabelID:     labelID,
		LabelName:   labelName,
		Status:      ItemStatus(status),
		CompletedAt: completedAtPtr,
		CreatedAt:   parsedCreatedAt,
	}, nil
}
