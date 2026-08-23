package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/liamawhite/finances/internal/storage/sqlcgen"
)

// ErrExchangeRateNotFound is returned when no exchange rate is stored on or
// before the requested date.
var ErrExchangeRateNotFound = errors.New("exchange rate not found")

// ExchangeRate is a single day's GBP->USD rate.
type ExchangeRate struct {
	Date      string // YYYY-MM-DD
	USDPerGBP float64
	FetchedAt time.Time
}

// ExchangeRates is the repository for the exchange_rates table - written
// only by internal/exchangerate's background refresher, read by
// exchangerateservice.
type ExchangeRates struct {
	q *sqlcgen.Queries
}

// NewExchangeRates returns an ExchangeRates repository backed by db.
func NewExchangeRates(db *sql.DB) *ExchangeRates {
	return &ExchangeRates{q: sqlcgen.New(db)}
}

// Upsert stores (or overwrites) the rate for date.
func (e *ExchangeRates) Upsert(ctx context.Context, date string, usdPerGBP float64) error {
	if err := e.q.UpsertExchangeRate(ctx, sqlcgen.UpsertExchangeRateParams{
		Date:      date,
		UsdPerGbp: usdPerGBP,
		FetchedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return fmt.Errorf("upserting exchange rate for %s: %w", date, err)
	}
	return nil
}

// RateOnOrBefore returns the most recent stored rate on or before date -
// weekends/holidays have no rate of their own, so callers get the last
// known-good one instead. Returns ErrExchangeRateNotFound if nothing is
// stored on or before date yet.
func (e *ExchangeRates) RateOnOrBefore(ctx context.Context, date string) (ExchangeRate, error) {
	row, err := e.q.GetExchangeRateOnOrBefore(ctx, date)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExchangeRate{}, ErrExchangeRateNotFound
		}
		return ExchangeRate{}, fmt.Errorf("getting exchange rate on or before %s: %w", date, err)
	}

	fetchedAt, err := time.Parse(time.RFC3339Nano, row.FetchedAt)
	if err != nil {
		return ExchangeRate{}, fmt.Errorf("parsing exchange rate fetched_at: %w", err)
	}

	return ExchangeRate{Date: row.Date, USDPerGBP: row.UsdPerGbp, FetchedAt: fetchedAt}, nil
}
