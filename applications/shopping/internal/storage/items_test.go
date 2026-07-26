package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *Items {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return NewItems(db)
}

func TestItems_CreateAndList(t *testing.T) {
	ctx := context.Background()
	items := openTestDB(t)

	list, err := items.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List() on empty db = %d items, want 0", len(list))
	}

	created, err := items.Create(ctx, "Milk")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}
	if created.Name != "Milk" {
		t.Fatalf("Create() name = %q, want %q", created.Name, "Milk")
	}

	list, err = items.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() after create = %d items, want 1", len(list))
	}
	if list[0].ID != created.ID || list[0].Name != created.Name {
		t.Fatalf("List()[0] = %+v, want %+v", list[0], created)
	}
}

func TestItems_ListOrdersByCreatedAt(t *testing.T) {
	ctx := context.Background()
	items := openTestDB(t)

	first, err := items.Create(ctx, "Bread")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := items.Create(ctx, "Eggs")
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
	if list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("List() order = [%s, %s], want [%s, %s]", list[0].ID, list[1].ID, first.ID, second.ID)
	}
}

func TestItems_Delete(t *testing.T) {
	ctx := context.Background()
	items := openTestDB(t)

	created, err := items.Create(ctx, "Milk")
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
	items := openTestDB(t)

	err := items.Delete(ctx, "does-not-exist")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("Delete() error = %v, want ErrItemNotFound", err)
	}
}
