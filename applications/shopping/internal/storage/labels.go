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

// ErrLabelNotFound is returned when no label matches the given ID.
var ErrLabelNotFound = errors.New("label not found")

// ErrInvalidColor is returned when a color isn't one of LabelPalette's
// values.
var ErrInvalidColor = errors.New("color must be one of the palette values")

// LabelPalette is the fixed set of hex colors a label's color can ever be -
// add new values here and to the CHECK constraint in
// 0006_add_labels_color.sql together. Labels are auto-assigned the next
// color in this list (by creation order) and can be reassigned to any other
// entry afterward, but never to an arbitrary hex value.
var LabelPalette = []string{
	"#e03131", "#f08c00", "#f5c518", "#2f9e44", "#0ca678",
	"#1971c2", "#4263eb", "#7048e8", "#ae3ec9", "#e64980",
}

// Label is a row from the labels table.
type Label struct {
	ID        string
	Name      string
	Archived  bool
	Color     string
	CreatedAt time.Time
}

// Labels is the repository for the labels table, backed by sqlc-generated
// queries (see internal/storage/queries.sql and sqlcgen/).
type Labels struct {
	q *sqlcgen.Queries
}

// NewLabels returns a Labels repository backed by db.
func NewLabels(db *sql.DB) *Labels {
	return &Labels{q: sqlcgen.New(db)}
}

// List returns every label, in no particular order - display order is a
// client concern (see web/src/lib/labels.ts's useLabels).
func (l *Labels) List(ctx context.Context) ([]Label, error) {
	rows, err := l.q.ListLabels(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing labels: %w", err)
	}

	labels := make([]Label, 0, len(rows))
	for _, row := range rows {
		label, err := labelFromRow(row)
		if err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}

	return labels, nil
}

// Get returns the label with the given ID. It returns ErrLabelNotFound if no
// such label exists.
func (l *Labels) Get(ctx context.Context, id string) (Label, error) {
	row, err := l.q.GetLabel(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Label{}, ErrLabelNotFound
		}
		return Label{}, fmt.Errorf("getting label: %w", err)
	}

	return labelFromRow(row)
}

// Create inserts a new, active label with a generated ID and returns it.
// Its color is auto-assigned by rotating through LabelPalette based on how
// many labels already exist (archived ones included, so the rotation never
// resets).
func (l *Labels) Create(ctx context.Context, name string) (Label, error) {
	count, err := l.q.CountLabels(ctx)
	if err != nil {
		return Label{}, fmt.Errorf("counting labels: %w", err)
	}
	color := LabelPalette[int(count)%len(LabelPalette)]

	row, err := l.q.CreateLabel(ctx, sqlcgen.CreateLabelParams{
		ID:        uuid.NewString(),
		Name:      name,
		Color:     color,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Label{}, fmt.Errorf("creating label: %w", err)
	}

	return labelFromRow(row)
}

// Archive hides the label with the given ID from suggestions/tabs without
// deleting it or the items that reference it. It returns ErrLabelNotFound if
// no such label exists.
func (l *Labels) Archive(ctx context.Context, id string) error {
	affected, err := l.q.ArchiveLabel(ctx, id)
	if err != nil {
		return fmt.Errorf("archiving label: %w", err)
	}
	if affected == 0 {
		return ErrLabelNotFound
	}

	return nil
}

// Restore un-archives the label with the given ID. It returns
// ErrLabelNotFound if no such label exists.
func (l *Labels) Restore(ctx context.Context, id string) error {
	affected, err := l.q.RestoreLabel(ctx, id)
	if err != nil {
		return fmt.Errorf("restoring label: %w", err)
	}
	if affected == 0 {
		return ErrLabelNotFound
	}

	return nil
}

// SetColor reassigns a label's color. It returns ErrInvalidColor if color
// isn't one of LabelPalette's values, or ErrLabelNotFound if no such label
// exists.
func (l *Labels) SetColor(ctx context.Context, id, color string) (Label, error) {
	valid := false
	for _, c := range LabelPalette {
		if c == color {
			valid = true
			break
		}
	}
	if !valid {
		return Label{}, ErrInvalidColor
	}

	row, err := l.q.SetLabelColor(ctx, sqlcgen.SetLabelColorParams{Color: color, ID: id})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Label{}, ErrLabelNotFound
		}
		return Label{}, fmt.Errorf("setting label color: %w", err)
	}

	return labelFromRow(row)
}

func labelFromRow(row sqlcgen.Label) (Label, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return Label{}, fmt.Errorf("parsing label created_at: %w", err)
	}

	return Label{ID: row.ID, Name: row.Name, Archived: row.Archived != 0, Color: row.Color, CreatedAt: createdAt}, nil
}
