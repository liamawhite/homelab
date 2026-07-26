package userservice

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

	return New(storage.NewUsers(db))
}

func TestService_CreateUser_RejectsEmptyName(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateUser(context.Background(), connect.NewRequest(&v1.CreateUserRequest{Name: "   "}))
	if err == nil {
		t.Fatal("CreateUser() with blank name error = nil, want CodeInvalidArgument")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateUser() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateUser_ThenListUsers(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateUser(ctx, connect.NewRequest(&v1.CreateUserRequest{Name: "Liam"}))
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.Msg.GetUser().GetId() == "" {
		t.Fatal("CreateUser() returned empty user id")
	}
	if created.Msg.GetUser().GetName() != "Liam" {
		t.Fatalf("CreateUser() name = %q, want %q", created.Msg.GetUser().GetName(), "Liam")
	}
	if created.Msg.GetUser().GetCreatedAt() == nil {
		t.Fatal("CreateUser() returned nil created_at")
	}

	listed, err := svc.ListUsers(ctx, connect.NewRequest(&v1.ListUsersRequest{}))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(listed.Msg.GetUsers()) != 1 {
		t.Fatalf("ListUsers() = %d users, want 1", len(listed.Msg.GetUsers()))
	}
	if listed.Msg.GetUsers()[0].GetId() != created.Msg.GetUser().GetId() {
		t.Fatalf("ListUsers()[0].Id = %q, want %q", listed.Msg.GetUsers()[0].GetId(), created.Msg.GetUser().GetId())
	}
}

func TestService_DeleteUser_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteUser(context.Background(), connect.NewRequest(&v1.DeleteUserRequest{Id: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("DeleteUser() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_DeleteUser_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteUser(context.Background(), connect.NewRequest(&v1.DeleteUserRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteUser() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DeleteUser_ThenListUsers(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateUser(ctx, connect.NewRequest(&v1.CreateUserRequest{Name: "Liam"}))
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if _, err := svc.DeleteUser(ctx, connect.NewRequest(&v1.DeleteUserRequest{Id: created.Msg.GetUser().GetId()})); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	listed, err := svc.ListUsers(ctx, connect.NewRequest(&v1.ListUsersRequest{}))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(listed.Msg.GetUsers()) != 0 {
		t.Fatalf("ListUsers() after delete = %d users, want 0", len(listed.Msg.GetUsers()))
	}
}
