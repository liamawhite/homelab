package itemservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/shopping/gen/shopping/v1"
	"github.com/liamawhite/shopping/internal/storage"
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

	return New(storage.NewItems(db))
}

func TestService_CreateItem_RejectsEmptyName(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateItem(context.Background(), connect.NewRequest(&v1.CreateItemRequest{Name: "   "}))
	if err == nil {
		t.Fatal("CreateItem() with blank name error = nil, want CodeInvalidArgument")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateItem_ThenListItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateItem(ctx, connect.NewRequest(&v1.CreateItemRequest{Name: "Milk"}))
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}
	if created.Msg.GetItem().GetId() == "" {
		t.Fatal("CreateItem() returned empty item id")
	}
	if created.Msg.GetItem().GetName() != "Milk" {
		t.Fatalf("CreateItem() name = %q, want %q", created.Msg.GetItem().GetName(), "Milk")
	}
	if created.Msg.GetItem().GetCreatedAt() == nil {
		t.Fatal("CreateItem() returned nil created_at")
	}

	listed, err := svc.ListItems(ctx, connect.NewRequest(&v1.ListItemsRequest{}))
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(listed.Msg.GetItems()) != 1 {
		t.Fatalf("ListItems() = %d items, want 1", len(listed.Msg.GetItems()))
	}
	if listed.Msg.GetItems()[0].GetId() != created.Msg.GetItem().GetId() {
		t.Fatalf("ListItems()[0].Id = %q, want %q", listed.Msg.GetItems()[0].GetId(), created.Msg.GetItem().GetId())
	}
}

func TestService_DeleteItem_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteItem(context.Background(), connect.NewRequest(&v1.DeleteItemRequest{Id: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("DeleteItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_DeleteItem_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.DeleteItem(context.Background(), connect.NewRequest(&v1.DeleteItemRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteItem() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DeleteItem_ThenListItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateItem(ctx, connect.NewRequest(&v1.CreateItemRequest{Name: "Milk"}))
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}

	if _, err := svc.DeleteItem(ctx, connect.NewRequest(&v1.DeleteItemRequest{Id: created.Msg.GetItem().GetId()})); err != nil {
		t.Fatalf("DeleteItem() error = %v", err)
	}

	listed, err := svc.ListItems(ctx, connect.NewRequest(&v1.ListItemsRequest{}))
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(listed.Msg.GetItems()) != 0 {
		t.Fatalf("ListItems() after delete = %d items, want 0", len(listed.Msg.GetItems()))
	}
}
