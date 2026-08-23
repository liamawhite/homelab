-- One row per calendar date, GBP->USD only (the only pair this app needs
-- today). Refreshed by internal/exchangerate's background loop (at most
-- once a day, checked hourly), never written to on the request path -
-- GetRates only ever reads this table.
CREATE TABLE exchange_rates (
    date        TEXT PRIMARY KEY,
    usd_per_gbp REAL NOT NULL,
    fetched_at  TEXT NOT NULL
);

-- Seeds the one historical date this household's data actually needs
-- (every asset's valuation is dated 2025-12-31) - there's no automated
-- backfill, so any earlier date simply has no rate. Rate is the real
-- Frankfurter GBP->USD rate for that date, fetched once by hand.
INSERT INTO exchange_rates (date, usd_per_gbp, fetched_at) VALUES
    ('2025-12-31', 1.3475, '2025-12-31T00:00:00Z');
