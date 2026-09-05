-- Replaces the is_liquid boolean with a three-way liquidity tier: 'liquid'
-- (cash), 'invested' (a liquid market position that still takes a
-- sell/transfer to spend - ETFs, individual stocks, loans, the capital
-- balance ledger), or 'illiquid' (locked up - retirement, real estate,
-- mortgage, or any manually-overridden instance). Plain ADD COLUMN + DROP
-- COLUMN, no rebuild-table technique needed (no FK-cascade risk).
ALTER TABLE assets ADD COLUMN liquidity TEXT NOT NULL DEFAULT 'liquid'
    CHECK (liquidity IN ('liquid', 'invested', 'illiquid'));

-- Backfill from the existing is_liquid column: an explicit is_liquid=0
-- override (whatever its category) stays illiquid; a liquid cash asset
-- stays liquid; a liquid bond ETF (a money-market/T-bill fund like VBIL -
-- functionally a cash equivalent) also stays liquid; every other liquid
-- asset becomes invested.
UPDATE assets SET liquidity = CASE
    WHEN is_liquid = 0 THEN 'illiquid'
    WHEN category = 'cash' THEN 'liquid'
    WHEN category = 'etf' AND security_type = 'bond' THEN 'liquid'
    ELSE 'invested'
END;

ALTER TABLE assets DROP COLUMN is_liquid;
