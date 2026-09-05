package assetservice

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/liamawhite/finances/gen/finances/v1"
	"github.com/liamawhite/finances/internal/storage"
)

// newTestService returns a Service over a fresh, migrated temp-file DB,
// plus the two seeded users' IDs for tests that need to reference a real
// user_id.
func newTestService(t *testing.T) (*Service, []storage.User) {
	t.Helper()

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("storage.Migrate() error = %v", err)
	}

	users, err := storage.NewUsers(db).List(context.Background())
	if err != nil {
		t.Fatalf("listing seeded users: %v", err)
	}

	return New(storage.NewAssets(db)), users
}

func cashCreateRequest(owners []*v1.OwnerShare, exposures []*v1.ExposureShare) *v1.CreateAssetRequest {
	return &v1.CreateAssetRequest{
		Category:               v1.AssetCategory_ASSET_CATEGORY_CASH,
		Currency:               v1.Currency_CURRENCY_USD,
		Owners:                 owners,
		Exposures:              exposures,
		InitialValuationDate:   "2026-01-01",
		InitialValuationAmount: 100000,
		Liquidity:              v1.AssetLiquidity_ASSET_LIQUIDITY_LIQUID,
		Details: &v1.CreateAssetRequest_Cash{Cash: &v1.CashDetails{
			AccountName: "Checking",
			Institution: "Chase",
		}},
	}
}

func TestService_CreateAsset_RejectsOwnersNotSummingTo100(t *testing.T) {
	svc, users := newTestService(t)

	req := cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 60}, {UserId: users[1].ID, Percentage: 30}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)
	_, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateAsset() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateAsset_RejectsExposuresNotSummingTo100(t *testing.T) {
	svc, users := newTestService(t)

	req := cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 50}, {Region: v1.Region_REGION_UK, Percentage: 40}},
	)
	_, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateAsset() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateAsset_AcceptsSingleOwnerAt100(t *testing.T) {
	svc, users := newTestService(t)

	req := cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)
	resp, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if len(resp.Msg.GetAsset().GetOwners()) != 1 || resp.Msg.GetAsset().GetOwners()[0].GetPercentage() != 100 {
		t.Fatalf("CreateAsset() owners = %+v, want one owner at 100%%", resp.Msg.GetAsset().GetOwners())
	}
}

func TestService_CreateAsset_RejectsCategoryDetailsMismatch(t *testing.T) {
	svc, users := newTestService(t)

	req := &v1.CreateAssetRequest{
		Category:               v1.AssetCategory_ASSET_CATEGORY_CASH,
		Currency:               v1.Currency_CURRENCY_USD,
		Owners:                 []*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		Exposures:              []*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
		InitialValuationDate:   "2026-01-01",
		InitialValuationAmount: 100000,
		Liquidity:              v1.AssetLiquidity_ASSET_LIQUIDITY_LIQUID,
		Details: &v1.CreateAssetRequest_StockOrBond{StockOrBond: &v1.StockOrBondDetails{
			CompanyOrBondName: "Apple Inc.",
			Platform:          "Vanguard",
		}},
	}
	_, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateAsset() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateAsset_CreatesInitialValuation(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	req := cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)
	created, err := svc.CreateAsset(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	latest := created.Msg.GetAsset().GetLatestValuation()
	if latest == nil || latest.GetDate() != "2026-01-01" || latest.GetAmount() != 100000 {
		t.Fatalf("CreateAsset() latest valuation = %+v, want {2026-01-01, 100000}", latest)
	}

	listed, err := svc.ListAssetValuations(ctx, connect.NewRequest(&v1.ListAssetValuationsRequest{AssetId: created.Msg.GetAsset().GetId()}))
	if err != nil {
		t.Fatalf("ListAssetValuations() error = %v", err)
	}
	if len(listed.Msg.GetValuations()) != 1 {
		t.Fatalf("ListAssetValuations() = %d valuations, want 1", len(listed.Msg.GetValuations()))
	}
}

func TestService_UpdateAsset_ReplacesOwnersAndExposures(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	created, err := svc.CreateAsset(ctx, connect.NewRequest(cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	updated, err := svc.UpdateAsset(ctx, connect.NewRequest(&v1.UpdateAssetRequest{
		Id:       created.Msg.GetAsset().GetId(),
		Category: v1.AssetCategory_ASSET_CATEGORY_CASH,
		Currency: v1.Currency_CURRENCY_USD,
		Owners: []*v1.OwnerShare{
			{UserId: users[0].ID, Percentage: 50},
			{UserId: users[1].ID, Percentage: 50},
		},
		Exposures: []*v1.ExposureShare{{Region: v1.Region_REGION_UK, Percentage: 100}},
		Liquidity: v1.AssetLiquidity_ASSET_LIQUIDITY_LIQUID,
		Details: &v1.UpdateAssetRequest_Cash{Cash: &v1.CashDetails{
			AccountName: "Checking",
			Institution: "Chase",
		}},
	}))
	if err != nil {
		t.Fatalf("UpdateAsset() error = %v", err)
	}
	if len(updated.Msg.GetAsset().GetOwners()) != 2 {
		t.Fatalf("UpdateAsset() owners = %+v, want 2", updated.Msg.GetAsset().GetOwners())
	}
	if len(updated.Msg.GetAsset().GetExposures()) != 1 || updated.Msg.GetAsset().GetExposures()[0].GetRegion() != v1.Region_REGION_UK {
		t.Fatalf("UpdateAsset() regions = %+v, want one UK region", updated.Msg.GetAsset().GetExposures())
	}
}

func TestService_Liquidity_RoundTripsThroughCreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	req := cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)
	req.Liquidity = v1.AssetLiquidity_ASSET_LIQUIDITY_ILLIQUID
	created, err := svc.CreateAsset(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if created.Msg.GetAsset().GetLiquidity() != v1.AssetLiquidity_ASSET_LIQUIDITY_ILLIQUID {
		t.Fatalf("CreateAsset() liquidity = %v, want %v", created.Msg.GetAsset().GetLiquidity(), v1.AssetLiquidity_ASSET_LIQUIDITY_ILLIQUID)
	}

	updated, err := svc.UpdateAsset(ctx, connect.NewRequest(&v1.UpdateAssetRequest{
		Id:        created.Msg.GetAsset().GetId(),
		Category:  v1.AssetCategory_ASSET_CATEGORY_CASH,
		Currency:  v1.Currency_CURRENCY_USD,
		Owners:    []*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		Exposures: []*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
		Liquidity: v1.AssetLiquidity_ASSET_LIQUIDITY_INVESTED,
		Details: &v1.UpdateAssetRequest_Cash{Cash: &v1.CashDetails{
			AccountName: "Checking",
			Institution: "Chase",
		}},
	}))
	if err != nil {
		t.Fatalf("UpdateAsset() error = %v", err)
	}
	if updated.Msg.GetAsset().GetLiquidity() != v1.AssetLiquidity_ASSET_LIQUIDITY_INVESTED {
		t.Fatalf(
			"UpdateAsset() liquidity = %v, want %v (override should persist)",
			updated.Msg.GetAsset().GetLiquidity(), v1.AssetLiquidity_ASSET_LIQUIDITY_INVESTED,
		)
	}
}

func TestService_UpdateAsset_NotFound(t *testing.T) {
	svc, users := newTestService(t)

	_, err := svc.UpdateAsset(context.Background(), connect.NewRequest(&v1.UpdateAssetRequest{
		Id:        "does-not-exist",
		Category:  v1.AssetCategory_ASSET_CATEGORY_CASH,
		Currency:  v1.Currency_CURRENCY_USD,
		Owners:    []*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		Exposures: []*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
		Liquidity: v1.AssetLiquidity_ASSET_LIQUIDITY_LIQUID,
		Details: &v1.UpdateAssetRequest_Cash{Cash: &v1.CashDetails{
			AccountName: "Checking",
			Institution: "Chase",
		}},
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("UpdateAsset() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_DeleteAsset_ThenListAssets(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	created, err := svc.CreateAsset(ctx, connect.NewRequest(cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	if _, err := svc.DeleteAsset(ctx, connect.NewRequest(&v1.DeleteAssetRequest{Id: created.Msg.GetAsset().GetId()})); err != nil {
		t.Fatalf("DeleteAsset() error = %v", err)
	}

	listed, err := svc.ListAssets(ctx, connect.NewRequest(&v1.ListAssetsRequest{}))
	if err != nil {
		t.Fatalf("ListAssets() error = %v", err)
	}
	if len(listed.Msg.GetAssets()) != 0 {
		t.Fatalf("ListAssets() after delete = %d assets, want 0", len(listed.Msg.GetAssets()))
	}
}

func TestService_DeleteAsset_NotFound(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.DeleteAsset(context.Background(), connect.NewRequest(&v1.DeleteAssetRequest{Id: "does-not-exist"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteAsset() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestService_AddAssetValuation_ThenListAssetValuations_NewestFirst(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	created, err := svc.CreateAsset(ctx, connect.NewRequest(cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	assetID := created.Msg.GetAsset().GetId()

	if _, err := svc.AddAssetValuation(ctx, connect.NewRequest(&v1.AddAssetValuationRequest{
		AssetId: assetID, Date: "2026-06-01", Amount: 150000,
	})); err != nil {
		t.Fatalf("AddAssetValuation() error = %v", err)
	}

	listed, err := svc.ListAssetValuations(ctx, connect.NewRequest(&v1.ListAssetValuationsRequest{AssetId: assetID}))
	if err != nil {
		t.Fatalf("ListAssetValuations() error = %v", err)
	}
	if len(listed.Msg.GetValuations()) != 2 {
		t.Fatalf("ListAssetValuations() = %d valuations, want 2", len(listed.Msg.GetValuations()))
	}
	if listed.Msg.GetValuations()[0].GetAmount() != 150000 {
		t.Fatalf("ListAssetValuations()[0] amount = %d, want 150000 (newest first)", listed.Msg.GetValuations()[0].GetAmount())
	}
}

func TestService_AddAssetValuation_RejectsNegativeAmount(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	created, err := svc.CreateAsset(ctx, connect.NewRequest(cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	_, err = svc.AddAssetValuation(ctx, connect.NewRequest(&v1.AddAssetValuationRequest{
		AssetId: created.Msg.GetAsset().GetId(), Date: "2026-06-01", Amount: -1,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("AddAssetValuation() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateAsset_RetirementWithBondsRegion(t *testing.T) {
	svc, users := newTestService(t)

	req := &v1.CreateAssetRequest{
		Category: v1.AssetCategory_ASSET_CATEGORY_RETIREMENT,
		Currency: v1.Currency_CURRENCY_USD,
		Owners:   []*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		Exposures: []*v1.ExposureShare{
			{Region: v1.Region_REGION_US, Percentage: 55},
			{Region: v1.Region_REGION_INTERNATIONAL, Percentage: 35},
			{Region: v1.Region_REGION_BONDS, Percentage: 10},
		},
		InitialValuationDate:   "2025-12-31",
		InitialValuationAmount: 14290300,
		Liquidity:              v1.AssetLiquidity_ASSET_LIQUIDITY_ILLIQUID,
		Details: &v1.CreateAssetRequest_Retirement{Retirement: &v1.RetirementDetails{
			PlanName: "Tetrate 401k",
			Platform: "Empower",
		}},
	}
	resp, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if len(resp.Msg.GetAsset().GetExposures()) != 3 {
		t.Fatalf("CreateAsset() regions = %+v, want 3", resp.Msg.GetAsset().GetExposures())
	}
	if resp.Msg.GetAsset().GetRetirement().GetPlanName() != "Tetrate 401k" {
		t.Fatalf("CreateAsset() plan_name = %q, want %q", resp.Msg.GetAsset().GetRetirement().GetPlanName(), "Tetrate 401k")
	}
}

func TestService_CreateAsset_LoanWithNoExposuresAndBlankLender(t *testing.T) {
	svc, users := newTestService(t)

	req := &v1.CreateAssetRequest{
		Category:               v1.AssetCategory_ASSET_CATEGORY_LOAN,
		Currency:               v1.Currency_CURRENCY_USD,
		Owners:                 []*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		InitialValuationDate:   "2025-12-31",
		InitialValuationAmount: 13000000,
		Liquidity:              v1.AssetLiquidity_ASSET_LIQUIDITY_INVESTED,
		Details: &v1.CreateAssetRequest_Loan{Loan: &v1.LoanDetails{
			Name:   "Personal Loan",
			Lender: "",
		}},
	}
	resp, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if len(resp.Msg.GetAsset().GetExposures()) != 0 {
		t.Fatalf("CreateAsset() regions = %+v, want none", resp.Msg.GetAsset().GetExposures())
	}
	if resp.Msg.GetAsset().GetLoan().GetLender() != "" {
		t.Fatalf("CreateAsset() lender = %q, want blank", resp.Msg.GetAsset().GetLoan().GetLender())
	}
}

func TestService_CreateAsset_CashWithNoExposuresStillSucceeds(t *testing.T) {
	svc, users := newTestService(t)

	req := cashCreateRequest([]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}}, nil)
	resp, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if len(resp.Msg.GetAsset().GetExposures()) != 0 {
		t.Fatalf("CreateAsset() regions = %+v, want none", resp.Msg.GetAsset().GetExposures())
	}
}

func TestService_CreateAsset_RejectsExposuresProvidedButNotSummingTo100(t *testing.T) {
	svc, users := newTestService(t)

	req := cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 50}},
	)
	_, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateAsset() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_CreateAsset_RejectsZeroOwnersEvenWithNoExposures(t *testing.T) {
	svc, _ := newTestService(t)

	req := cashCreateRequest(nil, nil)
	_, err := svc.CreateAsset(context.Background(), connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateAsset() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}

func TestService_DeleteAssetValuation_ThenListAssetValuations(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	created, err := svc.CreateAsset(ctx, connect.NewRequest(cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	assetID := created.Msg.GetAsset().GetId()

	added, err := svc.AddAssetValuation(ctx, connect.NewRequest(&v1.AddAssetValuationRequest{
		AssetId: assetID, Date: "2026-06-01", Amount: 150000,
	}))
	if err != nil {
		t.Fatalf("AddAssetValuation() error = %v", err)
	}

	if _, err := svc.DeleteAssetValuation(ctx, connect.NewRequest(&v1.DeleteAssetValuationRequest{
		ValuationId: added.Msg.GetValuation().GetId(),
	})); err != nil {
		t.Fatalf("DeleteAssetValuation() error = %v", err)
	}

	listed, err := svc.ListAssetValuations(ctx, connect.NewRequest(&v1.ListAssetValuationsRequest{AssetId: assetID}))
	if err != nil {
		t.Fatalf("ListAssetValuations() error = %v", err)
	}
	if len(listed.Msg.GetValuations()) != 1 {
		t.Fatalf("ListAssetValuations() after delete = %d valuations, want 1", len(listed.Msg.GetValuations()))
	}
}

func TestService_DeleteAssetValuation_NotFound(t *testing.T) {
	svc, _ := newTestService(t)

	_, err := svc.DeleteAssetValuation(context.Background(), connect.NewRequest(&v1.DeleteAssetValuationRequest{
		ValuationId: "does-not-exist",
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("DeleteAssetValuation() code = %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func capitalBalanceCreateRequest(owners []*v1.OwnerShare, initialAmount int64) *v1.CreateAssetRequest {
	return &v1.CreateAssetRequest{
		Category:               v1.AssetCategory_ASSET_CATEGORY_CAPITAL_BALANCE,
		Currency:               v1.Currency_CURRENCY_USD,
		Owners:                 owners,
		InitialValuationDate:   "2026-01-01",
		InitialValuationAmount: initialAmount,
		Liquidity:              v1.AssetLiquidity_ASSET_LIQUIDITY_INVESTED,
		Details: &v1.CreateAssetRequest_CapitalBalance{CapitalBalance: &v1.CapitalBalanceDetails{
			Name: "Shared Capital",
		}},
	}
}

func TestService_CreateAsset_CapitalBalance_AllowsNegativeInitialValuation(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	resp, err := svc.CreateAsset(ctx, connect.NewRequest(capitalBalanceCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}}, -13000,
	)))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if got := resp.Msg.GetAsset().GetLatestValuation().GetAmount(); got != -13000 {
		t.Fatalf("CreateAsset() latest valuation amount = %d, want -13000", got)
	}
	if got := resp.Msg.GetAsset().GetCapitalBalance().GetName(); got != "Shared Capital" {
		t.Fatalf("CreateAsset() capital_balance.name = %q, want %q", got, "Shared Capital")
	}
}

func TestService_AddAssetValuation_AllowsNegativeForCapitalBalance(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	created, err := svc.CreateAsset(ctx, connect.NewRequest(capitalBalanceCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}}, 0,
	)))
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}

	resp, err := svc.AddAssetValuation(ctx, connect.NewRequest(&v1.AddAssetValuationRequest{
		AssetId: created.Msg.GetAsset().GetId(), Date: "2026-06-01", Amount: -5000,
	}))
	if err != nil {
		t.Fatalf("AddAssetValuation() error = %v", err)
	}
	if got := resp.Msg.GetValuation().GetAmount(); got != -5000 {
		t.Fatalf("AddAssetValuation() amount = %d, want -5000", got)
	}
}

func TestService_CreateAsset_RejectsNegativeInitialValuation_ForNonCapitalBalanceCategory(t *testing.T) {
	ctx := context.Background()
	svc, users := newTestService(t)

	req := cashCreateRequest(
		[]*v1.OwnerShare{{UserId: users[0].ID, Percentage: 100}},
		[]*v1.ExposureShare{{Region: v1.Region_REGION_US, Percentage: 100}},
	)
	req.InitialValuationAmount = -1

	_, err := svc.CreateAsset(ctx, connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateAsset() code = %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
	}
}
