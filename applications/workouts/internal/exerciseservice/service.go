// Package exerciseservice implements the workouts.v1.ExerciseService Connect
// handler over the storage.Exercises repository.
package exerciseservice

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/workouts/gen/workouts/v1"
	"github.com/liamawhite/workouts/internal/storage"
)

// Service implements workoutsv1connect.ExerciseServiceHandler.
type Service struct {
	exercises *storage.Exercises
}

// New returns a Service backed by exercises.
func New(exercises *storage.Exercises) *Service {
	return &Service{exercises: exercises}
}

// ListExercises returns every exercise, active and archived.
func (s *Service) ListExercises(ctx context.Context, req *connect.Request[v1.ListExercisesRequest]) (*connect.Response[v1.ListExercisesResponse], error) {
	exercises, err := s.exercises.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListExercisesResponse{
		Exercises: make([]*v1.Exercise, 0, len(exercises)),
	}
	for _, exercise := range exercises {
		resp.Exercises = append(resp.Exercises, toProto(exercise))
	}

	return connect.NewResponse(resp), nil
}

// CreateExercise creates a new, active exercise.
func (s *Service) CreateExercise(ctx context.Context, req *connect.Request[v1.CreateExerciseRequest]) (*connect.Response[v1.CreateExerciseResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	category, err := fromProtoCategory(req.Msg.GetCategory())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	equipment, err := fromProtoEquipment(req.Msg.GetEquipment())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	exercise, err := s.exercises.Create(ctx, name, category, equipment)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateExerciseResponse{Exercise: toProto(exercise)}), nil
}

// ArchiveExercise archives an exercise - it's kept, along with every
// training max that references it, just hidden from selection (see
// storage.Exercises.Archive's doc comment).
func (s *Service) ArchiveExercise(ctx context.Context, req *connect.Request[v1.ArchiveExerciseRequest]) (*connect.Response[v1.ArchiveExerciseResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.exercises.Archive(ctx, id); err != nil {
		if errors.Is(err, storage.ErrExerciseNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.ArchiveExerciseResponse{}), nil
}

// RestoreExercise un-archives an exercise.
func (s *Service) RestoreExercise(ctx context.Context, req *connect.Request[v1.RestoreExerciseRequest]) (*connect.Response[v1.RestoreExerciseResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.exercises.Restore(ctx, id); err != nil {
		if errors.Is(err, storage.ErrExerciseNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.RestoreExerciseResponse{}), nil
}

func toProto(exercise storage.Exercise) *v1.Exercise {
	return &v1.Exercise{
		Id:        exercise.ID,
		Name:      exercise.Name,
		Category:  toProtoCategory(exercise.Category),
		Equipment: toProtoEquipment(exercise.Equipment),
		Archived:  exercise.Archived,
		CreatedAt: timestamppb.New(exercise.CreatedAt),
	}
}

func toProtoCategory(category storage.ExerciseCategory) v1.ExerciseCategory {
	switch category {
	case storage.ExerciseCategoryMainLift:
		return v1.ExerciseCategory_EXERCISE_CATEGORY_MAIN_LIFT
	case storage.ExerciseCategoryAccessory:
		return v1.ExerciseCategory_EXERCISE_CATEGORY_ACCESSORY
	default:
		return v1.ExerciseCategory_EXERCISE_CATEGORY_UNSPECIFIED
	}
}

func fromProtoCategory(category v1.ExerciseCategory) (storage.ExerciseCategory, error) {
	switch category {
	case v1.ExerciseCategory_EXERCISE_CATEGORY_MAIN_LIFT:
		return storage.ExerciseCategoryMainLift, nil
	case v1.ExerciseCategory_EXERCISE_CATEGORY_ACCESSORY:
		return storage.ExerciseCategoryAccessory, nil
	default:
		return "", errors.New("category must be MAIN_LIFT or ACCESSORY")
	}
}

func toProtoEquipment(equipment storage.ExerciseEquipment) v1.ExerciseEquipment {
	switch equipment {
	case storage.ExerciseEquipmentBarbell:
		return v1.ExerciseEquipment_EXERCISE_EQUIPMENT_BARBELL
	case storage.ExerciseEquipmentDumbbell:
		return v1.ExerciseEquipment_EXERCISE_EQUIPMENT_DUMBBELL
	case storage.ExerciseEquipmentBodyweight:
		return v1.ExerciseEquipment_EXERCISE_EQUIPMENT_BODYWEIGHT
	default:
		return v1.ExerciseEquipment_EXERCISE_EQUIPMENT_UNSPECIFIED
	}
}

func fromProtoEquipment(equipment v1.ExerciseEquipment) (storage.ExerciseEquipment, error) {
	switch equipment {
	case v1.ExerciseEquipment_EXERCISE_EQUIPMENT_BARBELL:
		return storage.ExerciseEquipmentBarbell, nil
	case v1.ExerciseEquipment_EXERCISE_EQUIPMENT_DUMBBELL:
		return storage.ExerciseEquipmentDumbbell, nil
	case v1.ExerciseEquipment_EXERCISE_EQUIPMENT_BODYWEIGHT:
		return storage.ExerciseEquipmentBodyweight, nil
	default:
		return "", errors.New("equipment must be BARBELL, DUMBBELL, or BODYWEIGHT")
	}
}
