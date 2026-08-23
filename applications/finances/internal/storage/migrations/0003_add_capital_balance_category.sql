-- Widen assets.category to add 'capital_balance', and drop
-- asset_valuations.amount's CHECK (amount >= 0) entirely - a capital
-- balance tracker's valuations are signed (see AssetValuation's doc
-- comment in asset.proto), and SQLite can't express "only this category
-- may be negative" as a CHECK (no cross-table joins in CHECK). Enforcing
-- non-negative for every other category moves fully into assetservice.
--
-- Both are CHECK-constraint changes, which SQLite only allows via the
-- rebuild technique already used throughout this schema's history. This is
-- the exact scenario that once cascade-deleted live data (see migrate.go's
-- foreign_keys-safety fix and its regression test) - that fix already
-- covers this migration generically, verified again here with a
-- migration-specific regression test in migrate_test.go.

CREATE TABLE assets_new (
    id                    TEXT PRIMARY KEY,
    category              TEXT NOT NULL CHECK (category IN (
        'stock_or_bond', 'cash', 'etf', 'retirement', 'real_estate', 'loan', 'mortgage', 'capital_balance'
    )),
    currency              TEXT NOT NULL CHECK (currency IN ('usd', 'gbp')),

    -- stock_or_bond
    company_or_bond_name  TEXT,
    -- shared by stock_or_bond and etf
    platform              TEXT,
    -- cash, capital_balance (name)
    account_name          TEXT,
    institution           TEXT,
    -- etf
    etf_name              TEXT,
    security_type         TEXT CHECK (security_type IS NULL OR security_type IN (
        'stock', 'bond', 'commodity', 'crypto'
    )),
    -- retirement
    plan_name             TEXT,
    -- real_estate, mortgage
    property_name         TEXT,
    -- real_estate
    notes                 TEXT,

    created_at            TEXT NOT NULL
);

INSERT INTO assets_new (
    id, category, currency, company_or_bond_name, platform, account_name,
    institution, etf_name, security_type, plan_name, property_name, notes,
    created_at
)
SELECT
    id, category, currency, company_or_bond_name, platform, account_name,
    institution, etf_name, security_type, plan_name, property_name, notes,
    created_at
FROM assets;

DROP TABLE assets;

ALTER TABLE assets_new RENAME TO assets;

CREATE TABLE asset_valuations_new (
    id         TEXT PRIMARY KEY,
    asset_id   TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    date       TEXT NOT NULL,
    amount     INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

INSERT INTO asset_valuations_new (id, asset_id, date, amount, created_at)
SELECT id, asset_id, date, amount, created_at FROM asset_valuations;

DROP TABLE asset_valuations;

ALTER TABLE asset_valuations_new RENAME TO asset_valuations;

CREATE INDEX idx_asset_valuations_asset_id_date ON asset_valuations(asset_id, date DESC);
