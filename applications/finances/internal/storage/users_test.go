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

func TestUsers_SeededDefaults(t *testing.T) {
	ctx := context.Background()
	users := openTestDB(t)

	list, err := users.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List() on fresh db = %d users, want 2 seeded users", len(list))
	}
	if list[0].Name != "Liam White" || list[1].Name != "Tia Louden" {
		t.Fatalf("List() names = [%q, %q], want [%q, %q]", list[0].Name, list[1].Name, "Liam White", "Tia Louden")
	}
}

func TestUsers_CreateAndList(t *testing.T) {
	ctx := context.Background()
	users := openTestDB(t)

	created, err := users.Create(ctx, "Guest")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}
	if created.Name != "Guest" {
		t.Fatalf("Create() name = %q, want %q", created.Name, "Guest")
	}

	list, err := users.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List() after create = %d users, want 3", len(list))
	}
	if list[2].ID != created.ID || list[2].Name != created.Name {
		t.Fatalf("List()[2] = %+v, want %+v", list[2], created)
	}
}

func TestUsers_Delete(t *testing.T) {
	ctx := context.Background()
	users := openTestDB(t)

	created, err := users.Create(ctx, "Guest")
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
	if len(list) != 2 {
		t.Fatalf("List() after delete = %d users, want 2", len(list))
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
