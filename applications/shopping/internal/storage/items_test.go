package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return db
}

// newTestItems returns an Items repository plus a freshly created label -
// there's no seeded fallback label, so every test needs a real one to
// create items under.
func newTestItems(t *testing.T) (*Items, Label) {
	t.Helper()

	db := openTestDB(t)
	label, err := NewLabels(db).Create(context.Background(), "Test")
	if err != nil {
		t.Fatalf("labels.Create() error = %v", err)
	}

	return NewItems(db), label
}

func TestItems_CreateAndList(t *testing.T) {
	ctx := context.Background()
	items, label := newTestItems(t)

	list, err := items.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List() on empty db = %d items, want 0", len(list))
	}

	created, err := items.Create(ctx, "Milk", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}
	if created.Name != "Milk" {
		t.Fatalf("Create() name = %q, want %q", created.Name, "Milk")
	}
	if created.LabelID != label.ID || created.LabelName != label.Name {
		t.Fatalf("Create() label = (%q, %q), want (%q, %q)", created.LabelID, created.LabelName, label.ID, label.Name)
	}
	if created.Status != ItemStatusTodo {
		t.Fatalf("Create() status = %q, want %q", created.Status, ItemStatusTodo)
	}
	if created.CompletedAt != nil {
		t.Fatalf("Create() completed_at = %v, want nil", created.CompletedAt)
	}

	list, err = items.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() after create = %d items, want 1", len(list))
	}
	if list[0].ID != created.ID || list[0].Name != created.Name || list[0].LabelName != created.LabelName {
		t.Fatalf("List()[0] = %+v, want %+v", list[0], created)
	}
}

func TestItems_Create_UnderCustomLabel(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	groceries, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("labels.Create() error = %v", err)
	}

	created, err := NewItems(db).Create(ctx, "Milk", groceries)
	if err != nil {
		t.Fatalf("items.Create() error = %v", err)
	}
	if created.LabelID != groceries.ID || created.LabelName != "Groceries" {
		t.Fatalf("Create() label = (%q, %q), want (%q, %q)", created.LabelID, created.LabelName, groceries.ID, "Groceries")
	}
}

func TestItems_ListIncludesEveryItem(t *testing.T) {
	ctx := context.Background()
	items, label := newTestItems(t)

	first, err := items.Create(ctx, "Bread", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := items.Create(ctx, "Eggs", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, err := items.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() = %d items, want 2", len(list))
	}
	// Display order is a client concern (see web/src/lib/items.ts) - List()
	// makes no ordering guarantee, so this only checks membership.
	ids := map[string]bool{list[0].ID: true, list[1].ID: true}
	if !ids[first.ID] || !ids[second.ID] {
		t.Fatalf("List() = %v, want to contain %q and %q", ids, first.ID, second.ID)
	}
}

func TestItems_Delete(t *testing.T) {
	ctx := context.Background()
	items, label := newTestItems(t)

	created, err := items.Create(ctx, "Milk", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := items.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	list, err := items.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List() after delete = %d items, want 0", len(list))
	}
}

func TestItems_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	items, _ := newTestItems(t)

	err := items.Delete(ctx, "does-not-exist")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("Delete() error = %v, want ErrItemNotFound", err)
	}
}

func TestItems_SetStatus_DoneSetsCompletedAt(t *testing.T) {
	ctx := context.Background()
	items, label := newTestItems(t)

	created, err := items.Create(ctx, "Milk", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := items.SetStatus(ctx, created.ID, ItemStatusDone)
	if err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	if updated.Status != ItemStatusDone {
		t.Fatalf("SetStatus() status = %q, want %q", updated.Status, ItemStatusDone)
	}
	if updated.CompletedAt == nil {
		t.Fatal("SetStatus(done) completed_at = nil, want set")
	}
}

func TestItems_SetStatus_BackToTodoClearsCompletedAt(t *testing.T) {
	ctx := context.Background()
	items, label := newTestItems(t)

	created, err := items.Create(ctx, "Milk", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := items.SetStatus(ctx, created.ID, ItemStatusDone); err != nil {
		t.Fatalf("SetStatus(done) error = %v", err)
	}

	updated, err := items.SetStatus(ctx, created.ID, ItemStatusTodo)
	if err != nil {
		t.Fatalf("SetStatus(todo) error = %v", err)
	}
	if updated.Status != ItemStatusTodo {
		t.Fatalf("SetStatus() status = %q, want %q", updated.Status, ItemStatusTodo)
	}
	if updated.CompletedAt != nil {
		t.Fatalf("SetStatus(todo) completed_at = %v, want nil", updated.CompletedAt)
	}
}

func TestItems_SetStatus_NotFound(t *testing.T) {
	ctx := context.Background()
	items, _ := newTestItems(t)

	_, err := items.SetStatus(ctx, "does-not-exist", ItemStatusDone)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("SetStatus() error = %v, want ErrItemNotFound", err)
	}
}

func TestItems_List_MixOfTodoAndDone(t *testing.T) {
	ctx := context.Background()
	items, label := newTestItems(t)

	a, err := items.Create(ctx, "A", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	b, err := items.Create(ctx, "B", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	c, err := items.Create(ctx, "C", label)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := items.SetStatus(ctx, a.ID, ItemStatusDone); err != nil {
		t.Fatalf("SetStatus(a, done) error = %v", err)
	}
	if _, err := items.SetStatus(ctx, b.ID, ItemStatusDone); err != nil {
		t.Fatalf("SetStatus(b, done) error = %v", err)
	}

	// Display order is a client concern (see web/src/lib/items.ts) - List()
	// makes no ordering guarantee, so this only checks each item's own
	// status/completed_at came back correctly, not their relative order.
	list, err := items.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List() = %d items, want 3", len(list))
	}
	byID := make(map[string]Item, len(list))
	for _, item := range list {
		byID[item.ID] = item
	}

	if got := byID[a.ID]; got.Status != ItemStatusDone || got.CompletedAt == nil {
		t.Fatalf("List()[a] = %+v, want done with completed_at set", got)
	}
	if got := byID[b.ID]; got.Status != ItemStatusDone || got.CompletedAt == nil {
		t.Fatalf("List()[b] = %+v, want done with completed_at set", got)
	}
	if got := byID[c.ID]; got.Status != ItemStatusTodo || got.CompletedAt != nil {
		t.Fatalf("List()[c] = %+v, want todo with no completed_at", got)
	}
}
