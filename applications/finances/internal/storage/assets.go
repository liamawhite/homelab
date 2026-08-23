package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/liamawhite/finances/internal/storage/sqlcgen"
)

// ErrAssetNotFound is returned when no asset matches the given ID.
var ErrAssetNotFound = errors.New("asset not found")

// ErrAssetValuationNotFound is returned when no valuation matches the given ID.
var ErrAssetValuationNotFound = errors.New("asset valuation not found")

// AssetCategory determines which of AssetDetails' fields apply to an asset -
// add new values here and to the assets table's category CHECK together.
type AssetCategory string

const (
	AssetCategoryStockOrBond    AssetCategory = "stock_or_bond"
	AssetCategoryCash           AssetCategory = "cash"
	AssetCategoryEtf            AssetCategory = "etf"
	AssetCategoryRetirement     AssetCategory = "retirement"
	AssetCategoryRealEstate     AssetCategory = "real_estate"
	AssetCategoryLoan           AssetCategory = "loan"
	AssetCategoryMortgage       AssetCategory = "mortgage"
	AssetCategoryCapitalBalance AssetCategory = "capital_balance"
)

// Currency is the denomination of an asset's value - fixed for the asset's
// lifetime, not per-valuation. Add new values here and to
// 0003_create_assets.sql's CHECK together.
type Currency string

const (
	CurrencyUSD Currency = "usd"
	CurrencyGBP Currency = "gbp"
)

// Region is one value within an AssetExposure breakdown - geographic for
// most categories, but also doubles as a general allocation bucket via
// RegionBonds for assets (like retirement accounts) that report a blended
// stock/bond mix. Add new values here and to
// 0010_rename_asset_regions_to_asset_exposures.sql's CHECK together.
type Region string

const (
	RegionUS            Region = "us"
	RegionInternational Region = "international"
	RegionUK            Region = "uk"
	RegionBonds         Region = "bonds"
)

// SecurityType is an ETF's stock-or-bond (or commodity/crypto) sub-type.
// Add new values here and to
// 0008_widen_asset_category_and_security_type_checks.sql's CHECK together.
type SecurityType string

const (
	SecurityTypeStock     SecurityType = "stock"
	SecurityTypeBond      SecurityType = "bond"
	SecurityTypeCommodity SecurityType = "commodity"
	SecurityTypeCrypto    SecurityType = "crypto"
)

// AssetDetails holds the category-specific fields on an asset. Only the
// fields relevant to the asset's Category are expected to be set - which
// ones is enforced in assetservice, not here (see 0003_create_assets.sql's
// doc comment for why these are flat nullable columns instead of a
// per-category table).
type AssetDetails struct {
	CompanyOrBondName *string
	Platform          *string
	AccountName       *string
	Institution       *string
	EtfName           *string
	SecurityType      *SecurityType
	PlanName          *string
	PropertyName      *string
	Notes             *string
}

// AssetOwner is a share of an asset's ownership.
type AssetOwner struct {
	ID         string
	AssetID    string
	UserID     string
	UserName   string
	Percentage int
	CreatedAt  time.Time
}

// AssetExposure is a percentage breakdown of an asset's value - geographic
// for most categories, but also a general allocation bucket via RegionBonds
// for assets (like retirement accounts) that report a blended stock/bond mix.
type AssetExposure struct {
	ID         string
	AssetID    string
	Region     Region
	Percentage int
	CreatedAt  time.Time
}

// AssetValuation is a dated snapshot of an asset's value, in minor units
// (cents/pence) of the asset's currency.
type AssetValuation struct {
	ID        string
	AssetID   string
	Date      string
	Amount    int64
	CreatedAt time.Time
}

// OwnerInput is one row of a Create/Update ownership split.
type OwnerInput struct {
	UserID     string
	Percentage int
}

// ExposureInput is one row of a Create/Update exposure breakdown.
type ExposureInput struct {
	Region     Region
	Percentage int
}

// Asset is an asset with its ownership split, exposure breakdown, and most
// recent valuation assembled in.
type Asset struct {
	ID              string
	Category        AssetCategory
	Currency        Currency
	Details         AssetDetails
	IsLiquid        bool
	Owners          []AssetOwner
	Exposures       []AssetExposure
	LatestValuation *AssetValuation
	CreatedAt       time.Time
}

// Assets is the repository for the assets table and its owner/exposure/
// valuation children, backed by sqlc-generated queries (see
// internal/storage/queries.sql and sqlcgen/).
type Assets struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

// NewAssets returns an Assets repository backed by db.
func NewAssets(db *sql.DB) *Assets {
	return &Assets{db: db, q: sqlcgen.New(db)}
}

// List returns every asset, oldest first.
func (a *Assets) List(ctx context.Context) ([]Asset, error) {
	rows, err := a.q.ListAssets(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing assets: %w", err)
	}

	assets := make([]Asset, 0, len(rows))
	for _, row := range rows {
		asset, err := a.assemble(ctx, a.q, row)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}

	return assets, nil
}

// Get returns the asset with the given ID. It returns ErrAssetNotFound if
// no such asset exists.
func (a *Assets) Get(ctx context.Context, id string) (Asset, error) {
	row, err := a.q.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Asset{}, ErrAssetNotFound
		}
		return Asset{}, fmt.Errorf("getting asset: %w", err)
	}

	return a.assemble(ctx, a.q, row)
}

// Create inserts a new asset along with its ownership split, exposure
// breakdown, and an initial valuation, all in one transaction.
func (a *Assets) Create(
	ctx context.Context,
	category AssetCategory,
	currency Currency,
	details AssetDetails,
	isLiquid bool,
	owners []OwnerInput,
	exposures []ExposureInput,
	initialValuationDate string,
	initialValuationAmount int64,
) (Asset, error) {
	var result Asset
	err := withTx(ctx, a.db, func(tx *sql.Tx) error {
		q := a.q.WithTx(tx)
		now := time.Now().UTC().Format(time.RFC3339Nano)

		assetRow, err := q.CreateAsset(ctx, sqlcgen.CreateAssetParams{
			ID:                uuid.NewString(),
			Category:          string(category),
			Currency:          string(currency),
			CompanyOrBondName: nullableString(details.CompanyOrBondName),
			Platform:          nullableString(details.Platform),
			AccountName:       nullableString(details.AccountName),
			Institution:       nullableString(details.Institution),
			EtfName:           nullableString(details.EtfName),
			SecurityType:      nullableSecurityType(details.SecurityType),
			PlanName:          nullableString(details.PlanName),
			PropertyName:      nullableString(details.PropertyName),
			Notes:             nullableString(details.Notes),
			IsLiquid:          boolToInt64(isLiquid),
			CreatedAt:         now,
		})
		if err != nil {
			return fmt.Errorf("creating asset: %w", err)
		}

		if err := createOwners(ctx, q, assetRow.ID, owners, now); err != nil {
			return err
		}
		if err := createExposures(ctx, q, assetRow.ID, exposures, now); err != nil {
			return err
		}

		if _, err := q.CreateAssetValuation(ctx, sqlcgen.CreateAssetValuationParams{
			ID:        uuid.NewString(),
			AssetID:   assetRow.ID,
			Date:      initialValuationDate,
			Amount:    initialValuationAmount,
			CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("creating initial asset valuation: %w", err)
		}

		asset, err := a.assemble(ctx, q, assetRow)
		if err != nil {
			return err
		}
		result = asset
		return nil
	})
	if err != nil {
		return Asset{}, err
	}

	return result, nil
}

// Update replaces an asset's descriptive fields, ownership split, and
// exposure breakdown, all in one transaction. It never touches the asset's
// valuations (see AddValuation). It returns ErrAssetNotFound if no such
// asset exists.
func (a *Assets) Update(
	ctx context.Context,
	id string,
	category AssetCategory,
	currency Currency,
	details AssetDetails,
	isLiquid bool,
	owners []OwnerInput,
	exposures []ExposureInput,
) (Asset, error) {
	var result Asset
	err := withTx(ctx, a.db, func(tx *sql.Tx) error {
		q := a.q.WithTx(tx)

		assetRow, err := q.UpdateAsset(ctx, sqlcgen.UpdateAssetParams{
			ID:                id,
			Category:          string(category),
			Currency:          string(currency),
			CompanyOrBondName: nullableString(details.CompanyOrBondName),
			Platform:          nullableString(details.Platform),
			AccountName:       nullableString(details.AccountName),
			Institution:       nullableString(details.Institution),
			EtfName:           nullableString(details.EtfName),
			SecurityType:      nullableSecurityType(details.SecurityType),
			PlanName:          nullableString(details.PlanName),
			PropertyName:      nullableString(details.PropertyName),
			Notes:             nullableString(details.Notes),
			IsLiquid:          boolToInt64(isLiquid),
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrAssetNotFound
			}
			return fmt.Errorf("updating asset: %w", err)
		}

		if _, err := q.DeleteAssetOwnersByAsset(ctx, id); err != nil {
			return fmt.Errorf("clearing asset owners: %w", err)
		}
		if _, err := q.DeleteAssetExposuresByAsset(ctx, id); err != nil {
			return fmt.Errorf("clearing asset exposures: %w", err)
		}

		now := time.Now().UTC().Format(time.RFC3339Nano)
		if err := createOwners(ctx, q, id, owners, now); err != nil {
			return err
		}
		if err := createExposures(ctx, q, id, exposures, now); err != nil {
			return err
		}

		asset, err := a.assemble(ctx, q, assetRow)
		if err != nil {
			return err
		}
		result = asset
		return nil
	})
	if err != nil {
		return Asset{}, err
	}

	return result, nil
}

// Delete removes the asset with the given ID, cascading to its owners,
// regions, and valuations. It returns ErrAssetNotFound if no such asset
// exists.
func (a *Assets) Delete(ctx context.Context, id string) error {
	affected, err := a.q.DeleteAsset(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting asset: %w", err)
	}
	if affected == 0 {
		return ErrAssetNotFound
	}

	return nil
}

// ListValuations returns every valuation for assetID, newest first. It
// returns ErrAssetNotFound if no such asset exists.
func (a *Assets) ListValuations(ctx context.Context, assetID string) ([]AssetValuation, error) {
	if _, err := a.q.GetAsset(ctx, assetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAssetNotFound
		}
		return nil, fmt.Errorf("getting asset: %w", err)
	}

	rows, err := a.q.ListAssetValuationsByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("listing asset valuations: %w", err)
	}

	valuations := make([]AssetValuation, 0, len(rows))
	for _, row := range rows {
		valuation, err := fromValuationRow(row)
		if err != nil {
			return nil, err
		}
		valuations = append(valuations, valuation)
	}

	return valuations, nil
}

// AddValuation records a new dated valuation for assetID. It returns
// ErrAssetNotFound if no such asset exists.
func (a *Assets) AddValuation(ctx context.Context, assetID, date string, amount int64) (AssetValuation, error) {
	if _, err := a.q.GetAsset(ctx, assetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AssetValuation{}, ErrAssetNotFound
		}
		return AssetValuation{}, fmt.Errorf("getting asset: %w", err)
	}

	row, err := a.q.CreateAssetValuation(ctx, sqlcgen.CreateAssetValuationParams{
		ID:        uuid.NewString(),
		AssetID:   assetID,
		Date:      date,
		Amount:    amount,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return AssetValuation{}, fmt.Errorf("creating asset valuation: %w", err)
	}

	return fromValuationRow(row)
}

// DeleteValuation removes the valuation with the given ID. It returns
// ErrAssetValuationNotFound if no such valuation exists. Unlike assets,
// there's no cascade to worry about - a valuation has no children.
func (a *Assets) DeleteValuation(ctx context.Context, valuationID string) error {
	affected, err := a.q.DeleteAssetValuation(ctx, valuationID)
	if err != nil {
		return fmt.Errorf("deleting asset valuation: %w", err)
	}
	if affected == 0 {
		return ErrAssetValuationNotFound
	}

	return nil
}

func createOwners(ctx context.Context, q *sqlcgen.Queries, assetID string, owners []OwnerInput, now string) error {
	for _, owner := range owners {
		if _, err := q.CreateAssetOwner(ctx, sqlcgen.CreateAssetOwnerParams{
			ID:         uuid.NewString(),
			AssetID:    assetID,
			UserID:     owner.UserID,
			Percentage: int64(owner.Percentage),
			CreatedAt:  now,
		}); err != nil {
			return fmt.Errorf("creating asset owner: %w", err)
		}
	}
	return nil
}

func createExposures(ctx context.Context, q *sqlcgen.Queries, assetID string, exposures []ExposureInput, now string) error {
	for _, exposure := range exposures {
		if _, err := q.CreateAssetExposure(ctx, sqlcgen.CreateAssetExposureParams{
			ID:         uuid.NewString(),
			AssetID:    assetID,
			Region:     string(exposure.Region),
			Percentage: int64(exposure.Percentage),
			CreatedAt:  now,
		}); err != nil {
			return fmt.Errorf("creating asset exposure: %w", err)
		}
	}
	return nil
}

// assemble builds an Asset from an already-fetched assets row plus its
// owners, exposures, and latest valuation - each queried separately since
// sqlc's SQLite mode has no way to return a nested/aggregated result (see
// this package's doc comment on why this mirrors the workouts app's
// cycleservice assembly pattern). q is passed in rather than using a.q
// directly so this can run against either the plain repository or a
// transaction-scoped Queries.
func (a *Assets) assemble(ctx context.Context, q *sqlcgen.Queries, row sqlcgen.Asset) (Asset, error) {
	asset, err := fromAssetRow(row)
	if err != nil {
		return Asset{}, err
	}

	ownerRows, err := q.ListAssetOwnersByAsset(ctx, row.ID)
	if err != nil {
		return Asset{}, fmt.Errorf("listing asset owners: %w", err)
	}
	owners := make([]AssetOwner, 0, len(ownerRows))
	for _, ownerRow := range ownerRows {
		owner, err := fromOwnerRow(ownerRow)
		if err != nil {
			return Asset{}, err
		}
		owners = append(owners, owner)
	}
	asset.Owners = owners

	exposureRows, err := q.ListAssetExposuresByAsset(ctx, row.ID)
	if err != nil {
		return Asset{}, fmt.Errorf("listing asset exposures: %w", err)
	}
	exposures := make([]AssetExposure, 0, len(exposureRows))
	for _, exposureRow := range exposureRows {
		exposure, err := fromExposureRow(exposureRow)
		if err != nil {
			return Asset{}, err
		}
		exposures = append(exposures, exposure)
	}
	asset.Exposures = exposures

	latestRow, err := q.GetLatestAssetValuation(ctx, row.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return Asset{}, fmt.Errorf("getting latest asset valuation: %w", err)
		}
	} else {
		latest, err := fromValuationRow(latestRow)
		if err != nil {
			return Asset{}, err
		}
		asset.LatestValuation = &latest
	}

	return asset, nil
}

func fromAssetRow(row sqlcgen.Asset) (Asset, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return Asset{}, fmt.Errorf("parsing asset created_at: %w", err)
	}

	return Asset{
		ID:       row.ID,
		Category: AssetCategory(row.Category),
		Currency: Currency(row.Currency),
		Details: AssetDetails{
			CompanyOrBondName: stringPtr(row.CompanyOrBondName),
			Platform:          stringPtr(row.Platform),
			AccountName:       stringPtr(row.AccountName),
			Institution:       stringPtr(row.Institution),
			EtfName:           stringPtr(row.EtfName),
			SecurityType:      securityTypePtr(row.SecurityType),
			PlanName:          stringPtr(row.PlanName),
			PropertyName:      stringPtr(row.PropertyName),
			Notes:             stringPtr(row.Notes),
		},
		IsLiquid:  row.IsLiquid != 0,
		CreatedAt: createdAt,
	}, nil
}

func fromOwnerRow(row sqlcgen.ListAssetOwnersByAssetRow) (AssetOwner, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return AssetOwner{}, fmt.Errorf("parsing asset owner created_at: %w", err)
	}

	return AssetOwner{
		ID:         row.ID,
		AssetID:    row.AssetID,
		UserID:     row.UserID,
		UserName:   row.UserName,
		Percentage: int(row.Percentage),
		CreatedAt:  createdAt,
	}, nil
}

func fromExposureRow(row sqlcgen.AssetExposure) (AssetExposure, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return AssetExposure{}, fmt.Errorf("parsing asset exposure created_at: %w", err)
	}

	return AssetExposure{
		ID:         row.ID,
		AssetID:    row.AssetID,
		Region:     Region(row.Region),
		Percentage: int(row.Percentage),
		CreatedAt:  createdAt,
	}, nil
}

func fromValuationRow(row sqlcgen.AssetValuation) (AssetValuation, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return AssetValuation{}, fmt.Errorf("parsing asset valuation created_at: %w", err)
	}

	return AssetValuation{
		ID:        row.ID,
		AssetID:   row.AssetID,
		Date:      row.Date,
		Amount:    row.Amount,
		CreatedAt: createdAt,
	}, nil
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func stringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func nullableSecurityType(st *SecurityType) sql.NullString {
	if st == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*st), Valid: true}
}

func securityTypePtr(ns sql.NullString) *SecurityType {
	if !ns.Valid {
		return nil
	}
	st := SecurityType(ns.String)
	return &st
}
