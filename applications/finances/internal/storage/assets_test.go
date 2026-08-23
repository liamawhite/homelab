package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openAssetsTestDB(t *testing.T) (*Assets, *Users) {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return NewAssets(db), NewUsers(db)
}

func strPtr(s string) *string { return &s }

func TestAssets_CreateAndGet_StockOrBond(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}
	owner := userList[0]

	details := AssetDetails{
		CompanyOrBondName: strPtr("Apple Inc."),
		Platform:          strPtr("Vanguard"),
	}
	created, err := assets.Create(ctx, AssetCategoryStockOrBond, CurrencyUSD, details,
		true,
		[]OwnerInput{{UserID: owner.ID, Percentage: 100}},
		[]ExposureInput{{Region: RegionUS, Percentage: 100}},
		"2026-01-01", 10000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("Create() returned empty ID")
	}
	if created.Category != AssetCategoryStockOrBond {
		t.Fatalf("Create() category = %q, want %q", created.Category, AssetCategoryStockOrBond)
	}
	if created.Details.CompanyOrBondName == nil || *created.Details.CompanyOrBondName != "Apple Inc." {
		t.Fatalf("Create() company_or_bond_name = %v, want %q", created.Details.CompanyOrBondName, "Apple Inc.")
	}
	if len(created.Owners) != 1 || created.Owners[0].Percentage != 100 {
		t.Fatalf("Create() owners = %+v, want one owner at 100%%", created.Owners)
	}
	if len(created.Exposures) != 1 || created.Exposures[0].Region != RegionUS {
		t.Fatalf("Create() regions = %+v, want one US region", created.Exposures)
	}
	if created.LatestValuation == nil || created.LatestValuation.Amount != 10000 || created.LatestValuation.Date != "2026-01-01" {
		t.Fatalf("Create() latest valuation = %+v, want {Date: 2026-01-01, Amount: 10000}", created.LatestValuation)
	}

	got, err := assets.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("Get() ID = %q, want %q", got.ID, created.ID)
	}
}

func TestAssets_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	assets, _ := openAssetsTestDB(t)

	_, err := assets.Get(ctx, "does-not-exist")
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("Get() error = %v, want ErrAssetNotFound", err)
	}
}

func TestAssets_Create_SplitOwnership(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}
	if len(userList) < 2 {
		t.Fatalf("users.List() = %d users, want at least 2 seeded users", len(userList))
	}

	created, err := assets.Create(ctx, AssetCategoryCash, CurrencyGBP,
		AssetDetails{AccountName: strPtr("Joint Savings"), Institution: strPtr("Monzo")},
		true,
		[]OwnerInput{
			{UserID: userList[0].ID, Percentage: 60},
			{UserID: userList[1].ID, Percentage: 40},
		},
		[]ExposureInput{{Region: RegionUK, Percentage: 100}},
		"2026-01-01", 500000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.Owners) != 2 {
		t.Fatalf("Create() owners = %+v, want 2 owners", created.Owners)
	}

	total := 0
	for _, owner := range created.Owners {
		total += owner.Percentage
		if owner.UserName == "" {
			t.Fatalf("Create() owner %+v has empty UserName", owner)
		}
	}
	if total != 100 {
		t.Fatalf("Create() owner percentages sum to %d, want 100", total)
	}
}

func TestAssets_Update_ReplacesOwnersAndExposures(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryEtf, CurrencyUSD,
		AssetDetails{EtfName: strPtr("VOO"), SecurityType: securityTypePtrHelper(SecurityTypeStock), Platform: strPtr("Vanguard")},
		true,
		[]OwnerInput{{UserID: userList[0].ID, Percentage: 100}},
		[]ExposureInput{{Region: RegionUS, Percentage: 100}},
		"2026-01-01", 10000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := assets.Update(ctx, created.ID, AssetCategoryEtf, CurrencyUSD,
		AssetDetails{EtfName: strPtr("VXUS"), SecurityType: securityTypePtrHelper(SecurityTypeStock), Platform: strPtr("Vanguard")},
		true,
		[]OwnerInput{
			{UserID: userList[0].ID, Percentage: 50},
			{UserID: userList[1].ID, Percentage: 50},
		},
		[]ExposureInput{{Region: RegionInternational, Percentage: 100}},
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Details.EtfName == nil || *updated.Details.EtfName != "VXUS" {
		t.Fatalf("Update() etf_name = %v, want %q", updated.Details.EtfName, "VXUS")
	}
	if len(updated.Owners) != 2 {
		t.Fatalf("Update() owners = %+v, want 2 owners", updated.Owners)
	}
	if len(updated.Exposures) != 1 || updated.Exposures[0].Region != RegionInternational {
		t.Fatalf("Update() regions = %+v, want one International region", updated.Exposures)
	}

	// Update must not touch valuations.
	valuations, err := assets.ListValuations(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListValuations() error = %v", err)
	}
	if len(valuations) != 1 {
		t.Fatalf("ListValuations() after Update = %d valuations, want 1 (untouched)", len(valuations))
	}
}

func TestAssets_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	assets, _ := openAssetsTestDB(t)

	_, err := assets.Update(ctx, "does-not-exist", AssetCategoryCash, CurrencyUSD,
		AssetDetails{AccountName: strPtr("x"), Institution: strPtr("y")}, true, nil, nil)
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("Update() error = %v, want ErrAssetNotFound", err)
	}
}

func TestAssets_Delete_CascadesChildren(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryCash, CurrencyUSD,
		AssetDetails{AccountName: strPtr("Checking"), Institution: strPtr("Chase")},
		true,
		[]OwnerInput{{UserID: userList[0].ID, Percentage: 100}},
		[]ExposureInput{{Region: RegionUS, Percentage: 100}},
		"2026-01-01", 100000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := assets.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := assets.Get(ctx, created.ID); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrAssetNotFound", err)
	}
	if _, err := assets.ListValuations(ctx, created.ID); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("ListValuations() after delete error = %v, want ErrAssetNotFound", err)
	}
}

func TestAssets_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	assets, _ := openAssetsTestDB(t)

	err := assets.Delete(ctx, "does-not-exist")
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("Delete() error = %v, want ErrAssetNotFound", err)
	}
}

func TestAssets_AddValuation_ThenListValuations_NewestFirst(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryCash, CurrencyUSD,
		AssetDetails{AccountName: strPtr("Savings"), Institution: strPtr("Chase")},
		true,
		[]OwnerInput{{UserID: userList[0].ID, Percentage: 100}},
		[]ExposureInput{{Region: RegionUS, Percentage: 100}},
		"2026-01-01", 100000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := assets.AddValuation(ctx, created.ID, "2026-06-01", 150000); err != nil {
		t.Fatalf("AddValuation() error = %v", err)
	}

	valuations, err := assets.ListValuations(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListValuations() error = %v", err)
	}
	if len(valuations) != 2 {
		t.Fatalf("ListValuations() = %d valuations, want 2", len(valuations))
	}
	if valuations[0].Date != "2026-06-01" || valuations[0].Amount != 150000 {
		t.Fatalf("ListValuations()[0] = %+v, want newest (2026-06-01, 150000) first", valuations[0])
	}

	got, err := assets.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.LatestValuation == nil || got.LatestValuation.Amount != 150000 {
		t.Fatalf("Get() latest valuation = %+v, want amount 150000", got.LatestValuation)
	}
}

func TestAssets_AddValuation_NotFound(t *testing.T) {
	ctx := context.Background()
	assets, _ := openAssetsTestDB(t)

	_, err := assets.AddValuation(ctx, "does-not-exist", "2026-01-01", 100)
	if !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("AddValuation() error = %v, want ErrAssetNotFound", err)
	}
}

func securityTypePtrHelper(st SecurityType) *SecurityType { return &st }

func TestAssets_Create_RetirementWithBondsRegion(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryRetirement, CurrencyUSD,
		AssetDetails{PlanName: strPtr("Tetrate 401k"), Platform: strPtr("Empower")},
		true,
		[]OwnerInput{{UserID: userList[0].ID, Percentage: 100}},
		[]ExposureInput{
			{Region: RegionUS, Percentage: 55},
			{Region: RegionInternational, Percentage: 35},
			{Region: RegionBonds, Percentage: 10},
		},
		"2025-12-31", 14290300,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Details.PlanName == nil || *created.Details.PlanName != "Tetrate 401k" {
		t.Fatalf("Create() plan_name = %v, want %q", created.Details.PlanName, "Tetrate 401k")
	}
	if len(created.Exposures) != 3 {
		t.Fatalf("Create() regions = %+v, want 3 rows", created.Exposures)
	}
	total := 0
	for _, exposure := range created.Exposures {
		total += exposure.Percentage
	}
	if total != 100 {
		t.Fatalf("Create() region percentages sum to %d, want 100", total)
	}
}

func TestAssets_Create_LoanWithNoExposures(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryLoan, CurrencyUSD,
		AssetDetails{AccountName: strPtr("Personal Loan")},
		true,
		[]OwnerInput{{UserID: userList[0].ID, Percentage: 100}},
		nil,
		"2025-12-31", 13000000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.Exposures) != 0 {
		t.Fatalf("Create() regions = %+v, want none", created.Exposures)
	}
}

func TestAssets_Create_RealEstateSplitOwnership(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryRealEstate, CurrencyUSD,
		AssetDetails{PropertyName: strPtr("House"), Notes: strPtr("Adjusted for realtor fees")},
		true,
		[]OwnerInput{
			{UserID: userList[0].ID, Percentage: 50},
			{UserID: userList[1].ID, Percentage: 50},
		},
		[]ExposureInput{{Region: RegionUS, Percentage: 100}},
		"2025-12-31", 127400000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Details.PropertyName == nil || *created.Details.PropertyName != "House" {
		t.Fatalf("Create() property_name = %v, want %q", created.Details.PropertyName, "House")
	}
	if created.Details.Notes == nil || *created.Details.Notes != "Adjusted for realtor fees" {
		t.Fatalf("Create() notes = %v, want %q", created.Details.Notes, "Adjusted for realtor fees")
	}
	if len(created.Owners) != 2 {
		t.Fatalf("Create() owners = %+v, want 2", created.Owners)
	}
}

func TestAssets_Create_MortgageWithBlankLender(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryMortgage, CurrencyUSD,
		AssetDetails{PropertyName: strPtr("House"), Institution: strPtr("PNC")},
		true,
		[]OwnerInput{
			{UserID: userList[0].ID, Percentage: 50},
			{UserID: userList[1].ID, Percentage: 50},
		},
		nil,
		"2025-12-31", 49764700,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.Exposures) != 0 {
		t.Fatalf("Create() regions = %+v, want none", created.Exposures)
	}
	if created.Details.Institution == nil || *created.Details.Institution != "PNC" {
		t.Fatalf("Create() institution = %v, want %q", created.Details.Institution, "PNC")
	}
}

func TestAssets_DeleteValuation(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryCash, CurrencyUSD,
		AssetDetails{AccountName: strPtr("Savings"), Institution: strPtr("Chase")},
		true,
		[]OwnerInput{{UserID: userList[0].ID, Percentage: 100}},
		nil,
		"2026-01-01", 100000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	added, err := assets.AddValuation(ctx, created.ID, "2026-06-01", 200000)
	if err != nil {
		t.Fatalf("AddValuation() error = %v", err)
	}

	if err := assets.DeleteValuation(ctx, added.ID); err != nil {
		t.Fatalf("DeleteValuation() error = %v", err)
	}

	valuations, err := assets.ListValuations(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListValuations() error = %v", err)
	}
	if len(valuations) != 1 {
		t.Fatalf("ListValuations() after DeleteValuation = %d valuations, want 1 (the initial one)", len(valuations))
	}
	if valuations[0].Amount != 100000 {
		t.Fatalf("ListValuations()[0].Amount = %d, want 100000 (the surviving initial valuation)", valuations[0].Amount)
	}
}

func TestAssets_IsLiquid_RoundTripsThroughCreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	assets, users := openAssetsTestDB(t)

	userList, err := users.List(ctx)
	if err != nil {
		t.Fatalf("users.List() error = %v", err)
	}

	created, err := assets.Create(ctx, AssetCategoryRealEstate, CurrencyUSD,
		AssetDetails{PropertyName: strPtr("House"), Notes: strPtr("")},
		false,
		[]OwnerInput{{UserID: userList[0].ID, Percentage: 100}},
		[]ExposureInput{{Region: RegionUS, Percentage: 100}},
		"2026-01-01", 100000,
	)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.IsLiquid {
		t.Fatalf("Create() IsLiquid = true, want false")
	}

	got, err := assets.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.IsLiquid {
		t.Fatalf("Get() IsLiquid = true, want false")
	}

	updated, err := assets.Update(ctx, created.ID, AssetCategoryRealEstate, CurrencyUSD,
		AssetDetails{PropertyName: strPtr("House"), Notes: strPtr("")}, true,
		[]OwnerInput{{UserID: userList[0].ID, Percentage: 100}},
		[]ExposureInput{{Region: RegionUS, Percentage: 100}},
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !updated.IsLiquid {
		t.Fatalf("Update() IsLiquid = false, want true (override should persist)")
	}
}

func TestAssets_DeleteValuation_NotFound(t *testing.T) {
	ctx := context.Background()
	assets, _ := openAssetsTestDB(t)

	err := assets.DeleteValuation(ctx, "does-not-exist")
	if !errors.Is(err, ErrAssetValuationNotFound) {
		t.Fatalf("DeleteValuation() error = %v, want ErrAssetValuationNotFound", err)
	}
}
