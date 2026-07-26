package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *Users {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return NewUsers(db)
}

func TestUsers_CreateAndList(t *testing.T) {
	ctx := context.Background()
	users := openTestDB(t)

	list, err := users.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List() on empty db = %d users, want 0", len(list))
	}

	created, err := users.Create(ctx, "Liam")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}
	if created.Name != "Liam" {
		t.Fatalf("Create() name = %q, want %q", created.Name, "Liam")
	}

	list, err = users.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() after create = %d users, want 1", len(list))
	}
	if list[0].ID != created.ID || list[0].Name != created.Name {
		t.Fatalf("List()[0] = %+v, want %+v", list[0], created)
	}
}

func TestUsers_ListOrdersByCreatedAt(t *testing.T) {
	ctx := context.Background()
	users := openTestDB(t)

	first, err := users.Create(ctx, "First")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := users.Create(ctx, "Second")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, err := users.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() = %d users, want 2", len(list))
	}
	if list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("List() order = [%s, %s], want [%s, %s]", list[0].ID, list[1].ID, first.ID, second.ID)
	}
}

func TestUsers_Delete(t *testing.T) {
	ctx := context.Background()
	users := openTestDB(t)

	created, err := users.Create(ctx, "Liam")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := users.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	list, err := users.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List() after delete = %d users, want 0", len(list))
	}
}

func TestUsers_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	users := openTestDB(t)

	err := users.Delete(ctx, "does-not-exist")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("Delete() error = %v, want ErrUserNotFound", err)
	}
}
