// Package cycleservice implements the workouts.v1.CycleService Connect
// handler over the storage.Cycles/Blocks/CycleExercises/ExerciseSets
// repositories.
package cycleservice

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/workouts/gen/workouts/v1"
	"github.com/liamawhite/workouts/internal/storage"
)

// Service implements workoutsv1connect.CycleServiceHandler.
type Service struct {
	cycles         *storage.Cycles
	blocks         *storage.Blocks
	cycleExercises *storage.CycleExercises
	exerciseSets   *storage.ExerciseSets
	users          *storage.Users
	exercises      *storage.Exercises
}

// New returns a Service backed by the given repositories - users and
// exercises are needed to validate the user_id/exercise_id referenced when
// creating cycles and adding cycle exercises.
func New(cycles *storage.Cycles, blocks *storage.Blocks, cycleExercises *storage.CycleExercises, exerciseSets *storage.ExerciseSets, users *storage.Users, exercises *storage.Exercises) *Service {
	return &Service{
		cycles:         cycles,
		blocks:         blocks,
		cycleExercises: cycleExercises,
		exerciseSets:   exerciseSets,
		users:          users,
		exercises:      exercises,
	}
}

// ListCycles returns every cycle belonging to the given user, as summaries
// (blocks/cycle_exercises/exercise_sets left empty - see GetCycle for the
// full tree).
func (s *Service) ListCycles(ctx context.Context, req *connect.Request[v1.ListCyclesRequest]) (*connect.Response[v1.ListCyclesResponse], error) {
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

	cycles, err := s.cycles.ListByUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListCyclesResponse{Cycles: make([]*v1.Cycle, 0, len(cycles))}
	for _, cycle := range cycles {
		resp.Cycles = append(resp.Cycles, toProtoCycleSummary(cycle))
	}

	return connect.NewResponse(resp), nil
}

// GetCycle returns the full cycle tree: the cycle itself plus flat sibling
// lists of every block, cycle exercise, and exercise set - the frontend
// reassembles the grid by joining on cycle_exercise_id/block_id.
func (s *Service) GetCycle(ctx context.Context, req *connect.Request[v1.GetCycleRequest]) (*connect.Response[v1.GetCycleResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	cycle, err := s.cycles.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrCycleNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	blocks, err := s.blocks.ListByCycle(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cycleExercises, err := s.cycleExercises.ListByCycle(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exerciseSets, err := s.exerciseSets.ListByCycle(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.GetCycleResponse{
		Cycle: toProtoCycleFull(cycle, blocks, cycleExercises, exerciseSets),
	}), nil
}

// CreateCycle creates a new cycle for the given user.
func (s *Service) CreateCycle(ctx context.Context, req *connect.Request[v1.CreateCycleRequest]) (*connect.Response[v1.CreateCycleResponse], error) {
	userID := strings.TrimSpace(req.Msg.GetUserId())
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id must not be empty"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	if _, err := s.users.Get(ctx, userID); err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	cycle, err := s.cycles.Create(ctx, userID, name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateCycleResponse{Cycle: toProtoCycleSummary(cycle)}), nil
}

// DeleteCycle removes a cycle, cascading to its blocks, cycle exercises,
// and exercise sets.
func (s *Service) DeleteCycle(ctx context.Context, req *connect.Request[v1.DeleteCycleRequest]) (*connect.Response[v1.DeleteCycleResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.cycles.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrCycleNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteCycleResponse{}), nil
}

// DuplicateCycle deep-copies a source cycle's blocks, shared exercise
// lineup, and every prescribed set into a brand new cycle for the same
// user. Cycle exercises are copied even if their underlying exercise is
// now archived - duplication preserves the source cycle's structure as-is,
// unlike AddCycleExercise which rejects archived exercises for new work.
// This isn't wrapped in a single DB transaction (each storage call commits
// its own append), so a failure partway through leaves a partially-copied
// cycle rather than rolling back entirely - acceptable for this app's
// scale, but worth knowing if this ever needs to be bulletproof.
func (s *Service) DuplicateCycle(ctx context.Context, req *connect.Request[v1.DuplicateCycleRequest]) (*connect.Response[v1.DuplicateCycleResponse], error) {
	sourceCycleID := strings.TrimSpace(req.Msg.GetSourceCycleId())
	if sourceCycleID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_cycle_id must not be empty"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	source, err := s.cycles.Get(ctx, sourceCycleID)
	if err != nil {
		if errors.Is(err, storage.ErrCycleNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	blocks, err := s.blocks.ListByCycle(ctx, sourceCycleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	cycleExercises, err := s.cycleExercises.ListByCycle(ctx, sourceCycleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	exerciseSets, err := s.exerciseSets.ListByCycle(ctx, sourceCycleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	newCycle, err := s.cycles.Create(ctx, source.UserID, name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Each list below is already ordered by position (see the corresponding
	// ListByCycle query), and every Create/Add call appends to the end of
	// the new cycle's own sequence - replaying them in source order
	// reproduces the same relative order without needing to copy position
	// values explicitly.
	blockIDs := make(map[string]string, len(blocks))
	for _, block := range blocks {
		newBlock, err := s.blocks.Create(ctx, newCycle.ID, block.Name)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		blockIDs[block.ID] = newBlock.ID
	}

	cycleExerciseIDs := make(map[string]string, len(cycleExercises))
	for _, ce := range cycleExercises {
		exercise, err := s.exercises.Get(ctx, ce.ExerciseID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		newCE, err := s.cycleExercises.Add(ctx, newCycle.ID, exercise)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		cycleExerciseIDs[ce.ID] = newCE.ID
	}

	for _, set := range exerciseSets {
		newCEID, ok := cycleExerciseIDs[set.CycleExerciseID]
		if !ok {
			continue
		}
		newBlockID, ok := blockIDs[set.BlockID]
		if !ok {
			continue
		}
		if _, err := s.exerciseSets.Add(ctx, newCEID, newBlockID, set.Reps, set.PercentageOfTM); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&v1.DuplicateCycleResponse{Cycle: toProtoCycleSummary(newCycle)}), nil
}

// CreateBlock appends a new block to the end of a cycle's block list.
func (s *Service) CreateBlock(ctx context.Context, req *connect.Request[v1.CreateBlockRequest]) (*connect.Response[v1.CreateBlockResponse], error) {
	cycleID := strings.TrimSpace(req.Msg.GetCycleId())
	if cycleID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cycle_id must not be empty"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name must not be empty"))
	}

	if _, err := s.cycles.Get(ctx, cycleID); err != nil {
		if errors.Is(err, storage.ErrCycleNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	block, err := s.blocks.Create(ctx, cycleID, name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateBlockResponse{Block: toProtoBlock(block)}), nil
}

// DeleteBlock removes a block, cascading to its exercise sets.
func (s *Service) DeleteBlock(ctx context.Context, req *connect.Request[v1.DeleteBlockRequest]) (*connect.Response[v1.DeleteBlockResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.blocks.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrBlockNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteBlockResponse{}), nil
}

// AddCycleExercise appends an exercise to the end of a cycle's shared
// exercise lineup.
func (s *Service) AddCycleExercise(ctx context.Context, req *connect.Request[v1.AddCycleExerciseRequest]) (*connect.Response[v1.AddCycleExerciseResponse], error) {
	cycleID := strings.TrimSpace(req.Msg.GetCycleId())
	if cycleID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cycle_id must not be empty"))
	}
	exerciseID := strings.TrimSpace(req.Msg.GetExerciseId())
	if exerciseID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("exercise_id must not be empty"))
	}

	if _, err := s.cycles.Get(ctx, cycleID); err != nil {
		if errors.Is(err, storage.ErrCycleNotFound) {
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

	cycleExercise, err := s.cycleExercises.Add(ctx, cycleID, exercise)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.AddCycleExerciseResponse{CycleExercise: toProtoCycleExercise(cycleExercise)}), nil
}

// RemoveCycleExercise removes a cycle exercise, cascading to its exercise
// sets across every block.
func (s *Service) RemoveCycleExercise(ctx context.Context, req *connect.Request[v1.RemoveCycleExerciseRequest]) (*connect.Response[v1.RemoveCycleExerciseResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.cycleExercises.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrCycleExerciseNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.RemoveCycleExerciseResponse{}), nil
}

// MoveCycleExercise swaps a cycle exercise's position with its nearest
// same-category neighbor.
func (s *Service) MoveCycleExercise(ctx context.Context, req *connect.Request[v1.MoveCycleExerciseRequest]) (*connect.Response[v1.MoveCycleExerciseResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	up, err := directionUp(req.Msg.GetDirection())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.cycleExercises.Move(ctx, id, up); err != nil {
		if errors.Is(err, storage.ErrCycleExerciseNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.MoveCycleExerciseResponse{}), nil
}

// AddSet appends a new prescribed set to a (cycle_exercise, block) cell.
func (s *Service) AddSet(ctx context.Context, req *connect.Request[v1.AddSetRequest]) (*connect.Response[v1.AddSetResponse], error) {
	cycleExerciseID := strings.TrimSpace(req.Msg.GetCycleExerciseId())
	if cycleExerciseID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cycle_exercise_id must not be empty"))
	}
	blockID := strings.TrimSpace(req.Msg.GetBlockId())
	if blockID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("block_id must not be empty"))
	}
	if req.Msg.GetReps() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reps must be greater than zero"))
	}
	if req.Msg.PercentageOfTm != nil && req.Msg.GetPercentageOfTm() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("percentage_of_tm must be greater than zero"))
	}

	cycleExercise, err := s.cycleExercises.Get(ctx, cycleExerciseID)
	if err != nil {
		if errors.Is(err, storage.ErrCycleExerciseNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	block, err := s.blocks.Get(ctx, blockID)
	if err != nil {
		if errors.Is(err, storage.ErrBlockNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if block.CycleID != cycleExercise.CycleID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("block does not belong to this cycle exercise's cycle"))
	}

	set, err := s.exerciseSets.Add(ctx, cycleExerciseID, blockID, req.Msg.GetReps(), req.Msg.PercentageOfTm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.AddSetResponse{ExerciseSet: toProtoExerciseSet(set)}), nil
}

// UpdateSet replaces the reps/percentage of an existing prescribed set.
func (s *Service) UpdateSet(ctx context.Context, req *connect.Request[v1.UpdateSetRequest]) (*connect.Response[v1.UpdateSetResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}
	if req.Msg.GetReps() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reps must be greater than zero"))
	}
	if req.Msg.PercentageOfTm != nil && req.Msg.GetPercentageOfTm() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("percentage_of_tm must be greater than zero"))
	}

	set, err := s.exerciseSets.Update(ctx, id, req.Msg.GetReps(), req.Msg.PercentageOfTm)
	if err != nil {
		if errors.Is(err, storage.ErrExerciseSetNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateSetResponse{ExerciseSet: toProtoExerciseSet(set)}), nil
}

// RemoveSet removes a prescribed set.
func (s *Service) RemoveSet(ctx context.Context, req *connect.Request[v1.RemoveSetRequest]) (*connect.Response[v1.RemoveSetResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.exerciseSets.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrExerciseSetNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.RemoveSetResponse{}), nil
}

// MoveSet swaps a set's position with its nearest neighbor within the same
// (cycle_exercise, block) cell.
func (s *Service) MoveSet(ctx context.Context, req *connect.Request[v1.MoveSetRequest]) (*connect.Response[v1.MoveSetResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	up, err := directionUp(req.Msg.GetDirection())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.exerciseSets.Move(ctx, id, up); err != nil {
		if errors.Is(err, storage.ErrExerciseSetNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.MoveSetResponse{}), nil
}

func directionUp(direction v1.MoveDirection) (bool, error) {
	switch direction {
	case v1.MoveDirection_MOVE_DIRECTION_UP:
		return true, nil
	case v1.MoveDirection_MOVE_DIRECTION_DOWN:
		return false, nil
	default:
		return false, errors.New("direction must be UP or DOWN")
	}
}

func toProtoCycleSummary(cycle storage.Cycle) *v1.Cycle {
	return &v1.Cycle{
		Id:        cycle.ID,
		UserId:    cycle.UserID,
		Name:      cycle.Name,
		CreatedAt: timestamppb.New(cycle.CreatedAt),
	}
}

func toProtoCycleFull(cycle storage.Cycle, blocks []storage.Block, cycleExercises []storage.CycleExercise, exerciseSets []storage.ExerciseSet) *v1.Cycle {
	proto := toProtoCycleSummary(cycle)

	proto.Blocks = make([]*v1.Block, 0, len(blocks))
	for _, block := range blocks {
		proto.Blocks = append(proto.Blocks, toProtoBlock(block))
	}

	proto.CycleExercises = make([]*v1.CycleExercise, 0, len(cycleExercises))
	for _, cycleExercise := range cycleExercises {
		proto.CycleExercises = append(proto.CycleExercises, toProtoCycleExercise(cycleExercise))
	}

	proto.ExerciseSets = make([]*v1.ExerciseSet, 0, len(exerciseSets))
	for _, set := range exerciseSets {
		proto.ExerciseSets = append(proto.ExerciseSets, toProtoExerciseSet(set))
	}

	return proto
}

func toProtoBlock(block storage.Block) *v1.Block {
	return &v1.Block{
		Id:        block.ID,
		CycleId:   block.CycleID,
		Name:      block.Name,
		Position:  block.Position,
		CreatedAt: timestamppb.New(block.CreatedAt),
	}
}

func toProtoCycleExercise(cycleExercise storage.CycleExercise) *v1.CycleExercise {
	return &v1.CycleExercise{
		Id:                cycleExercise.ID,
		CycleId:           cycleExercise.CycleID,
		ExerciseId:        cycleExercise.ExerciseID,
		ExerciseName:      cycleExercise.ExerciseName,
		ExerciseCategory:  toProtoCategory(cycleExercise.ExerciseCategory),
		ExerciseEquipment: toProtoEquipment(cycleExercise.ExerciseEquipment),
		Position:          cycleExercise.Position,
		CreatedAt:         timestamppb.New(cycleExercise.CreatedAt),
	}
}

func toProtoExerciseSet(set storage.ExerciseSet) *v1.ExerciseSet {
	return &v1.ExerciseSet{
		Id:              set.ID,
		CycleExerciseId: set.CycleExerciseID,
		BlockId:         set.BlockID,
		Position:        set.Position,
		Reps:            set.Reps,
		PercentageOfTm:  set.PercentageOfTM,
		CreatedAt:       timestamppb.New(set.CreatedAt),
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
