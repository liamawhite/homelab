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

// Area categorizes a one-off item - add new values here and to
// migrations/0004_add_area_to_one_off_items.sql's doc comment together.
type Area string

const (
	AreaPersonal     Area = "personal"
	AreaFamily       Area = "family"
	AreaWork         Area = "work"
	AreaProfessional Area = "professional"
)

// OneOffItemStatus tracks whether a one-off item is still outstanding - add
// new values here and to migrations/0006_add_status_to_one_off_items.sql's
// doc comment together.
type OneOffItemStatus string

const (
	OneOffItemStatusOpen      OneOffItemStatus = "open"
	OneOffItemStatusCompleted OneOffItemStatus = "completed"
)

// OneOffItem is a row from the one_off_items table.
type OneOffItem struct {
	ID        string
	Title     string
	Notes     string
	CreatedAt time.Time
	DueDate   *time.Time
	Area      Area
	Status    OneOffItemStatus
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
// dueDate may be nil for no due date. New items always start open.
func (i *OneOffItems) Create(ctx context.Context, title string, notes string, dueDate *time.Time, area Area) (OneOffItem, error) {
	row, err := i.q.CreateOneOffItem(ctx, sqlcgen.CreateOneOffItemParams{
		ID:        uuid.NewString(),
		Title:     title,
		Notes:     notes,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		DueDate:   toNullString(dueDate),
		Area:      string(area),
		Status:    string(OneOffItemStatusOpen),
	})
	if err != nil {
		return OneOffItem{}, fmt.Errorf("creating one-off item: %w", err)
	}

	return fromRow(row)
}

// UpdateTitle sets the title for the one-off item with the given ID and
// returns it. It returns ErrOneOffItemNotFound if no such item exists.
func (i *OneOffItems) UpdateTitle(ctx context.Context, id string, title string) (OneOffItem, error) {
	row, err := i.q.UpdateOneOffItemTitle(ctx, sqlcgen.UpdateOneOffItemTitleParams{
		ID:    id,
		Title: title,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OneOffItem{}, ErrOneOffItemNotFound
		}
		return OneOffItem{}, fmt.Errorf("updating one-off item title: %w", err)
	}

	return fromRow(row)
}

// UpdateNotes sets the notes for the one-off item with the given ID and
// returns it. It returns ErrOneOffItemNotFound if no such item exists.
func (i *OneOffItems) UpdateNotes(ctx context.Context, id string, notes string) (OneOffItem, error) {
	row, err := i.q.UpdateOneOffItemNotes(ctx, sqlcgen.UpdateOneOffItemNotesParams{
		ID:    id,
		Notes: notes,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OneOffItem{}, ErrOneOffItemNotFound
		}
		return OneOffItem{}, fmt.Errorf("updating one-off item notes: %w", err)
	}

	return fromRow(row)
}

// UpdateDueDate sets the due date for the one-off item with the given ID and
// returns it. dueDate may be nil to clear it. It returns
// ErrOneOffItemNotFound if no such item exists.
func (i *OneOffItems) UpdateDueDate(ctx context.Context, id string, dueDate *time.Time) (OneOffItem, error) {
	row, err := i.q.UpdateOneOffItemDueDate(ctx, sqlcgen.UpdateOneOffItemDueDateParams{
		ID:      id,
		DueDate: toNullString(dueDate),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OneOffItem{}, ErrOneOffItemNotFound
		}
		return OneOffItem{}, fmt.Errorf("updating one-off item due date: %w", err)
	}

	return fromRow(row)
}

// UpdateArea sets the area for the one-off item with the given ID and
// returns it. It returns ErrOneOffItemNotFound if no such item exists.
func (i *OneOffItems) UpdateArea(ctx context.Context, id string, area Area) (OneOffItem, error) {
	row, err := i.q.UpdateOneOffItemArea(ctx, sqlcgen.UpdateOneOffItemAreaParams{
		ID:   id,
		Area: string(area),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OneOffItem{}, ErrOneOffItemNotFound
		}
		return OneOffItem{}, fmt.Errorf("updating one-off item area: %w", err)
	}

	return fromRow(row)
}

// UpdateStatus sets the status for the one-off item with the given ID and
// returns it. It returns ErrOneOffItemNotFound if no such item exists.
func (i *OneOffItems) UpdateStatus(ctx context.Context, id string, status OneOffItemStatus) (OneOffItem, error) {
	row, err := i.q.UpdateOneOffItemStatus(ctx, sqlcgen.UpdateOneOffItemStatusParams{
		ID:     id,
		Status: string(status),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OneOffItem{}, ErrOneOffItemNotFound
		}
		return OneOffItem{}, fmt.Errorf("updating one-off item status: %w", err)
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

	dueDate, err := fromNullString(row.DueDate)
	if err != nil {
		return OneOffItem{}, fmt.Errorf("parsing one-off item due_date: %w", err)
	}

	return OneOffItem{ID: row.ID, Title: row.Title, Notes: row.Notes, CreatedAt: createdAt, DueDate: dueDate, Area: Area(row.Area), Status: OneOffItemStatus(row.Status)}, nil
}

func toNullString(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339Nano), Valid: true}
}

func fromNullString(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
