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

// ErrExerciseNotFound is returned when no exercise matches the given ID.
var ErrExerciseNotFound = errors.New("exercise not found")

// ExerciseCategory distinguishes the main lifts that carry a training max
// from accessory work that doesn't - add new values here and to the CHECK
// constraint in 0002_create_exercises.sql together.
type ExerciseCategory string

const (
	ExerciseCategoryMainLift  ExerciseCategory = "main_lift"
	ExerciseCategoryAccessory ExerciseCategory = "accessory"
)

// ExerciseEquipment is what an exercise is performed with - add new values
// here and to the CHECK constraint in 0004_add_exercises_equipment.sql
// together.
type ExerciseEquipment string

const (
	ExerciseEquipmentBarbell    ExerciseEquipment = "barbell"
	ExerciseEquipmentDumbbell   ExerciseEquipment = "dumbbell"
	ExerciseEquipmentBodyweight ExerciseEquipment = "bodyweight"
)

// Exercise is a row from the exercises table.
type Exercise struct {
	ID        string
	Name      string
	Category  ExerciseCategory
	Equipment ExerciseEquipment
	Archived  bool
	CreatedAt time.Time
}

// Exercises is the repository for the exercises table, backed by
// sqlc-generated queries (see internal/storage/queries.sql and sqlcgen/).
type Exercises struct {
	q *sqlcgen.Queries
}

// NewExercises returns an Exercises repository backed by db.
func NewExercises(db *sql.DB) *Exercises {
	return &Exercises{q: sqlcgen.New(db)}
}

// List returns every exercise, active and archived, oldest first.
func (e *Exercises) List(ctx context.Context) ([]Exercise, error) {
	rows, err := e.q.ListExercises(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing exercises: %w", err)
	}

	exercises := make([]Exercise, 0, len(rows))
	for _, row := range rows {
		exercise, err := newExercise(row.ID, row.Name, row.Category, row.Equipment, row.Archived, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}

// Get returns the exercise with the given ID. It returns
// ErrExerciseNotFound if no such exercise exists.
func (e *Exercises) Get(ctx context.Context, id string) (Exercise, error) {
	row, err := e.q.GetExercise(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Exercise{}, ErrExerciseNotFound
		}
		return Exercise{}, fmt.Errorf("getting exercise: %w", err)
	}

	return newExercise(row.ID, row.Name, row.Category, row.Equipment, row.Archived, row.CreatedAt)
}

// Create inserts a new, active exercise with a generated ID and returns it.
func (e *Exercises) Create(ctx context.Context, name string, category ExerciseCategory, equipment ExerciseEquipment) (Exercise, error) {
	row, err := e.q.CreateExercise(ctx, sqlcgen.CreateExerciseParams{
		ID:        uuid.NewString(),
		Name:      name,
		Category:  string(category),
		Equipment: string(equipment),
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return Exercise{}, fmt.Errorf("creating exercise: %w", err)
	}

	return newExercise(row.ID, row.Name, row.Category, row.Equipment, row.Archived, row.CreatedAt)
}

// Archive hides the exercise with the given ID from selection without
// deleting it or the training maxes that reference it. It returns
// ErrExerciseNotFound if no such exercise exists.
func (e *Exercises) Archive(ctx context.Context, id string) error {
	affected, err := e.q.ArchiveExercise(ctx, id)
	if err != nil {
		return fmt.Errorf("archiving exercise: %w", err)
	}
	if affected == 0 {
		return ErrExerciseNotFound
	}

	return nil
}

// Restore un-archives the exercise with the given ID. It returns
// ErrExerciseNotFound if no such exercise exists.
func (e *Exercises) Restore(ctx context.Context, id string) error {
	affected, err := e.q.RestoreExercise(ctx, id)
	if err != nil {
		return fmt.Errorf("restoring exercise: %w", err)
	}
	if affected == 0 {
		return ErrExerciseNotFound
	}

	return nil
}

// newExercise adapts the columns shared by ListExercises/GetExercise/
// CreateExercise's distinct sqlc row types (they differ because the
// equipment column, added by 0004_add_exercises_equipment.sql, doesn't
// match the Exercise model struct's field order) into an Exercise.
func newExercise(id, name, category, equipment string, archived int64, createdAtStr string) (Exercise, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return Exercise{}, fmt.Errorf("parsing exercise created_at: %w", err)
	}

	return Exercise{
		ID:        id,
		Name:      name,
		Category:  ExerciseCategory(category),
		Equipment: ExerciseEquipment(equipment),
		Archived:  archived != 0,
		CreatedAt: createdAt,
	}, nil
}
