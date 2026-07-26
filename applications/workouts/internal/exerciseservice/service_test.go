package exerciseservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/workouts/gen/workouts/v1"
	"github.com/liamawhite/workouts/internal/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("storage.Migrate() error = %v", err)
	}

	return New(storage.NewExercises(db))
}

func TestService_CreateExercise_RejectsEmptyName(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateExercise(context.Background(), connect.NewRequest(&v1.CreateExerciseRequest{
		Name:     "   ",
		Category: v1.ExerciseCategory_EXERCISE_CATEGORY_MAIN_LIFT,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateExercise() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateExercise_RejectsUnspecifiedCategory(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateExercise(context.Background(), connect.NewRequest(&v1.CreateExerciseRequest{Name: "Squat"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateExercise() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateExercise_ThenListExercises(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateExercise(ctx, connect.NewRequest(&v1.CreateExerciseRequest{
		Name:     "Squat",
		Category: v1.ExerciseCategory_EXERCISE_CATEGORY_MAIN_LIFT,
	}))
	if err != nil {
		t.Fatalf("CreateExercise() error = %v", err)
	}
	if created.Msg.GetExercise().GetId() == "" {
		t.Fatal("CreateExercise() returned empty exercise id")
	}
	if created.Msg.GetExercise().GetCategory() != v1.ExerciseCategory_EXERCISE_CATEGORY_MAIN_LIFT {
		t.Fatalf("CreateExercise() category = %v, want MAIN_LIFT", created.Msg.GetExercise().GetCategory())
	}
	if created.Msg.GetExercise().GetArchived() {
		t.Fatal("CreateExercise() returned an archived exercise")
	}

	listed, err := svc.ListExercises(ctx, connect.NewRequest(&v1.ListExercisesRequest{}))
	if err != nil {
		t.Fatalf("ListExercises() error = %v", err)
	}
	if len(listed.Msg.GetExercises()) != 1 {
		t.Fatalf("ListExercises() = %d exercises, want 1", len(listed.Msg.GetExercises()))
	}
}

func TestService_ArchiveExercise_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.ArchiveExercise(context.Background(), connect.NewRequest(&v1.ArchiveExerciseRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("ArchiveExercise() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_ArchiveExercise_ThenRestore(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateExercise(ctx, connect.NewRequest(&v1.CreateExerciseRequest{
		Name:     "Squat",
		Category: v1.ExerciseCategory_EXERCISE_CATEGORY_MAIN_LIFT,
	}))
	if err != nil {
		t.Fatalf("CreateExercise() error = %v", err)
	}
	id := created.Msg.GetExercise().GetId()

	if _, err := svc.ArchiveExercise(ctx, connect.NewRequest(&v1.ArchiveExerciseRequest{Id: id})); err != nil {
		t.Fatalf("ArchiveExercise() error = %v", err)
	}

	listed, err := svc.ListExercises(ctx, connect.NewRequest(&v1.ListExercisesRequest{}))
	if err != nil {
		t.Fatalf("ListExercises() error = %v", err)
	}
	if !listed.Msg.GetExercises()[0].GetArchived() {
		t.Fatal("ListExercises() exercise not archived after Archive()")
	}

	if _, err := svc.RestoreExercise(ctx, connect.NewRequest(&v1.RestoreExerciseRequest{Id: id})); err != nil {
		t.Fatalf("RestoreExercise() error = %v", err)
	}

	listed, err = svc.ListExercises(ctx, connect.NewRequest(&v1.ListExercisesRequest{}))
	if err != nil {
		t.Fatalf("ListExercises() error = %v", err)
	}
	if listed.Msg.GetExercises()[0].GetArchived() {
		t.Fatal("ListExercises() exercise still archived after Restore()")
	}
}
