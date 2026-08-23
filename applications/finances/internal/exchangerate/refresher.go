package exchangerate

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/liamawhite/finances/internal/storage"
)

// checkInterval is how often Run wakes up to check whether today's rate
// still needs fetching - not how often Frankfurter actually gets called.
const checkInterval = time.Hour

// Refresher keeps storage.ExchangeRates populated with at most one fetch a
// day - there's no historical backfill; the table starts with only the
// single 2025-12-31 rate seeded by the schema migration, and this adds one
// more row per day it's run from then on. It never runs on the request
// path - exchangerateservice only ever reads what this has already stored.
type Refresher struct {
	client *Client
	rates  *storage.ExchangeRates
}

// NewRefresher returns a Refresher backed by rates.
func NewRefresher(rates *storage.ExchangeRates) *Refresher {
	return &Refresher{client: NewClient(), rates: rates}
}

// Run checks every checkInterval whether today already has a stored rate,
// and only calls Frankfurter when it doesn't - idempotent by construction,
// so restarting the process (or ticking again within the same day) never
// causes a redundant fetch.
func (r *Refresher) Run(ctx context.Context) {
	r.refreshIfDue(ctx)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.refreshIfDue(ctx)
		}
	}
}

func (r *Refresher) refreshIfDue(ctx context.Context) {
	today := time.Now().UTC().Format("2006-01-02")

	existing, err := r.rates.RateOnOrBefore(ctx, today)
	if err != nil && !errors.Is(err, storage.ErrExchangeRateNotFound) {
		log.Printf("exchangerate: checking today's stored rate: %v", err)
		return
	}
	if err == nil && existing.Date == today {
		return
	}

	rate, date, err := r.client.Latest(ctx)
	if err != nil {
		log.Printf("exchangerate: fetching latest rate: %v", err)
		return
	}

	if err := r.rates.Upsert(ctx, date, rate); err != nil {
		log.Printf("exchangerate: storing latest rate: %v", err)
		return
	}

	log.Printf("exchangerate: stored rate for %s", date)
}
