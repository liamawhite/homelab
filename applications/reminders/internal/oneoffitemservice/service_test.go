package oneoffitemservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/reminders/gen/reminders/v1"
	"github.com/liamawhite/reminders/internal/storage"
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

	return New(storage.NewOneOffItems(db))
}

func TestService_CreateOneOffItem_RejectsEmptyTitle(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateOneOffItem(context.Background(), connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "   "}))
	if err == nil {
		t.Fatal("CreateOneOffItem() with blank title error = nil, want CodeInvalidArgument")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateOneOffItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateOneOffItem_ThenListOneOffItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk"}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}
	if created.Msg.GetItem().GetId() == "" {
		t.Fatal("CreateOneOffItem() returned empty item id")
	}
	if created.Msg.GetItem().GetTitle() != "Buy milk" {
		t.Fatalf("CreateOneOffItem() title = %q, want %q", created.Msg.GetItem().GetTitle(), "Buy milk")
	}
	if created.Msg.GetItem().GetCreatedAt() == nil {
		t.Fatal("CreateOneOffItem() returned nil created_at")
	}

	listed, err := svc.ListOneOffItems(ctx, connect.NewRequest(&v1.ListOneOffItemsRequest{}))
	if err != nil {
		t.Fatalf("ListOneOffItems() error = %v", err)
	}
	if len(listed.Msg.GetItems()) != 1 {
		t.Fatalf("ListOneOffItems() = %d items, want 1", len(listed.Msg.GetItems()))
	}
	if listed.Msg.GetItems()[0].GetId() != created.Msg.GetItem().GetId() {
		t.Fatalf("ListOneOffItems()[0].Id = %q, want %q", listed.Msg.GetItems()[0].GetId(), created.Msg.GetItem().GetId())
	}
}

func TestService_DeleteOneOffItem_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteOneOffItem(context.Background(), connect.NewRequest(&v1.DeleteOneOffItemRequest{Id: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("DeleteOneOffItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_DeleteOneOffItem_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteOneOffItem(context.Background(), connect.NewRequest(&v1.DeleteOneOffItemRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteOneOffItem() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DeleteOneOffItem_ThenListOneOffItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk"}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}

	if _, err := svc.DeleteOneOffItem(ctx, connect.NewRequest(&v1.DeleteOneOffItemRequest{Id: created.Msg.GetItem().GetId()})); err != nil {
		t.Fatalf("DeleteOneOffItem() error = %v", err)
	}

	listed, err := svc.ListOneOffItems(ctx, connect.NewRequest(&v1.ListOneOffItemsRequest{}))
	if err != nil {
		t.Fatalf("ListOneOffItems() error = %v", err)
	}
	if len(listed.Msg.GetItems()) != 0 {
		t.Fatalf("ListOneOffItems() after delete = %d items, want 0", len(listed.Msg.GetItems()))
	}
}
