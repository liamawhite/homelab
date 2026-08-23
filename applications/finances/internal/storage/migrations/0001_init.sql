-- Consolidated schema, flattening what were previously 11 incremental
-- migrations (0001_create_users.sql through
-- 0011_rename_pension_category_to_retirement.sql) into the single shape
-- they'd converged on. This is a clean-slate init, not a migration path -
-- it was applied by wiping and recreating the database (see migrate.go's
-- foreign_keys-safety fix, added after a rebuild-style migration on this
-- table shape cascade-deleted live data; that fix still applies to any
-- future rebuild migration on top of this baseline).

CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- Finances only ever tracks this household's two people. Seed them so the
-- app is usable immediately; UserService still supports adding/removing
-- more later if that changes.
INSERT INTO users (id, name, created_at) VALUES
    ('1e0bbdf5-a356-47ea-803d-ea1af043059d', 'Liam White', '2026-07-27T00:00:00Z'),
    ('a402564b-0a65-4bae-9e80-90a38b47f712', 'Tia Louden', '2026-07-27T00:00:00Z');

-- category and currency are TEXT enums (SQLite has no native enum type).
-- Category-specific fields are flat, nullable columns rather than a
-- per-category table: sqlc has no sum-type support, and it's a handful of
-- optional text columns, not worth a join. Which columns are populated is
-- determined by category and enforced in assetservice, not here.
CREATE TABLE assets (
    id                    TEXT PRIMARY KEY,
    category              TEXT NOT NULL CHECK (category IN (
        'stock_or_bond', 'cash', 'etf', 'retirement', 'real_estate', 'loan', 'mortgage'
    )),
    currency              TEXT NOT NULL CHECK (currency IN ('usd', 'gbp')),

    -- stock_or_bond
    company_or_bond_name  TEXT,
    -- shared by stock_or_bond and etf
    platform              TEXT,
    -- cash
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

-- One row per (asset, user) sharing ownership. Percentages must sum to 100
-- across an asset's owners - SQLite has no cross-row CHECK/trigger for
-- this, so it's validated in assetservice before every write.
CREATE TABLE asset_owners (
    id         TEXT PRIMARY KEY,
    asset_id   TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    percentage INTEGER NOT NULL CHECK (percentage > 0 AND percentage <= 100),
    created_at TEXT NOT NULL,
    UNIQUE (asset_id, user_id)
);

CREATE INDEX idx_asset_owners_asset_id ON asset_owners(asset_id);

-- A percentage breakdown of an asset's value - geographic (us/international/uk)
-- for most categories, but also doubles as a general allocation bucket via
-- 'bonds' for assets (like retirement accounts) that report a blended
-- stock/bond mix. Optional - an asset may have zero rows (e.g. commodities,
-- crypto, and liabilities have no meaningful geography) - but if any are
-- present they must sum to exactly 100, enforced in assetservice, not here.
CREATE TABLE asset_exposures (
    id         TEXT PRIMARY KEY,
    asset_id   TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    region     TEXT NOT NULL CHECK (region IN ('us', 'international', 'uk', 'bonds')),
    percentage INTEGER NOT NULL CHECK (percentage > 0 AND percentage <= 100),
    created_at TEXT NOT NULL,
    UNIQUE (asset_id, region)
);

CREATE INDEX idx_asset_exposures_asset_id ON asset_exposures(asset_id);

-- A dated snapshot of an asset's value, in minor units (cents/pence) of the
-- asset's currency. Every asset gets its first row transactionally at
-- creation (storage.Assets.Create); "current value" for display is the
-- valuation with the most recent date.
CREATE TABLE asset_valuations (
    id         TEXT PRIMARY KEY,
    asset_id   TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    date       TEXT NOT NULL,
    amount     INTEGER NOT NULL CHECK (amount >= 0),
    created_at TEXT NOT NULL
);

CREATE INDEX idx_asset_valuations_asset_id_date ON asset_valuations(asset_id, date DESC);
