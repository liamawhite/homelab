package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/liamawhite/workouts/internal/storage/sqlcgen"
)

// ErrCycleNotFound is returned when no cycle matches the given ID.
var ErrCycleNotFound = errors.New("cycle not found")

// Cycle is a row from the cycles table - a single-user training program.
type Cycle struct {
	ID        string
	UserID    string
	Name      string
	CreatedAt time.Time
}

// Cycles is the repository for the cycles table, backed by sqlc-generated
// queries (see internal/storage/queries.sql and sqlcgen/).
type Cycles struct {
	q *sqlcgen.Queries
}

// NewCycles returns a Cycles repository backed by db.
func NewCycles(db *sql.DB) *Cycles {
	return &Cycles{q: sqlcgen.New(db)}
}

// ListByUser returns every cycle belonging to userID, oldest first.
func (c *Cycles) ListByUser(ctx context.Context, userID string) ([]Cycle, error) {
	rows, err := c.q.ListCyclesByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing cycles: %w", err)
	}

	cycles := make([]Cycle, 0, len(rows))
	for _, row := range rows {
		cycle, err := cycleFromRow(row)
		if err != nil {
			return nil, err
		}
		cycles = append(cycles, cycle)
	}

	return cycles, nil
}

// Get returns the cycle with the given ID. It returns ErrCycleNotFound if
// no such cycle exists.
func (c *Cycles) Get(ctx context.Context, id string) (Cycle, error) {
	row, err := c.q.GetCycle(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Cycle{}, ErrCycleNotFound
		}
		return Cycle{}, fmt.Errorf("getting cycle: %w", err)
	}

	return cycleFromRow(row)
}

// Create inserts a new cycle with a generated ID and returns it.
func (c *Cycles) Create(ctx context.Context, userID, name string) (Cycle, error) {
	row, err := c.q.CreateCycle(ctx, sqlcgen.CreateCycleParams{
		ID:        uuid.NewString(),
		UserID:    userID,
		Name:      name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Cycle{}, fmt.Errorf("creating cycle: %w", err)
	}

	return cycleFromRow(row)
}

// Delete removes the cycle with the given ID, cascading to its blocks,
// cycle_exercises, and exercise_sets. It returns ErrCycleNotFound if no
// such cycle exists.
func (c *Cycles) Delete(ctx context.Context, id string) error {
	affected, err := c.q.DeleteCycle(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting cycle: %w", err)
	}
	if affected == 0 {
		return ErrCycleNotFound
	}

	return nil
}

func cycleFromRow(row sqlcgen.Cycle) (Cycle, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return Cycle{}, fmt.Errorf("parsing cycle created_at: %w", err)
	}

	return Cycle{
		ID:        row.ID,
		UserID:    row.UserID,
		Name:      row.Name,
		CreatedAt: createdAt,
	}, nil
}
