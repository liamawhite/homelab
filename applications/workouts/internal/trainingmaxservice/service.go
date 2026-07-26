// Package trainingmaxservice implements the workouts.v1.TrainingMaxService
// Connect handler over the storage.TrainingMaxes repository.
package trainingmaxservice

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/workouts/gen/workouts/v1"
	"github.com/liamawhite/workouts/internal/storage"
)

// Service implements workoutsv1connect.TrainingMaxServiceHandler.
type Service struct {
	trainingMaxes *storage.TrainingMaxes
	users         *storage.Users
	exercises     *storage.Exercises
}

// New returns a Service backed by trainingMaxes, users, and exercises - users
// and exercises are needed to validate the user_id/exercise_id a training max
// is recorded against.
func New(trainingMaxes *storage.TrainingMaxes, users *storage.Users, exercises *storage.Exercises) *Service {
	return &Service{trainingMaxes: trainingMaxes, users: users, exercises: exercises}
}

// ListCurrentTrainingMaxes returns one training max per exercise for the
// given user - each exercise's latest recorded value.
func (s *Service) ListCurrentTrainingMaxes(ctx context.Context, req *connect.Request[v1.ListCurrentTrainingMaxesRequest]) (*connect.Response[v1.ListCurrentTrainingMaxesResponse], error) {
	userID := strings.TrimSpace(req.Msg.GetUserId())
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id must not be empty"))
	}

	if _, err := s.users.Get(ctx, userID); err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	maxes, err := s.trainingMaxes.ListCurrent(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListCurrentTrainingMaxesResponse{
		TrainingMaxes: make([]*v1.TrainingMax, 0, len(maxes)),
	}
	for _, max := range maxes {
		resp.TrainingMaxes = append(resp.TrainingMaxes, toProto(max))
	}

	return connect.NewResponse(resp), nil
}

// ListTrainingMaxHistory returns every recorded value for the given
// user+exercise, newest first.
func (s *Service) ListTrainingMaxHistory(ctx context.Context, req *connect.Request[v1.ListTrainingMaxHistoryRequest]) (*connect.Response[v1.ListTrainingMaxHistoryResponse], error) {
	userID := strings.TrimSpace(req.Msg.GetUserId())
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id must not be empty"))
	}
	exerciseID := strings.TrimSpace(req.Msg.GetExerciseId())
	if exerciseID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("exercise_id must not be empty"))
	}

	if _, err := s.users.Get(ctx, userID); err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if _, err := s.exercises.Get(ctx, exerciseID); err != nil {
		if errors.Is(err, storage.ErrExerciseNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	maxes, err := s.trainingMaxes.ListHistory(ctx, userID, exerciseID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListTrainingMaxHistoryResponse{
		TrainingMaxes: make([]*v1.TrainingMax, 0, len(maxes)),
	}
	for _, max := range maxes {
		resp.TrainingMaxes = append(resp.TrainingMaxes, toProto(max))
	}

	return connect.NewResponse(resp), nil
}

// RecordTrainingMax inserts a new training max history row for a user's
// exercise. effective_at defaults to now if unset.
func (s *Service) RecordTrainingMax(ctx context.Context, req *connect.Request[v1.RecordTrainingMaxRequest]) (*connect.Response[v1.RecordTrainingMaxResponse], error) {
	userID := strings.TrimSpace(req.Msg.GetUserId())
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id must not be empty"))
	}
	exerciseID := strings.TrimSpace(req.Msg.GetExerciseId())
	if exerciseID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("exercise_id must not be empty"))
	}
	if req.Msg.GetWeight() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("weight must be greater than zero"))
	}
	unit, err := fromProtoUnit(req.Msg.GetUnit())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if _, err := s.users.Get(ctx, userID); err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	exercise, err := s.exercises.Get(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, storage.ErrExerciseNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if exercise.Archived {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("exercise is archived"))
	}

	effectiveAt := time.Now().UTC()
	if req.Msg.GetEffectiveAt() != nil {
		effectiveAt = req.Msg.GetEffectiveAt().AsTime()
	}

	max, err := s.trainingMaxes.Record(ctx, userID, exercise, req.Msg.GetWeight(), unit, effectiveAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.RecordTrainingMaxResponse{TrainingMax: toProto(max)}), nil
}

func toProto(max storage.TrainingMax) *v1.TrainingMax {
	return &v1.TrainingMax{
		Id:           max.ID,
		UserId:       max.UserID,
		ExerciseId:   max.ExerciseID,
		ExerciseName: max.ExerciseName,
		Weight:       max.Weight,
		Unit:         toProtoUnit(max.Unit),
		EffectiveAt:  timestamppb.New(max.EffectiveAt),
		CreatedAt:    timestamppb.New(max.CreatedAt),
	}
}

func toProtoUnit(unit storage.WeightUnit) v1.WeightUnit {
	switch unit {
	case storage.WeightUnitKg:
		return v1.WeightUnit_WEIGHT_UNIT_KG
	case storage.WeightUnitLb:
		return v1.WeightUnit_WEIGHT_UNIT_LB
	default:
		return v1.WeightUnit_WEIGHT_UNIT_UNSPECIFIED
	}
}

func fromProtoUnit(unit v1.WeightUnit) (storage.WeightUnit, error) {
	switch unit {
	case v1.WeightUnit_WEIGHT_UNIT_KG:
		return storage.WeightUnitKg, nil
	case v1.WeightUnit_WEIGHT_UNIT_LB:
		return storage.WeightUnitLb, nil
	default:
		return "", errors.New("unit must be KG or LB")
	}
}
