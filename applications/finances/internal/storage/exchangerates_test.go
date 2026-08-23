package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openExchangeRatesTestDB(t *testing.T) *ExchangeRates {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return NewExchangeRates(db)
}

func TestExchangeRates_SeededRate(t *testing.T) {
	ctx := context.Background()
	rates := openExchangeRatesTestDB(t)

	rate, err := rates.RateOnOrBefore(ctx, "2025-12-31")
	if err != nil {
		t.Fatalf("RateOnOrBefore() error = %v", err)
	}
	if rate.Date != "2025-12-31" || rate.USDPerGBP != 1.3475 {
		t.Fatalf("RateOnOrBefore() = %+v, want date 2025-12-31 rate 1.3475", rate)
	}
}

func TestExchangeRates_Upsert_ThenExactMatch(t *testing.T) {
	ctx := context.Background()
	rates := openExchangeRatesTestDB(t)

	if err := rates.Upsert(ctx, "2026-01-05", 1.30); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	rate, err := rates.RateOnOrBefore(ctx, "2026-01-05")
	if err != nil {
		t.Fatalf("RateOnOrBefore() error = %v", err)
	}
	if rate.USDPerGBP != 1.30 {
		t.Fatalf("RateOnOrBefore() rate = %v, want 1.30", rate.USDPerGBP)
	}
}

func TestExchangeRates_Upsert_Overwrites(t *testing.T) {
	ctx := context.Background()
	rates := openExchangeRatesTestDB(t)

	if err := rates.Upsert(ctx, "2026-01-05", 1.30); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := rates.Upsert(ctx, "2026-01-05", 1.32); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	rate, err := rates.RateOnOrBefore(ctx, "2026-01-05")
	if err != nil {
		t.Fatalf("RateOnOrBefore() error = %v", err)
	}
	if rate.USDPerGBP != 1.32 {
		t.Fatalf("RateOnOrBefore() rate = %v, want 1.32 (overwritten)", rate.USDPerGBP)
	}
}

func TestExchangeRates_RateOnOrBefore_FallsBackToEarlierDate(t *testing.T) {
	ctx := context.Background()
	rates := openExchangeRatesTestDB(t)

	// No rate stored for 2026-01-01 (a weekend/holiday gap) - should fall
	// back to the seeded 2025-12-31 rate.
	rate, err := rates.RateOnOrBefore(ctx, "2026-01-01")
	if err != nil {
		t.Fatalf("RateOnOrBefore() error = %v", err)
	}
	if rate.Date != "2025-12-31" {
		t.Fatalf("RateOnOrBefore() date = %q, want fallback to 2025-12-31", rate.Date)
	}
}

func TestExchangeRates_RateOnOrBefore_NotFoundBeforeAnyRate(t *testing.T) {
	ctx := context.Background()
	rates := openExchangeRatesTestDB(t)

	_, err := rates.RateOnOrBefore(ctx, "2025-01-01")
	if !errors.Is(err, ErrExchangeRateNotFound) {
		t.Fatalf("RateOnOrBefore() error = %v, want ErrExchangeRateNotFound", err)
	}
}
