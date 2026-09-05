package oneoffitemservice

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

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

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
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

func TestService_UpdateOneOffItemNotes_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateOneOffItemNotes(context.Background(), connect.NewRequest(&v1.UpdateOneOffItemNotesRequest{Id: "  ", Notes: "n"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateOneOffItemNotes() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateOneOffItemNotes_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateOneOffItemNotes(context.Background(), connect.NewRequest(&v1.UpdateOneOffItemNotesRequest{Id: "does-not-exist", Notes: "n"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateOneOffItemNotes() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateOneOffItemNotes_ThenListOneOffItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}
	if created.Msg.GetItem().GetNotes() != "" {
		t.Fatalf("CreateOneOffItem() notes = %q, want empty", created.Msg.GetItem().GetNotes())
	}

	updated, err := svc.UpdateOneOffItemNotes(ctx, connect.NewRequest(&v1.UpdateOneOffItemNotesRequest{
		Id:    created.Msg.GetItem().GetId(),
		Notes: "low-fat, sell-by Friday",
	}))
	if err != nil {
		t.Fatalf("UpdateOneOffItemNotes() error = %v", err)
	}
	if updated.Msg.GetItem().GetNotes() != "low-fat, sell-by Friday" {
		t.Fatalf("UpdateOneOffItemNotes() notes = %q, want %q", updated.Msg.GetItem().GetNotes(), "low-fat, sell-by Friday")
	}

	listed, err := svc.ListOneOffItems(ctx, connect.NewRequest(&v1.ListOneOffItemsRequest{}))
	if err != nil {
		t.Fatalf("ListOneOffItems() error = %v", err)
	}
	if listed.Msg.GetItems()[0].GetNotes() != "low-fat, sell-by Friday" {
		t.Fatalf("ListOneOffItems()[0].Notes = %q, want %q", listed.Msg.GetItems()[0].GetNotes(), "low-fat, sell-by Friday")
	}
}

func TestService_CreateOneOffItem_WithDueDate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{
		Title:   "Renew passport",
		DueDate: timestamppb.New(due),
		Area:    v1.Area_AREA_PERSONAL,
	}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}
	if got := created.Msg.GetItem().GetDueDate(); got == nil || !got.AsTime().Equal(due) {
		t.Fatalf("CreateOneOffItem() due_date = %v, want %v", got, due)
	}
}

func TestService_UpdateOneOffItemDueDate_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateOneOffItemDueDate(context.Background(), connect.NewRequest(&v1.UpdateOneOffItemDueDateRequest{Id: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateOneOffItemDueDate() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateOneOffItemDueDate_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateOneOffItemDueDate(context.Background(), connect.NewRequest(&v1.UpdateOneOffItemDueDateRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateOneOffItemDueDate() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateOneOffItemDueDate_SetThenClear(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}
	if created.Msg.GetItem().GetDueDate() != nil {
		t.Fatalf("CreateOneOffItem() due_date = %v, want nil", created.Msg.GetItem().GetDueDate())
	}

	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	updated, err := svc.UpdateOneOffItemDueDate(ctx, connect.NewRequest(&v1.UpdateOneOffItemDueDateRequest{
		Id:      created.Msg.GetItem().GetId(),
		DueDate: timestamppb.New(due),
	}))
	if err != nil {
		t.Fatalf("UpdateOneOffItemDueDate() error = %v", err)
	}
	if got := updated.Msg.GetItem().GetDueDate(); got == nil || !got.AsTime().Equal(due) {
		t.Fatalf("UpdateOneOffItemDueDate() due_date = %v, want %v", got, due)
	}

	cleared, err := svc.UpdateOneOffItemDueDate(ctx, connect.NewRequest(&v1.UpdateOneOffItemDueDateRequest{
		Id: created.Msg.GetItem().GetId(),
	}))
	if err != nil {
		t.Fatalf("UpdateOneOffItemDueDate() error = %v", err)
	}
	if got := cleared.Msg.GetItem().GetDueDate(); got != nil {
		t.Fatalf("UpdateOneOffItemDueDate() due_date = %v, want nil", got)
	}
}

func TestService_CreateOneOffItem_RejectsUnspecifiedArea(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateOneOffItem(context.Background(), connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateOneOffItem() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateOneOffItem_WithArea(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{
		Title: "Plan birthday party",
		Area:  v1.Area_AREA_FAMILY,
	}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}
	if got := created.Msg.GetItem().GetArea(); got != v1.Area_AREA_FAMILY {
		t.Fatalf("CreateOneOffItem() area = %v, want %v", got, v1.Area_AREA_FAMILY)
	}
}

func TestService_UpdateOneOffItemArea_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateOneOffItemArea(context.Background(), connect.NewRequest(&v1.UpdateOneOffItemAreaRequest{Id: "  ", Area: v1.Area_AREA_WORK}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateOneOffItemArea() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateOneOffItemArea_RejectsUnspecifiedArea(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}

	_, err = svc.UpdateOneOffItemArea(ctx, connect.NewRequest(&v1.UpdateOneOffItemAreaRequest{Id: created.Msg.GetItem().GetId()}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateOneOffItemArea() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateOneOffItemArea_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateOneOffItemArea(context.Background(), connect.NewRequest(&v1.UpdateOneOffItemAreaRequest{Id: "does-not-exist", Area: v1.Area_AREA_WORK}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateOneOffItemArea() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateOneOffItemArea_ThenListOneOffItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}

	updated, err := svc.UpdateOneOffItemArea(ctx, connect.NewRequest(&v1.UpdateOneOffItemAreaRequest{
		Id:   created.Msg.GetItem().GetId(),
		Area: v1.Area_AREA_PROFESSIONAL,
	}))
	if err != nil {
		t.Fatalf("UpdateOneOffItemArea() error = %v", err)
	}
	if got := updated.Msg.GetItem().GetArea(); got != v1.Area_AREA_PROFESSIONAL {
		t.Fatalf("UpdateOneOffItemArea() area = %v, want %v", got, v1.Area_AREA_PROFESSIONAL)
	}

	listed, err := svc.ListOneOffItems(ctx, connect.NewRequest(&v1.ListOneOffItemsRequest{}))
	if err != nil {
		t.Fatalf("ListOneOffItems() error = %v", err)
	}
	if got := listed.Msg.GetItems()[0].GetArea(); got != v1.Area_AREA_PROFESSIONAL {
		t.Fatalf("ListOneOffItems()[0].Area = %v, want %v", got, v1.Area_AREA_PROFESSIONAL)
	}
}

func TestService_CreateOneOffItem_DefaultsToOpenStatus(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}
	if got := created.Msg.GetItem().GetStatus(); got != v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_OPEN {
		t.Fatalf("CreateOneOffItem() status = %v, want %v", got, v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_OPEN)
	}
}

func TestService_UpdateOneOffItemStatus_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateOneOffItemStatus(context.Background(), connect.NewRequest(&v1.UpdateOneOffItemStatusRequest{
		Id:     "  ",
		Status: v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateOneOffItemStatus() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateOneOffItemStatus_RejectsUnspecifiedStatus(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}

	_, err = svc.UpdateOneOffItemStatus(ctx, connect.NewRequest(&v1.UpdateOneOffItemStatusRequest{Id: created.Msg.GetItem().GetId()}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateOneOffItemStatus() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateOneOffItemStatus_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateOneOffItemStatus(context.Background(), connect.NewRequest(&v1.UpdateOneOffItemStatusRequest{
		Id:     "does-not-exist",
		Status: v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED,
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateOneOffItemStatus() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateOneOffItemStatus_ThenListOneOffItems(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
	if err != nil {
		t.Fatalf("CreateOneOffItem() error = %v", err)
	}

	updated, err := svc.UpdateOneOffItemStatus(ctx, connect.NewRequest(&v1.UpdateOneOffItemStatusRequest{
		Id:     created.Msg.GetItem().GetId(),
		Status: v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED,
	}))
	if err != nil {
		t.Fatalf("UpdateOneOffItemStatus() error = %v", err)
	}
	if got := updated.Msg.GetItem().GetStatus(); got != v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED {
		t.Fatalf("UpdateOneOffItemStatus() status = %v, want %v", got, v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED)
	}

	// Listing still returns completed items - filtering them out of the
	// visible "today" list is a client-side concern (see web/src/lib/oneOffItems.ts).
	listed, err := svc.ListOneOffItems(ctx, connect.NewRequest(&v1.ListOneOffItemsRequest{}))
	if err != nil {
		t.Fatalf("ListOneOffItems() error = %v", err)
	}
	if got := listed.Msg.GetItems()[0].GetStatus(); got != v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED {
		t.Fatalf("ListOneOffItems()[0].Status = %v, want %v", got, v1.OneOffItemStatus_ONE_OFF_ITEM_STATUS_COMPLETED)
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

	created, err := svc.CreateOneOffItem(ctx, connect.NewRequest(&v1.CreateOneOffItemRequest{Title: "Buy milk", Area: v1.Area_AREA_PERSONAL}))
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
