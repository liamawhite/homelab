package trainingmaxservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/workouts/gen/workouts/v1"
	"github.com/liamawhite/workouts/internal/storage"
)

type testFixture struct {
	svc        *Service
	userID     string
	exerciseID string
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
	trainingMaxes := storage.NewTrainingMaxes(db)

	ctx := context.Background()
	user, err := users.Create(ctx, "Liam")
	if err != nil {
		t.Fatalf("users.Create() error = %v", err)
	}
	exercise, err := exercises.Create(ctx, "Squat", storage.ExerciseCategoryMainLift)
	if err != nil {
		t.Fatalf("exercises.Create() error = %v", err)
	}

	return testFixture{
		svc:        New(trainingMaxes, users, exercises),
		userID:     user.ID,
		exerciseID: exercise.ID,
	}
}

func TestService_RecordTrainingMax_RejectsBadUser(t *testing.T) {
	f := newTestFixture(t)

	_, err := f.svc.RecordTrainingMax(context.Background(), connect.NewRequest(&v1.RecordTrainingMaxRequest{
		UserId:     "does-not-exist",
		ExerciseId: f.exerciseID,
		Weight:     100,
		Unit:       v1.WeightUnit_WEIGHT_UNIT_KG,
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("RecordTrainingMax() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_RecordTrainingMax_RejectsBadExercise(t *testing.T) {
	f := newTestFixture(t)

	_, err := f.svc.RecordTrainingMax(context.Background(), connect.NewRequest(&v1.RecordTrainingMaxRequest{
		UserId:     f.userID,
		ExerciseId: "does-not-exist",
		Weight:     100,
		Unit:       v1.WeightUnit_WEIGHT_UNIT_KG,
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("RecordTrainingMax() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_RecordTrainingMax_RejectsUnspecifiedUnit(t *testing.T) {
	f := newTestFixture(t)

	_, err := f.svc.RecordTrainingMax(context.Background(), connect.NewRequest(&v1.RecordTrainingMaxRequest{
		UserId:     f.userID,
		ExerciseId: f.exerciseID,
		Weight:     100,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("RecordTrainingMax() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_RecordTrainingMax_RejectsNonPositiveWeight(t *testing.T) {
	f := newTestFixture(t)

	_, err := f.svc.RecordTrainingMax(context.Background(), connect.NewRequest(&v1.RecordTrainingMaxRequest{
		UserId:     f.userID,
		ExerciseId: f.exerciseID,
		Weight:     0,
		Unit:       v1.WeightUnit_WEIGHT_UNIT_KG,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("RecordTrainingMax() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_RecordTrainingMax_RejectsArchivedExercise(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	if err := f.svc.exercises.Archive(ctx, f.exerciseID); err != nil {
		t.Fatalf("exercises.Archive() error = %v", err)
	}

	_, err := f.svc.RecordTrainingMax(ctx, connect.NewRequest(&v1.RecordTrainingMaxRequest{
		UserId:     f.userID,
		ExerciseId: f.exerciseID,
		Weight:     100,
		Unit:       v1.WeightUnit_WEIGHT_UNIT_KG,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("RecordTrainingMax() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_RecordTrainingMax_ThenListCurrent(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	recorded, err := f.svc.RecordTrainingMax(ctx, connect.NewRequest(&v1.RecordTrainingMaxRequest{
		UserId:     f.userID,
		ExerciseId: f.exerciseID,
		Weight:     100,
		Unit:       v1.WeightUnit_WEIGHT_UNIT_KG,
	}))
	if err != nil {
		t.Fatalf("RecordTrainingMax() error = %v", err)
	}
	if recorded.Msg.GetTrainingMax().GetExerciseName() != "Squat" {
		t.Fatalf("RecordTrainingMax() exercise_name = %q, want %q", recorded.Msg.GetTrainingMax().GetExerciseName(), "Squat")
	}

	current, err := f.svc.ListCurrentTrainingMaxes(ctx, connect.NewRequest(&v1.ListCurrentTrainingMaxesRequest{UserId: f.userID}))
	if err != nil {
		t.Fatalf("ListCurrentTrainingMaxes() error = %v", err)
	}
	if len(current.Msg.GetTrainingMaxes()) != 1 {
		t.Fatalf("ListCurrentTrainingMaxes() = %d, want 1", len(current.Msg.GetTrainingMaxes()))
	}
	if current.Msg.GetTrainingMaxes()[0].GetWeight() != 100 {
		t.Fatalf("ListCurrentTrainingMaxes()[0].Weight = %v, want 100", current.Msg.GetTrainingMaxes()[0].GetWeight())
	}
}

func TestService_RecordTrainingMax_TwiceLeavesOnlyLatestCurrent(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	for _, weight := range []float64{100, 105} {
		if _, err := f.svc.RecordTrainingMax(ctx, connect.NewRequest(&v1.RecordTrainingMaxRequest{
			UserId:     f.userID,
			ExerciseId: f.exerciseID,
			Weight:     weight,
			Unit:       v1.WeightUnit_WEIGHT_UNIT_KG,
		})); err != nil {
			t.Fatalf("RecordTrainingMax(%v) error = %v", weight, err)
		}
	}

	current, err := f.svc.ListCurrentTrainingMaxes(ctx, connect.NewRequest(&v1.ListCurrentTrainingMaxesRequest{UserId: f.userID}))
	if err != nil {
		t.Fatalf("ListCurrentTrainingMaxes() error = %v", err)
	}
	if len(current.Msg.GetTrainingMaxes()) != 1 {
		t.Fatalf("ListCurrentTrainingMaxes() = %d, want 1", len(current.Msg.GetTrainingMaxes()))
	}
	if current.Msg.GetTrainingMaxes()[0].GetWeight() != 105 {
		t.Fatalf("ListCurrentTrainingMaxes()[0].Weight = %v, want 105 (latest)", current.Msg.GetTrainingMaxes()[0].GetWeight())
	}

	history, err := f.svc.ListTrainingMaxHistory(ctx, connect.NewRequest(&v1.ListTrainingMaxHistoryRequest{
		UserId:     f.userID,
		ExerciseId: f.exerciseID,
	}))
	if err != nil {
		t.Fatalf("ListTrainingMaxHistory() error = %v", err)
	}
	if len(history.Msg.GetTrainingMaxes()) != 2 {
		t.Fatalf("ListTrainingMaxHistory() = %d, want 2", len(history.Msg.GetTrainingMaxes()))
	}
	if history.Msg.GetTrainingMaxes()[0].GetWeight() != 105 {
		t.Fatalf("ListTrainingMaxHistory()[0].Weight = %v, want 105 (newest first)", history.Msg.GetTrainingMaxes()[0].GetWeight())
	}
	if history.Msg.GetTrainingMaxes()[1].GetWeight() != 100 {
		t.Fatalf("ListTrainingMaxHistory()[1].Weight = %v, want 100", history.Msg.GetTrainingMaxes()[1].GetWeight())
	}
}
