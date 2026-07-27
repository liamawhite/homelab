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

// ErrCycleExerciseNotFound is returned when no cycle exercise matches the
// given ID.
var ErrCycleExerciseNotFound = errors.New("cycle exercise not found")

// CycleExercise is a row from the cycle_exercises table - one entry in a
// Cycle's shared, ordered exercise lineup. Every Block implicitly has this
// exercise available (see 0008_create_cycle_exercises.sql's doc comment).
type CycleExercise struct {
	ID                string
	CycleID           string
	ExerciseID        string
	ExerciseName      string
	ExerciseCategory  ExerciseCategory
	ExerciseEquipment ExerciseEquipment
	Position          int64
	CreatedAt         time.Time
}

// CycleExercises is the repository for the cycle_exercises table, backed by
// sqlc-generated queries (see internal/storage/queries.sql and sqlcgen/).
type CycleExercises struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

// NewCycleExercises returns a CycleExercises repository backed by db.
func NewCycleExercises(db *sql.DB) *CycleExercises {
	return &CycleExercises{db: db, q: sqlcgen.New(db)}
}

// ListByCycle returns every cycle exercise for cycleID, ordered by the
// cycle-wide position.
func (c *CycleExercises) ListByCycle(ctx context.Context, cycleID string) ([]CycleExercise, error) {
	rows, err := c.q.ListCycleExercisesByCycle(ctx, cycleID)
	if err != nil {
		return nil, fmt.Errorf("listing cycle exercises: %w", err)
	}

	exercises := make([]CycleExercise, 0, len(rows))
	for _, row := range rows {
		exercise, err := newCycleExercise(
			row.ID, row.CycleID, row.ExerciseID, row.ExerciseName, row.ExerciseCategory, row.ExerciseEquipment,
			row.Position, row.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}

// Get returns the cycle exercise with the given ID. It returns
// ErrCycleExerciseNotFound if no such cycle exercise exists.
func (c *CycleExercises) Get(ctx context.Context, id string) (CycleExercise, error) {
	row, err := c.q.GetCycleExercise(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CycleExercise{}, ErrCycleExerciseNotFound
		}
		return CycleExercise{}, fmt.Errorf("getting cycle exercise: %w", err)
	}

	return newCycleExercise(
		row.ID, row.CycleID, row.ExerciseID, row.ExerciseName, row.ExerciseCategory, row.ExerciseEquipment,
		row.Position, row.CreatedAt,
	)
}

// Add appends exercise to the end of cycleID's exercise lineup. The caller
// passes an already-resolved Exercise (mirrors TrainingMaxes.Record's
// pattern) so ExerciseName/Category/Equipment are filled without a second
// query.
func (c *CycleExercises) Add(ctx context.Context, cycleID string, exercise Exercise) (CycleExercise, error) {
	var result CycleExercise
	err := withTx(ctx, c.db, func(tx *sql.Tx) error {
		q := c.q.WithTx(tx)

		maxPos, err := q.MaxCycleExercisePosition(ctx, cycleID)
		if err != nil {
			return fmt.Errorf("finding max cycle exercise position: %w", err)
		}
		pos, err := asPosition(maxPos)
		if err != nil {
			return err
		}

		row, err := q.CreateCycleExercise(ctx, sqlcgen.CreateCycleExerciseParams{
			ID:         uuid.NewString(),
			CycleID:    cycleID,
			ExerciseID: exercise.ID,
			Position:   pos + 1,
			CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return fmt.Errorf("creating cycle exercise: %w", err)
		}

		result, err = newCycleExercise(
			row.ID, row.CycleID, row.ExerciseID, exercise.Name, string(exercise.Category), string(exercise.Equipment),
			row.Position, row.CreatedAt,
		)
		return err
	})
	if err != nil {
		return CycleExercise{}, err
	}

	return result, nil
}

// Delete removes the cycle exercise with the given ID, cascading to its
// exercise_sets. It returns ErrCycleExerciseNotFound if no such cycle
// exercise exists.
func (c *CycleExercises) Delete(ctx context.Context, id string) error {
	affected, err := c.q.DeleteCycleExercise(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting cycle exercise: %w", err)
	}
	if affected == 0 {
		return ErrCycleExerciseNotFound
	}

	return nil
}

// Move swaps the given cycle exercise's position with its nearest
// same-category neighbor (up = smaller position, down = larger position),
// so category-grouped display stays consistent cycle-wide. Moving past a
// category-group boundary (already first/last) is a no-op, not an error -
// the frontend disables the corresponding button at boundaries (computed
// from its already-loaded, already-ordered list), so this path only fires
// from a stale-UI race.
func (c *CycleExercises) Move(ctx context.Context, id string, up bool) error {
	return withTx(ctx, c.db, func(tx *sql.Tx) error {
		q := c.q.WithTx(tx)

		current, err := q.GetCycleExercise(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrCycleExerciseNotFound
			}
			return fmt.Errorf("getting cycle exercise: %w", err)
		}

		var neighborID string
		var neighborPos int64
		if up {
			n, err := q.FindCycleExerciseAbove(ctx, sqlcgen.FindCycleExerciseAboveParams{
				CycleID: current.CycleID, Category: current.ExerciseCategory, Position: current.Position,
			})
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("finding cycle exercise above: %w", err)
			}
			neighborID, neighborPos = n.ID, n.Position
		} else {
			n, err := q.FindCycleExerciseBelow(ctx, sqlcgen.FindCycleExerciseBelowParams{
				CycleID: current.CycleID, Category: current.ExerciseCategory, Position: current.Position,
			})
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("finding cycle exercise below: %w", err)
			}
			neighborID, neighborPos = n.ID, n.Position
		}

		if err := q.SetCycleExercisePosition(ctx, sqlcgen.SetCycleExercisePositionParams{Position: neighborPos, ID: current.ID}); err != nil {
			return fmt.Errorf("setting cycle exercise position: %w", err)
		}
		if err := q.SetCycleExercisePosition(ctx, sqlcgen.SetCycleExercisePositionParams{Position: current.Position, ID: neighborID}); err != nil {
			return fmt.Errorf("setting neighbor cycle exercise position: %w", err)
		}

		return nil
	})
}

func newCycleExercise(id, cycleID, exerciseID, exerciseName, exerciseCategory, exerciseEquipment string, position int64, createdAtStr string) (CycleExercise, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return CycleExercise{}, fmt.Errorf("parsing cycle exercise created_at: %w", err)
	}

	return CycleExercise{
		ID:                id,
		CycleID:           cycleID,
		ExerciseID:        exerciseID,
		ExerciseName:      exerciseName,
		ExerciseCategory:  ExerciseCategory(exerciseCategory),
		ExerciseEquipment: ExerciseEquipment(exerciseEquipment),
		Position:          position,
		CreatedAt:         createdAt,
	}, nil
}
