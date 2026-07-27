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

// ErrExerciseSetNotFound is returned when no exercise set matches the
// given ID.
var ErrExerciseSetNotFound = errors.New("exercise set not found")

// ExerciseSet is a row from the exercise_sets table - one prescribed set
// for a (cycle_exercise, block) pair. PercentageOfTM is nil for
// reps-only prescriptions (e.g. bodyweight/accessory work).
type ExerciseSet struct {
	ID              string
	CycleExerciseID string
	BlockID         string
	Position        int64
	Reps            int64
	PercentageOfTM  *float64
	CreatedAt       time.Time
}

// ExerciseSets is the repository for the exercise_sets table, backed by
// sqlc-generated queries (see internal/storage/queries.sql and sqlcgen/).
type ExerciseSets struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

// NewExerciseSets returns an ExerciseSets repository backed by db.
func NewExerciseSets(db *sql.DB) *ExerciseSets {
	return &ExerciseSets{db: db, q: sqlcgen.New(db)}
}

// ListByCycle returns every exercise set belonging to any cycle exercise
// in cycleID, ordered by (cycle_exercise_id, block_id, position) - the
// flat list GetCycle reassembles into a grid client-side.
func (e *ExerciseSets) ListByCycle(ctx context.Context, cycleID string) ([]ExerciseSet, error) {
	rows, err := e.q.ListExerciseSetsByCycle(ctx, cycleID)
	if err != nil {
		return nil, fmt.Errorf("listing exercise sets: %w", err)
	}

	sets := make([]ExerciseSet, 0, len(rows))
	for _, row := range rows {
		set, err := exerciseSetFromRow(row)
		if err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}

	return sets, nil
}

// Get returns the exercise set with the given ID. It returns
// ErrExerciseSetNotFound if no such exercise set exists.
func (e *ExerciseSets) Get(ctx context.Context, id string) (ExerciseSet, error) {
	row, err := e.q.GetExerciseSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExerciseSet{}, ErrExerciseSetNotFound
		}
		return ExerciseSet{}, fmt.Errorf("getting exercise set: %w", err)
	}

	return exerciseSetFromRow(row)
}

// Add appends a new set to the end of the (cycleExerciseID, blockID)
// cell's set list.
func (e *ExerciseSets) Add(ctx context.Context, cycleExerciseID, blockID string, reps int64, percentageOfTM *float64) (ExerciseSet, error) {
	var result ExerciseSet
	err := withTx(ctx, e.db, func(tx *sql.Tx) error {
		q := e.q.WithTx(tx)

		maxPos, err := q.MaxExerciseSetPosition(ctx, sqlcgen.MaxExerciseSetPositionParams{
			CycleExerciseID: cycleExerciseID, BlockID: blockID,
		})
		if err != nil {
			return fmt.Errorf("finding max exercise set position: %w", err)
		}
		pos, err := asPosition(maxPos)
		if err != nil {
			return err
		}

		row, err := q.CreateExerciseSet(ctx, sqlcgen.CreateExerciseSetParams{
			ID:              uuid.NewString(),
			CycleExerciseID: cycleExerciseID,
			BlockID:         blockID,
			Position:        pos + 1,
			Reps:            reps,
			PercentageOfTm:  toNullFloat64(percentageOfTM),
			CreatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return fmt.Errorf("creating exercise set: %w", err)
		}

		result, err = exerciseSetFromRow(row)
		return err
	})
	if err != nil {
		return ExerciseSet{}, err
	}

	return result, nil
}

// Update replaces the reps/percentage of the given exercise set. It
// returns ErrExerciseSetNotFound if no such exercise set exists.
func (e *ExerciseSets) Update(ctx context.Context, id string, reps int64, percentageOfTM *float64) (ExerciseSet, error) {
	row, err := e.q.UpdateExerciseSet(ctx, sqlcgen.UpdateExerciseSetParams{
		Reps:           reps,
		PercentageOfTm: toNullFloat64(percentageOfTM),
		ID:             id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExerciseSet{}, ErrExerciseSetNotFound
		}
		return ExerciseSet{}, fmt.Errorf("updating exercise set: %w", err)
	}

	return exerciseSetFromRow(row)
}

// Delete removes the exercise set with the given ID. It returns
// ErrExerciseSetNotFound if no such exercise set exists.
func (e *ExerciseSets) Delete(ctx context.Context, id string) error {
	affected, err := e.q.DeleteExerciseSet(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting exercise set: %w", err)
	}
	if affected == 0 {
		return ErrExerciseSetNotFound
	}

	return nil
}

// Move swaps the given exercise set's position with its nearest neighbor
// within the same (cycle_exercise_id, block_id) cell. Moving past a
// boundary (already first/last in its cell) is a no-op, not an error -
// same rationale as CycleExercises.Move.
func (e *ExerciseSets) Move(ctx context.Context, id string, up bool) error {
	return withTx(ctx, e.db, func(tx *sql.Tx) error {
		q := e.q.WithTx(tx)

		current, err := q.GetExerciseSet(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrExerciseSetNotFound
			}
			return fmt.Errorf("getting exercise set: %w", err)
		}

		var neighborID string
		var neighborPos int64
		if up {
			n, err := q.FindExerciseSetAbove(ctx, sqlcgen.FindExerciseSetAboveParams{
				CycleExerciseID: current.CycleExerciseID, BlockID: current.BlockID, Position: current.Position,
			})
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("finding exercise set above: %w", err)
			}
			neighborID, neighborPos = n.ID, n.Position
		} else {
			n, err := q.FindExerciseSetBelow(ctx, sqlcgen.FindExerciseSetBelowParams{
				CycleExerciseID: current.CycleExerciseID, BlockID: current.BlockID, Position: current.Position,
			})
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("finding exercise set below: %w", err)
			}
			neighborID, neighborPos = n.ID, n.Position
		}

		if err := q.SetExerciseSetPosition(ctx, sqlcgen.SetExerciseSetPositionParams{Position: neighborPos, ID: current.ID}); err != nil {
			return fmt.Errorf("setting exercise set position: %w", err)
		}
		if err := q.SetExerciseSetPosition(ctx, sqlcgen.SetExerciseSetPositionParams{Position: current.Position, ID: neighborID}); err != nil {
			return fmt.Errorf("setting neighbor exercise set position: %w", err)
		}

		return nil
	})
}

func toNullFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func fromNullFloat64(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

func exerciseSetFromRow(row sqlcgen.ExerciseSet) (ExerciseSet, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return ExerciseSet{}, fmt.Errorf("parsing exercise set created_at: %w", err)
	}

	return ExerciseSet{
		ID:              row.ID,
		CycleExerciseID: row.CycleExerciseID,
		BlockID:         row.BlockID,
		Position:        row.Position,
		Reps:            row.Reps,
		PercentageOfTM:  fromNullFloat64(row.PercentageOfTm),
		CreatedAt:       createdAt,
	}, nil
}
