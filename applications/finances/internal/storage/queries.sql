-- name: ListUsers :many
SELECT id, name, created_at FROM users ORDER BY created_at ASC;

-- name: CreateUser :one
INSERT INTO users (id, name, created_at) VALUES (?, ?, ?) RETURNING id, name, created_at;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = ?;

-- name: GetUser :one
SELECT id, name, created_at FROM users WHERE id = ?;

-- Assets

-- name: ListAssets :many
SELECT id, category, currency, company_or_bond_name, platform, account_name,
       institution, etf_name, security_type, plan_name, property_name, notes,
       created_at
FROM assets ORDER BY created_at ASC;

-- name: GetAsset :one
SELECT id, category, currency, company_or_bond_name, platform, account_name,
       institution, etf_name, security_type, plan_name, property_name, notes,
       created_at
FROM assets WHERE id = ?;

-- name: CreateAsset :one
INSERT INTO assets (
    id, category, currency, company_or_bond_name, platform, account_name,
    institution, etf_name, security_type, plan_name, property_name, notes,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, category, currency, company_or_bond_name, platform, account_name,
    institution, etf_name, security_type, plan_name, property_name, notes,
    created_at;

-- name: UpdateAsset :one
UPDATE assets SET
    category = ?, currency = ?, company_or_bond_name = ?, platform = ?,
    account_name = ?, institution = ?, etf_name = ?, security_type = ?,
    plan_name = ?, property_name = ?, notes = ?
WHERE id = ?
RETURNING id, category, currency, company_or_bond_name, platform, account_name,
    institution, etf_name, security_type, plan_name, property_name, notes,
    created_at;

-- name: DeleteAsset :execrows
DELETE FROM assets WHERE id = ?;

-- Asset owners

-- name: ListAssetOwnersByAsset :many
SELECT asset_owners.id, asset_owners.asset_id, asset_owners.user_id,
       users.name AS user_name, asset_owners.percentage, asset_owners.created_at
FROM asset_owners JOIN users ON users.id = asset_owners.user_id
WHERE asset_owners.asset_id = ?
ORDER BY asset_owners.created_at ASC;

-- name: CreateAssetOwner :one
INSERT INTO asset_owners (id, asset_id, user_id, percentage, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id, asset_id, user_id, percentage, created_at;

-- name: DeleteAssetOwnersByAsset :execrows
DELETE FROM asset_owners WHERE asset_id = ?;

-- Asset exposures

-- name: ListAssetExposuresByAsset :many
SELECT id, asset_id, region, percentage, created_at
FROM asset_exposures WHERE asset_id = ? ORDER BY created_at ASC;

-- name: CreateAssetExposure :one
INSERT INTO asset_exposures (id, asset_id, region, percentage, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id, asset_id, region, percentage, created_at;

-- name: DeleteAssetExposuresByAsset :execrows
DELETE FROM asset_exposures WHERE asset_id = ?;

-- Asset valuations

-- name: ListAssetValuationsByAsset :many
SELECT id, asset_id, date, amount, created_at
FROM asset_valuations WHERE asset_id = ? ORDER BY date DESC, created_at DESC;

-- name: GetLatestAssetValuation :one
SELECT id, asset_id, date, amount, created_at
FROM asset_valuations WHERE asset_id = ?
ORDER BY date DESC, created_at DESC LIMIT 1;

-- name: CreateAssetValuation :one
INSERT INTO asset_valuations (id, asset_id, date, amount, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id, asset_id, date, amount, created_at;

-- name: DeleteAssetValuation :execrows
DELETE FROM asset_valuations WHERE id = ?;

-- Exchange rates

-- name: UpsertExchangeRate :exec
INSERT INTO exchange_rates (date, usd_per_gbp, fetched_at)
VALUES (?, ?, ?)
ON CONFLICT (date) DO UPDATE SET usd_per_gbp = excluded.usd_per_gbp, fetched_at = excluded.fetched_at;

-- name: GetExchangeRateOnOrBefore :one
SELECT date, usd_per_gbp, fetched_at
FROM exchange_rates WHERE date <= ? ORDER BY date DESC LIMIT 1;
