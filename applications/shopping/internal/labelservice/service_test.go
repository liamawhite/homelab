package labelservice

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

	return New(storage.NewLabels(db))
}

func TestService_ListLabels_EmptyOnFreshDB(t *testing.T) {
	svc := newTestService(t)

	listed, err := svc.ListLabels(context.Background(), connect.NewRequest(&v1.ListLabelsRequest{}))
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if len(listed.Msg.GetLabels()) != 0 {
		t.Fatalf("ListLabels() on a fresh db = %d labels, want 0", len(listed.Msg.GetLabels()))
	}
}

func TestService_CreateLabel_RejectsEmptyName(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.CreateLabel(context.Background(), connect.NewRequest(&v1.CreateLabelRequest{Name: "  "}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateLabel() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateLabel_ThenListLabels(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateLabel(ctx, connect.NewRequest(&v1.CreateLabelRequest{Name: "Groceries"}))
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}
	if created.Msg.GetLabel().GetArchived() {
		t.Fatal("CreateLabel() returned an already-archived label")
	}
	if created.Msg.GetLabel().GetColor() != storage.LabelPalette[0] {
		t.Fatalf("CreateLabel() color = %q, want %q", created.Msg.GetLabel().GetColor(), storage.LabelPalette[0])
	}

	listed, err := svc.ListLabels(ctx, connect.NewRequest(&v1.ListLabelsRequest{}))
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if len(listed.Msg.GetLabels()) != 1 {
		t.Fatalf("ListLabels() = %d labels, want 1", len(listed.Msg.GetLabels()))
	}
}

func TestService_ArchiveLabel_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.ArchiveLabel(context.Background(), connect.NewRequest(&v1.ArchiveLabelRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("ArchiveLabel() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_ArchiveThenRestoreLabel(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateLabel(ctx, connect.NewRequest(&v1.CreateLabelRequest{Name: "Groceries"}))
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}

	if _, err := svc.ArchiveLabel(ctx, connect.NewRequest(&v1.ArchiveLabelRequest{Id: created.Msg.GetLabel().GetId()})); err != nil {
		t.Fatalf("ArchiveLabel() error = %v", err)
	}

	listed, err := svc.ListLabels(ctx, connect.NewRequest(&v1.ListLabelsRequest{}))
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	var archived bool
	for _, l := range listed.Msg.GetLabels() {
		if l.GetId() == created.Msg.GetLabel().GetId() {
			archived = l.GetArchived()
		}
	}
	if !archived {
		t.Fatal("label not archived after ArchiveLabel()")
	}

	if _, err := svc.RestoreLabel(ctx, connect.NewRequest(&v1.RestoreLabelRequest{Id: created.Msg.GetLabel().GetId()})); err != nil {
		t.Fatalf("RestoreLabel() error = %v", err)
	}

	listed, err = svc.ListLabels(ctx, connect.NewRequest(&v1.ListLabelsRequest{}))
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	for _, l := range listed.Msg.GetLabels() {
		if l.GetId() == created.Msg.GetLabel().GetId() && l.GetArchived() {
			t.Fatal("label still archived after RestoreLabel()")
		}
	}
}

func TestService_UpdateLabelColor_RejectsEmptyID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateLabelColor(context.Background(), connect.NewRequest(&v1.UpdateLabelColorRequest{
		Id:    "  ",
		Color: storage.LabelPalette[0],
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateLabelColor() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateLabelColor_RejectsNonPaletteColor(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateLabel(ctx, connect.NewRequest(&v1.CreateLabelRequest{Name: "Groceries"}))
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}

	_, err = svc.UpdateLabelColor(ctx, connect.NewRequest(&v1.UpdateLabelColorRequest{
		Id:    created.Msg.GetLabel().GetId(),
		Color: "#123456",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateLabelColor() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_UpdateLabelColor_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.UpdateLabelColor(context.Background(), connect.NewRequest(&v1.UpdateLabelColorRequest{
		Id:    "does-not-exist",
		Color: storage.LabelPalette[0],
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateLabelColor() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_UpdateLabelColor_Success(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	created, err := svc.CreateLabel(ctx, connect.NewRequest(&v1.CreateLabelRequest{Name: "Groceries"}))
	if err != nil {
		t.Fatalf("CreateLabel() error = %v", err)
	}

	newColor := storage.LabelPalette[len(storage.LabelPalette)-1]
	updated, err := svc.UpdateLabelColor(ctx, connect.NewRequest(&v1.UpdateLabelColorRequest{
		Id:    created.Msg.GetLabel().GetId(),
		Color: newColor,
	}))
	if err != nil {
		t.Fatalf("UpdateLabelColor() error = %v", err)
	}
	if updated.Msg.GetLabel().GetColor() != newColor {
		t.Fatalf("UpdateLabelColor() color = %q, want %q", updated.Msg.GetLabel().GetColor(), newColor)
	}
}
