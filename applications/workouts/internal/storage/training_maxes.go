package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/liamawhite/workouts/internal/storage/sqlcgen"
)

// WeightUnit is the unit a TrainingMax's weight was recorded in - add new
// values here and to the CHECK constraint in 0003_create_training_maxes.sql
// together.
type WeightUnit string

const (
	WeightUnitKg WeightUnit = "kg"
	WeightUnitLb WeightUnit = "lb"
)

// TrainingMax is one recorded value for a user's exercise, not the "current"
// concept - see TrainingMaxes.ListCurrent/ListHistory for how "current" vs.
// history are derived from these rows.
type TrainingMax struct {
	ID           string
	UserID       string
	ExerciseID   string
	ExerciseName string
	Weight       float64
	Unit         WeightUnit
	EffectiveAt  time.Time
	CreatedAt    time.Time
}

// TrainingMaxes is the repository for the training_maxes table, backed by
// sqlc-generated queries (see internal/storage/queries.sql and sqlcgen/).
type TrainingMaxes struct {
	q *sqlcgen.Queries
}

// NewTrainingMaxes returns a TrainingMaxes repository backed by db.
func NewTrainingMaxes(db *sql.DB) *TrainingMaxes {
	return &TrainingMaxes{q: sqlcgen.New(db)}
}

// ListCurrent returns one TrainingMax per exercise for the given user - the
// row with that exercise's latest effective_at, ordered by exercise name.
func (t *TrainingMaxes) ListCurrent(ctx context.Context, userID string) ([]TrainingMax, error) {
	rows, err := t.q.ListCurrentTrainingMaxes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing current training maxes: %w", err)
	}

	maxes := make([]TrainingMax, 0, len(rows))
	for _, row := range rows {
		max, err := trainingMaxFromCurrentRow(row)
		if err != nil {
			return nil, err
		}
		maxes = append(maxes, max)
	}

	return maxes, nil
}

// ListHistory returns every recorded value for the given user+exercise,
// newest first.
func (t *TrainingMaxes) ListHistory(ctx context.Context, userID, exerciseID string) ([]TrainingMax, error) {
	rows, err := t.q.ListTrainingMaxHistory(ctx, sqlcgen.ListTrainingMaxHistoryParams{
		UserID:     userID,
		ExerciseID: exerciseID,
	})
	if err != nil {
		return nil, fmt.Errorf("listing training max history: %w", err)
	}

	maxes := make([]TrainingMax, 0, len(rows))
	for _, row := range rows {
		max, err := trainingMaxFromHistoryRow(row)
		if err != nil {
			return nil, err
		}
		maxes = append(maxes, max)
	}

	return maxes, nil
}

// Record inserts a new training max history row for userID+exercise. The
// caller passes an already-resolved Exercise (as Items.Create does with a
// resolved Label) so this can fill ExerciseName without a second query.
func (t *TrainingMaxes) Record(ctx context.Context, userID string, exercise Exercise, weight float64, unit WeightUnit, effectiveAt time.Time) (TrainingMax, error) {
	row, err := t.q.CreateTrainingMax(ctx, sqlcgen.CreateTrainingMaxParams{
		ID:          uuid.NewString(),
		UserID:      userID,
		ExerciseID:  exercise.ID,
		Weight:      weight,
		Unit:        string(unit),
		EffectiveAt: effectiveAt.UTC().Format(time.RFC3339Nano),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return TrainingMax{}, fmt.Errorf("recording training max: %w", err)
	}

	effective, err := time.Parse(time.RFC3339Nano, row.EffectiveAt)
	if err != nil {
		return TrainingMax{}, fmt.Errorf("parsing training max effective_at: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return TrainingMax{}, fmt.Errorf("parsing training max created_at: %w", err)
	}

	return TrainingMax{
		ID:           row.ID,
		UserID:       row.UserID,
		ExerciseID:   row.ExerciseID,
		ExerciseName: exercise.Name,
		Weight:       row.Weight,
		Unit:         WeightUnit(row.Unit),
		EffectiveAt:  effective,
		CreatedAt:    createdAt,
	}, nil
}

func trainingMaxFromCurrentRow(row sqlcgen.ListCurrentTrainingMaxesRow) (TrainingMax, error) {
	effective, err := time.Parse(time.RFC3339Nano, row.EffectiveAt)
	if err != nil {
		return TrainingMax{}, fmt.Errorf("parsing training max effective_at: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return TrainingMax{}, fmt.Errorf("parsing training max created_at: %w", err)
	}

	return TrainingMax{
		ID:           row.ID,
		UserID:       row.UserID,
		ExerciseID:   row.ExerciseID,
		ExerciseName: row.ExerciseName,
		Weight:       row.Weight,
		Unit:         WeightUnit(row.Unit),
		EffectiveAt:  effective,
		CreatedAt:    createdAt,
	}, nil
}

func trainingMaxFromHistoryRow(row sqlcgen.ListTrainingMaxHistoryRow) (TrainingMax, error) {
	effective, err := time.Parse(time.RFC3339Nano, row.EffectiveAt)
	if err != nil {
		return TrainingMax{}, fmt.Errorf("parsing training max effective_at: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return TrainingMax{}, fmt.Errorf("parsing training max created_at: %w", err)
	}

	return TrainingMax{
		ID:           row.ID,
		UserID:       row.UserID,
		ExerciseID:   row.ExerciseID,
		ExerciseName: row.ExerciseName,
		Weight:       row.Weight,
		Unit:         WeightUnit(row.Unit),
		EffectiveAt:  effective,
		CreatedAt:    createdAt,
	}, nil
}
