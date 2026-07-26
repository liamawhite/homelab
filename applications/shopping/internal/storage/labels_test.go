package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestLabels_NoSeededLabels(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	list, err := labels.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List() on a fresh db = %d labels, want 0 - there is no catch-all fallback label anymore", len(list))
	}
}

func TestLabels_CreateAndList(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	created, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}
	if created.Archived {
		t.Fatal("Create() returned an already-archived label")
	}
	if created.Color != LabelPalette[0] {
		t.Fatalf("Create() color = %q, want the first palette color %q", created.Color, LabelPalette[0])
	}

	list, err := labels.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() = %d labels, want 1", len(list))
	}
}

func TestLabels_ArchiveAndRestore(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	created, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := labels.Archive(ctx, created.ID); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	got, err := labels.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !got.Archived {
		t.Fatal("Get() after Archive() = not archived, want archived")
	}

	if err := labels.Restore(ctx, created.ID); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	got, err = labels.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Archived {
		t.Fatal("Get() after Restore() = archived, want not archived")
	}
}

func TestLabels_Archive_NotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	err := labels.Archive(ctx, "does-not-exist")
	if !errors.Is(err, ErrLabelNotFound) {
		t.Fatalf("Archive() error = %v, want ErrLabelNotFound", err)
	}
}

func TestLabels_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	_, err := labels.Get(ctx, "does-not-exist")
	if !errors.Is(err, ErrLabelNotFound) {
		t.Fatalf("Get() error = %v, want ErrLabelNotFound", err)
	}
}

func TestLabels_Create_RotatesThroughPalette(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	for i, want := range LabelPalette {
		created, err := labels.Create(ctx, fmt.Sprintf("Label %d", i))
		if err != nil {
			t.Fatalf("Create() #%d error = %v", i, err)
		}
		if created.Color != want {
			t.Fatalf("Create() #%d color = %q, want %q", i, created.Color, want)
		}
	}

	// Wraps back around to the start once every palette color is used.
	wrapped, err := labels.Create(ctx, "Label wrap")
	if err != nil {
		t.Fatalf("Create() (wrap) error = %v", err)
	}
	if wrapped.Color != LabelPalette[0] {
		t.Fatalf("Create() (wrap) color = %q, want %q", wrapped.Color, LabelPalette[0])
	}
}

func TestLabels_SetColor(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	created, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	newColor := LabelPalette[len(LabelPalette)-1]
	updated, err := labels.SetColor(ctx, created.ID, newColor)
	if err != nil {
		t.Fatalf("SetColor() error = %v", err)
	}
	if updated.Color != newColor {
		t.Fatalf("SetColor() color = %q, want %q", updated.Color, newColor)
	}
}

func TestLabels_SetColor_RejectsNonPaletteColor(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	created, err := labels.Create(ctx, "Groceries")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = labels.SetColor(ctx, created.ID, "#123456")
	if !errors.Is(err, ErrInvalidColor) {
		t.Fatalf("SetColor() error = %v, want ErrInvalidColor", err)
	}
}

func TestLabels_SetColor_NotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	labels := NewLabels(db)

	_, err := labels.SetColor(ctx, "does-not-exist", LabelPalette[0])
	if !errors.Is(err, ErrLabelNotFound) {
		t.Fatalf("SetColor() error = %v, want ErrLabelNotFound", err)
	}
}
