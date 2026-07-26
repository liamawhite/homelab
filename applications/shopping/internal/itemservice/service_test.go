package itemservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/shopping/gen/shopping/v1"
	"github.com/liamawhite/shopping/internal/storage"
)

// newTestService returns a Service plus the Labels repository backing it,
// so tests can set up labels (including archived ones) directly against the
// same database without going through another package's Connect handler.
func newTestService(t *testing.T) (*Service, *storage.Labels) {
	t.Helper()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("storage.Migrate() error = %v", err)
	}

	labels := storage.NewLabels(db)
	return New(storage.NewItems(db), labels), labels
}

func TestService_CreateItem_RejectsEmptyName(t *testing.T) {
	svc, labels := newTestService(t)
	groceries, err := labels.Create(context.Background(), "Groceries")
	if err != nil {
		t.Fatalf("labels.Create() error = %v", err)
	}

	_, err = svc.CreateItem(context.Background(), connect.NewRequest(&v1.CreateItemRequest{Name: "   ", LabelId: groceries.ID}))
	if err == nil {
		t.Fatal("CreateItem() with blank name error = nil, want CodeInvalidArgument")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateItem_RejectsEmptyLabel(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.CreateItem(context.Background(), connect.NewRequest(&v1.CreateItemRequest{Name: "Milk"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateItem_RejectsUnknownLabel(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.CreateItem(context.Background(), connect.NewRequest(&v1.CreateItemRequest{Name: "Milk", LabelId: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateItem_RejectsArchivedLabel(t *testing.T) {
	ctx := context.Background()
	svc, labels := newTestService(t)

	groceries, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("labels.Create() error = %v", err)
	}
	if err := labels.Archive(ctx, groceries.ID); err != nil {
		t.Fatalf("labels.Archive() error = %v", err)
	}

	_, err = svc.CreateItem(ctx, connect.NewRequest(&v1.CreateItemRequest{Name: "Milk", LabelId: groceries.ID}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateItem_ThenListItems(t *testing.T) {
	ctx := context.Background()
	svc, labels := newTestService(t)
	groceries, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("labels.Create() error = %v", err)
	}

	created, err := svc.CreateItem(ctx, connect.NewRequest(&v1.CreateItemRequest{Name: "Milk", LabelId: groceries.ID}))
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
	if created.Msg.GetItem().GetLabelName() != "Groceries" {
		t.Fatalf("CreateItem() label name = %q, want %q", created.Msg.GetItem().GetLabelName(), "Groceries")
	}
	if created.Msg.GetItem().GetStatus() != v1.ItemStatus_ITEM_STATUS_TODO {
		t.Fatalf("CreateItem() status = %v, want %v", created.Msg.GetItem().GetStatus(), v1.ItemStatus_ITEM_STATUS_TODO)
	}
	if created.Msg.GetItem().GetCompletedAt() != nil {
		t.Fatal("CreateItem() completed_at = non-nil, want nil")
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
	svc, _ := newTestService(t)

	_, err := svc.DeleteItem(context.Background(), connect.NewRequest(&v1.DeleteItemRequest{Id: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("DeleteItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_DeleteItem_NotFound(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.DeleteItem(context.Background(), connect.NewRequest(&v1.DeleteItemRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteItem() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DeleteItem_ThenListItems(t *testing.T) {
	ctx := context.Background()
	svc, labels := newTestService(t)
	groceries, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("labels.Create() error = %v", err)
	}

	created, err := svc.CreateItem(ctx, connect.NewRequest(&v1.CreateItemRequest{Name: "Milk", LabelId: groceries.ID}))
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

func TestService_UpdateItemStatus_RejectsEmptyID(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.UpdateItemStatus(context.Background(), connect.NewRequest(&v1.UpdateItemStatusRequest{
		Id:     "  ",
		Status: v1.ItemStatus_ITEM_STATUS_DONE,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateItemStatus() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateItemStatus_RejectsUnspecifiedStatus(t *testing.T) {
	ctx := context.Background()
	svc, labels := newTestService(t)
	groceries, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("labels.Create() error = %v", err)
	}
	created, err := svc.CreateItem(ctx, connect.NewRequest(&v1.CreateItemRequest{Name: "Milk", LabelId: groceries.ID}))
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}

	_, err = svc.UpdateItemStatus(ctx, connect.NewRequest(&v1.UpdateItemStatusRequest{
		Id: created.Msg.GetItem().GetId(),
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateItemStatus() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateItemStatus_NotFound(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.UpdateItemStatus(context.Background(), connect.NewRequest(&v1.UpdateItemStatusRequest{
		Id:     "does-not-exist",
		Status: v1.ItemStatus_ITEM_STATUS_DONE,
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateItemStatus() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateItemStatus_DoneThenBackToTodo(t *testing.T) {
	ctx := context.Background()
	svc, labels := newTestService(t)
	groceries, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("labels.Create() error = %v", err)
	}
	created, err := svc.CreateItem(ctx, connect.NewRequest(&v1.CreateItemRequest{Name: "Milk", LabelId: groceries.ID}))
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}
	id := created.Msg.GetItem().GetId()

	done, err := svc.UpdateItemStatus(ctx, connect.NewRequest(&v1.UpdateItemStatusRequest{
		Id:     id,
		Status: v1.ItemStatus_ITEM_STATUS_DONE,
	}))
	if err != nil {
		t.Fatalf("UpdateItemStatus(done) error = %v", err)
	}
	if done.Msg.GetItem().GetStatus() != v1.ItemStatus_ITEM_STATUS_DONE {
		t.Fatalf("UpdateItemStatus(done) status = %v, want %v", done.Msg.GetItem().GetStatus(), v1.ItemStatus_ITEM_STATUS_DONE)
	}
	if done.Msg.GetItem().GetCompletedAt() == nil {
		t.Fatal("UpdateItemStatus(done) completed_at = nil, want set")
	}
	if done.Msg.GetItem().GetLabelName() != "Groceries" {
		t.Fatalf("UpdateItemStatus(done) label name = %q, want %q", done.Msg.GetItem().GetLabelName(), "Groceries")
	}

	todo, err := svc.UpdateItemStatus(ctx, connect.NewRequest(&v1.UpdateItemStatusRequest{
		Id:     id,
		Status: v1.ItemStatus_ITEM_STATUS_TODO,
	}))
	if err != nil {
		t.Fatalf("UpdateItemStatus(todo) error = %v", err)
	}
	if todo.Msg.GetItem().GetStatus() != v1.ItemStatus_ITEM_STATUS_TODO {
		t.Fatalf("UpdateItemStatus(todo) status = %v, want %v", todo.Msg.GetItem().GetStatus(), v1.ItemStatus_ITEM_STATUS_TODO)
	}
	if todo.Msg.GetItem().GetCompletedAt() != nil {
		t.Fatal("UpdateItemStatus(todo) completed_at = non-nil, want nil")
	}
}
