package cycleservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/workouts/gen/workouts/v1"
	"github.com/liamawhite/workouts/internal/storage"
)

type testFixture struct {
	svc                 *Service
	exercises           *storage.Exercises
	userID              string
	mainExerciseID      string
	accessoryExerciseID string
}

func newTestFixture(t *testing.T) testFixture {
	t.Helper()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("storage.Migrate() error = %v", err)
	}

	users := storage.NewUsers(db)
	exercises := storage.NewExercises(db)
	cycles := storage.NewCycles(db)
	blocks := storage.NewBlocks(db)
	cycleExercises := storage.NewCycleExercises(db)
	exerciseSets := storage.NewExerciseSets(db)

	ctx := context.Background()
	user, err := users.Create(ctx, "Liam")
	if err != nil {
		t.Fatalf("users.Create() error = %v", err)
	}
	mainExercise, err := exercises.Create(ctx, "Squat", storage.ExerciseCategoryMainLift, storage.ExerciseEquipmentBarbell)
	if err != nil {
		t.Fatalf("exercises.Create(Squat) error = %v", err)
	}
	accessoryExercise, err := exercises.Create(ctx, "Face Pull", storage.ExerciseCategoryAccessory, storage.ExerciseEquipmentDumbbell)
	if err != nil {
		t.Fatalf("exercises.Create(Face Pull) error = %v", err)
	}

	return testFixture{
		svc:                 New(cycles, blocks, cycleExercises, exerciseSets, users, exercises),
		exercises:           exercises,
		userID:              user.ID,
		mainExerciseID:      mainExercise.ID,
		accessoryExerciseID: accessoryExercise.ID,
	}
}

func percentagePtr(v float64) *float64 { return &v }

func TestService_CreateCycle_ThenListCycles(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	created, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "531"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}
	if created.Msg.GetCycle().GetId() == "" {
		t.Fatal("CreateCycle() returned empty cycle id")
	}

	listed, err := f.svc.ListCycles(ctx, connect.NewRequest(&v1.ListCyclesRequest{UserId: f.userID}))
	if err != nil {
		t.Fatalf("ListCycles() error = %v", err)
	}
	if len(listed.Msg.GetCycles()) != 1 {
		t.Fatalf("ListCycles() = %d, want 1", len(listed.Msg.GetCycles()))
	}
	if len(listed.Msg.GetCycles()[0].GetBlocks()) != 0 {
		t.Fatal("ListCycles() summary should not include blocks")
	}
}

func TestService_CreateCycle_RejectsBadUser(t *testing.T) {
	f := newTestFixture(t)

	_, err := f.svc.CreateCycle(context.Background(), connect.NewRequest(&v1.CreateCycleRequest{UserId: "does-not-exist", Name: "531"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("CreateCycle() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DeleteCycle_Cascades(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	cycle, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "531"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}
	cycleID := cycle.Msg.GetCycle().GetId()

	block, err := f.svc.CreateBlock(ctx, connect.NewRequest(&v1.CreateBlockRequest{CycleId: cycleID, Name: "Week 1"}))
	if err != nil {
		t.Fatalf("CreateBlock() error = %v", err)
	}
	cycleExercise, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: cycleID, ExerciseId: f.mainExerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise() error = %v", err)
	}
	if _, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{
		CycleExerciseId: cycleExercise.Msg.GetCycleExercise().GetId(),
		BlockId:         block.Msg.GetBlock().GetId(),
		Reps:            5,
	})); err != nil {
		t.Fatalf("AddSet() error = %v", err)
	}

	if _, err := f.svc.DeleteCycle(ctx, connect.NewRequest(&v1.DeleteCycleRequest{Id: cycleID})); err != nil {
		t.Fatalf("DeleteCycle() error = %v", err)
	}

	if _, err := f.svc.GetCycle(ctx, connect.NewRequest(&v1.GetCycleRequest{Id: cycleID})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetCycle() after delete code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DuplicateCycle_CopiesStructure(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	source, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "531"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}
	sourceID := source.Msg.GetCycle().GetId()

	block1, err := f.svc.CreateBlock(ctx, connect.NewRequest(&v1.CreateBlockRequest{CycleId: sourceID, Name: "Week 1"}))
	if err != nil {
		t.Fatalf("CreateBlock(Week 1) error = %v", err)
	}
	block2, err := f.svc.CreateBlock(ctx, connect.NewRequest(&v1.CreateBlockRequest{CycleId: sourceID, Name: "Week 2"}))
	if err != nil {
		t.Fatalf("CreateBlock(Week 2) error = %v", err)
	}
	mainCE, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: sourceID, ExerciseId: f.mainExerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise(main) error = %v", err)
	}
	accessoryCE, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: sourceID, ExerciseId: f.accessoryExerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise(accessory) error = %v", err)
	}
	for _, blockID := range []string{block1.Msg.GetBlock().GetId(), block2.Msg.GetBlock().GetId()} {
		if _, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{
			CycleExerciseId: mainCE.Msg.GetCycleExercise().GetId(), BlockId: blockID, Reps: 5, PercentageOfTm: percentagePtr(75),
		})); err != nil {
			t.Fatalf("AddSet(main) error = %v", err)
		}
	}
	if _, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{
		CycleExerciseId: accessoryCE.Msg.GetCycleExercise().GetId(), BlockId: block1.Msg.GetBlock().GetId(), Reps: 12,
	})); err != nil {
		t.Fatalf("AddSet(accessory) error = %v", err)
	}

	// Archive the accessory exercise before duplicating - duplication
	// should still copy it, unlike AddCycleExercise which would reject it.
	if err := f.exercises.Archive(ctx, f.accessoryExerciseID); err != nil {
		t.Fatalf("exercises.Archive() error = %v", err)
	}

	dup, err := f.svc.DuplicateCycle(ctx, connect.NewRequest(&v1.DuplicateCycleRequest{SourceCycleId: sourceID, Name: "531 copy"}))
	if err != nil {
		t.Fatalf("DuplicateCycle() error = %v", err)
	}
	newID := dup.Msg.GetCycle().GetId()
	if newID == sourceID {
		t.Fatal("DuplicateCycle() returned the source cycle's id")
	}
	if dup.Msg.GetCycle().GetName() != "531 copy" {
		t.Fatalf("DuplicateCycle() name = %q, want %q", dup.Msg.GetCycle().GetName(), "531 copy")
	}

	got, err := f.svc.GetCycle(ctx, connect.NewRequest(&v1.GetCycleRequest{Id: newID}))
	if err != nil {
		t.Fatalf("GetCycle(duplicate) error = %v", err)
	}
	newCycle := got.Msg.GetCycle()
	if len(newCycle.GetBlocks()) != 2 {
		t.Fatalf("duplicate blocks = %d, want 2", len(newCycle.GetBlocks()))
	}
	if len(newCycle.GetCycleExercises()) != 2 {
		t.Fatalf("duplicate cycle_exercises = %d, want 2 (including the now-archived accessory)", len(newCycle.GetCycleExercises()))
	}
	if len(newCycle.GetExerciseSets()) != 3 {
		t.Fatalf("duplicate exercise_sets = %d, want 3", len(newCycle.GetExerciseSets()))
	}

	// Original cycle must be untouched.
	original, err := f.svc.GetCycle(ctx, connect.NewRequest(&v1.GetCycleRequest{Id: sourceID}))
	if err != nil {
		t.Fatalf("GetCycle(source) error = %v", err)
	}
	if len(original.Msg.GetCycle().GetExerciseSets()) != 3 {
		t.Fatalf("source exercise_sets after duplicate = %d, want 3 (unchanged)", len(original.Msg.GetCycle().GetExerciseSets()))
	}
}

func TestService_GetCycle_AssemblesFullTree(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	cycle, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "531"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}
	cycleID := cycle.Msg.GetCycle().GetId()

	block1, err := f.svc.CreateBlock(ctx, connect.NewRequest(&v1.CreateBlockRequest{CycleId: cycleID, Name: "Week 1"}))
	if err != nil {
		t.Fatalf("CreateBlock(Week 1) error = %v", err)
	}
	block2, err := f.svc.CreateBlock(ctx, connect.NewRequest(&v1.CreateBlockRequest{CycleId: cycleID, Name: "Week 2"}))
	if err != nil {
		t.Fatalf("CreateBlock(Week 2) error = %v", err)
	}

	mainCE, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: cycleID, ExerciseId: f.mainExerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise(main) error = %v", err)
	}
	accessoryCE, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: cycleID, ExerciseId: f.accessoryExerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise(accessory) error = %v", err)
	}

	for _, blockID := range []string{block1.Msg.GetBlock().GetId(), block2.Msg.GetBlock().GetId()} {
		for _, ceID := range []string{mainCE.Msg.GetCycleExercise().GetId(), accessoryCE.Msg.GetCycleExercise().GetId()} {
			if _, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{
				CycleExerciseId: ceID, BlockId: blockID, Reps: 5, PercentageOfTm: percentagePtr(75),
			})); err != nil {
				t.Fatalf("AddSet() error = %v", err)
			}
		}
	}

	got, err := f.svc.GetCycle(ctx, connect.NewRequest(&v1.GetCycleRequest{Id: cycleID}))
	if err != nil {
		t.Fatalf("GetCycle() error = %v", err)
	}
	if len(got.Msg.GetCycle().GetBlocks()) != 2 {
		t.Fatalf("GetCycle() blocks = %d, want 2", len(got.Msg.GetCycle().GetBlocks()))
	}
	if len(got.Msg.GetCycle().GetCycleExercises()) != 2 {
		t.Fatalf("GetCycle() cycle_exercises = %d, want 2", len(got.Msg.GetCycle().GetCycleExercises()))
	}
	if len(got.Msg.GetCycle().GetExerciseSets()) != 4 {
		t.Fatalf("GetCycle() exercise_sets = %d, want 4", len(got.Msg.GetCycle().GetExerciseSets()))
	}
	for _, set := range got.Msg.GetCycle().GetExerciseSets() {
		if set.GetBlockId() != block1.Msg.GetBlock().GetId() && set.GetBlockId() != block2.Msg.GetBlock().GetId() {
			t.Fatalf("exercise set has unexpected block_id %q", set.GetBlockId())
		}
		if set.GetCycleExerciseId() != mainCE.Msg.GetCycleExercise().GetId() && set.GetCycleExerciseId() != accessoryCE.Msg.GetCycleExercise().GetId() {
			t.Fatalf("exercise set has unexpected cycle_exercise_id %q", set.GetCycleExerciseId())
		}
	}
}

func TestService_AddSet_RejectsNonPositiveReps(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	_, blockID, ceID := setupCycleBlockExercise(t, f, f.mainExerciseID)

	_, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{CycleExerciseId: ceID, BlockId: blockID, Reps: 0}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("AddSet() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_AddSet_RejectsNonPositivePercentage(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	_, blockID, ceID := setupCycleBlockExercise(t, f, f.mainExerciseID)

	_, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{
		CycleExerciseId: ceID, BlockId: blockID, Reps: 5, PercentageOfTm: percentagePtr(0),
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("AddSet() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_AddSet_AllowsNilPercentage(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	_, blockID, ceID := setupCycleBlockExercise(t, f, f.mainExerciseID)

	resp, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{CycleExerciseId: ceID, BlockId: blockID, Reps: 8}))
	if err != nil {
		t.Fatalf("AddSet() error = %v", err)
	}
	if resp.Msg.GetExerciseSet().PercentageOfTm != nil {
		t.Fatalf("AddSet() percentage_of_tm = %v, want nil", resp.Msg.GetExerciseSet().PercentageOfTm)
	}
}

func TestService_AddSet_RejectsBlockFromDifferentCycle(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	_, _, ceID := setupCycleBlockExercise(t, f, f.mainExerciseID)

	otherCycle, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "Other"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}
	otherBlock, err := f.svc.CreateBlock(ctx, connect.NewRequest(&v1.CreateBlockRequest{CycleId: otherCycle.Msg.GetCycle().GetId(), Name: "Week 1"}))
	if err != nil {
		t.Fatalf("CreateBlock() error = %v", err)
	}

	_, err = f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{
		CycleExerciseId: ceID, BlockId: otherBlock.Msg.GetBlock().GetId(), Reps: 5,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("AddSet() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_AddCycleExercise_RejectsArchivedExercise(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	cycle, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "531"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}

	if err := f.exercises.Archive(ctx, f.mainExerciseID); err != nil {
		t.Fatalf("exercises.Archive() error = %v", err)
	}

	_, err = f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{
		CycleId: cycle.Msg.GetCycle().GetId(), ExerciseId: f.mainExerciseID,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("AddCycleExercise() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_MoveCycleExercise_OnlySwapsWithinSameCategory(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	cycle, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "531"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}
	cycleID := cycle.Msg.GetCycle().GetId()

	mainCE, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: cycleID, ExerciseId: f.mainExerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise(main) error = %v", err)
	}
	accessoryCE, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: cycleID, ExerciseId: f.accessoryExerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise(accessory) error = %v", err)
	}

	// Moving the accessory exercise up should be a no-op (it's alone in its
	// category group), not swap with the main-lift exercise even though
	// that's the immediately preceding position overall.
	if _, err := f.svc.MoveCycleExercise(ctx, connect.NewRequest(&v1.MoveCycleExerciseRequest{
		Id: accessoryCE.Msg.GetCycleExercise().GetId(), Direction: v1.MoveDirection_MOVE_DIRECTION_UP,
	})); err != nil {
		t.Fatalf("MoveCycleExercise() error = %v", err)
	}

	got, err := f.svc.GetCycle(ctx, connect.NewRequest(&v1.GetCycleRequest{Id: cycleID}))
	if err != nil {
		t.Fatalf("GetCycle() error = %v", err)
	}
	positions := map[string]int64{}
	for _, ce := range got.Msg.GetCycle().GetCycleExercises() {
		positions[ce.GetId()] = ce.GetPosition()
	}
	if positions[mainCE.Msg.GetCycleExercise().GetId()] >= positions[accessoryCE.Msg.GetCycleExercise().GetId()] {
		t.Fatal("MoveCycleExercise() swapped positions across category groups, want no-op")
	}
}

func TestService_MoveCycleExercise_SwapsWithinCategory(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	secondMain, err := f.exercises.Create(ctx, "Bench", storage.ExerciseCategoryMainLift, storage.ExerciseEquipmentBarbell)
	if err != nil {
		t.Fatalf("exercises.Create(Bench) error = %v", err)
	}

	cycle, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "531"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}
	cycleID := cycle.Msg.GetCycle().GetId()

	first, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: cycleID, ExerciseId: f.mainExerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise(Squat) error = %v", err)
	}
	second, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: cycleID, ExerciseId: secondMain.ID}))
	if err != nil {
		t.Fatalf("AddCycleExercise(Bench) error = %v", err)
	}

	if _, err := f.svc.MoveCycleExercise(ctx, connect.NewRequest(&v1.MoveCycleExerciseRequest{
		Id: second.Msg.GetCycleExercise().GetId(), Direction: v1.MoveDirection_MOVE_DIRECTION_UP,
	})); err != nil {
		t.Fatalf("MoveCycleExercise() error = %v", err)
	}

	got, err := f.svc.GetCycle(ctx, connect.NewRequest(&v1.GetCycleRequest{Id: cycleID}))
	if err != nil {
		t.Fatalf("GetCycle() error = %v", err)
	}
	positions := map[string]int64{}
	for _, ce := range got.Msg.GetCycle().GetCycleExercises() {
		positions[ce.GetId()] = ce.GetPosition()
	}
	if positions[second.Msg.GetCycleExercise().GetId()] >= positions[first.Msg.GetCycleExercise().GetId()] {
		t.Fatal("MoveCycleExercise(up) did not swap positions within the same category")
	}

	// Moving the now-topmost item up again is a no-op, not an error.
	if _, err := f.svc.MoveCycleExercise(ctx, connect.NewRequest(&v1.MoveCycleExerciseRequest{
		Id: second.Msg.GetCycleExercise().GetId(), Direction: v1.MoveDirection_MOVE_DIRECTION_UP,
	})); err != nil {
		t.Fatalf("MoveCycleExercise() at boundary error = %v, want nil (no-op)", err)
	}
}

func TestService_MoveSet_SwapsWithinCell(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	cycleID, blockID, ceID := setupCycleBlockExercise(t, f, f.mainExerciseID)

	first, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{CycleExerciseId: ceID, BlockId: blockID, Reps: 5, PercentageOfTm: percentagePtr(70)}))
	if err != nil {
		t.Fatalf("AddSet(1) error = %v", err)
	}
	second, err := f.svc.AddSet(ctx, connect.NewRequest(&v1.AddSetRequest{CycleExerciseId: ceID, BlockId: blockID, Reps: 3, PercentageOfTm: percentagePtr(80)}))
	if err != nil {
		t.Fatalf("AddSet(2) error = %v", err)
	}

	if _, err := f.svc.MoveSet(ctx, connect.NewRequest(&v1.MoveSetRequest{
		Id: second.Msg.GetExerciseSet().GetId(), Direction: v1.MoveDirection_MOVE_DIRECTION_UP,
	})); err != nil {
		t.Fatalf("MoveSet() error = %v", err)
	}

	history, err := f.svc.GetCycle(ctx, connect.NewRequest(&v1.GetCycleRequest{Id: cycleID}))
	if err != nil {
		t.Fatalf("GetCycle() error = %v", err)
	}
	positions := map[string]int64{}
	for _, s := range history.Msg.GetCycle().GetExerciseSets() {
		positions[s.GetId()] = s.GetPosition()
	}
	if positions[second.Msg.GetExerciseSet().GetId()] >= positions[first.Msg.GetExerciseSet().GetId()] {
		t.Fatal("MoveSet(up) did not swap positions within the cell")
	}

	// Boundary: moving the now-first set up again is a no-op.
	if _, err := f.svc.MoveSet(ctx, connect.NewRequest(&v1.MoveSetRequest{
		Id: second.Msg.GetExerciseSet().GetId(), Direction: v1.MoveDirection_MOVE_DIRECTION_UP,
	})); err != nil {
		t.Fatalf("MoveSet() at boundary error = %v, want nil (no-op)", err)
	}
}

// setupCycleBlockExercise creates a cycle with one block and one cycle
// exercise (for exerciseID), returning their IDs.
func setupCycleBlockExercise(t *testing.T, f testFixture, exerciseID string) (cycleID, blockID, cycleExerciseID string) {
	t.Helper()
	ctx := context.Background()

	cycle, err := f.svc.CreateCycle(ctx, connect.NewRequest(&v1.CreateCycleRequest{UserId: f.userID, Name: "531"}))
	if err != nil {
		t.Fatalf("CreateCycle() error = %v", err)
	}
	block, err := f.svc.CreateBlock(ctx, connect.NewRequest(&v1.CreateBlockRequest{CycleId: cycle.Msg.GetCycle().GetId(), Name: "Week 1"}))
	if err != nil {
		t.Fatalf("CreateBlock() error = %v", err)
	}
	ce, err := f.svc.AddCycleExercise(ctx, connect.NewRequest(&v1.AddCycleExerciseRequest{CycleId: cycle.Msg.GetCycle().GetId(), ExerciseId: exerciseID}))
	if err != nil {
		t.Fatalf("AddCycleExercise() error = %v", err)
	}

	return cycle.Msg.GetCycle().GetId(), block.Msg.GetBlock().GetId(), ce.Msg.GetCycleExercise().GetId()
}
