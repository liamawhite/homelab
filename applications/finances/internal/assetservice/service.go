// Package assetservice implements the finances.v1.AssetService Connect
// handler over the storage.Assets repository.
package assetservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/liamawhite/finances/gen/finances/v1"
	"github.com/liamawhite/finances/internal/storage"
)

// Service implements financesv1connect.AssetServiceHandler.
type Service struct {
	assets *storage.Assets
}

// New returns a Service backed by assets.
func New(assets *storage.Assets) *Service {
	return &Service{assets: assets}
}

// ListAssets returns every asset.
func (s *Service) ListAssets(ctx context.Context, req *connect.Request[v1.ListAssetsRequest]) (*connect.Response[v1.ListAssetsResponse], error) {
	assets, err := s.assets.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListAssetsResponse{
		Assets: make([]*v1.Asset, 0, len(assets)),
	}
	for _, asset := range assets {
		resp.Assets = append(resp.Assets, toProto(asset))
	}

	return connect.NewResponse(resp), nil
}

// CreateAsset creates a new asset along with its ownership split,
// exposure breakdown, and an initial valuation.
func (s *Service) CreateAsset(ctx context.Context, req *connect.Request[v1.CreateAssetRequest]) (*connect.Response[v1.CreateAssetResponse], error) {
	category, err := fromProtoCategory(req.Msg.GetCategory())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	currency, err := fromProtoCurrency(req.Msg.GetCurrency())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	details, err := detailsFromCreateRequest(category, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	owners, err := ownersFromProto(req.Msg.GetOwners())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	exposures, err := exposuresFromProto(req.Msg.GetExposures())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	date := strings.TrimSpace(req.Msg.GetInitialValuationDate())
	if err := validateDate(date); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	amount := req.Msg.GetInitialValuationAmount()
	if amount < 0 && category != storage.AssetCategoryCapitalBalance {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("initial_valuation_amount must not be negative"))
	}

	asset, err := s.assets.Create(ctx, category, currency, details, req.Msg.GetIsLiquid(), owners, exposures, date, amount)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.CreateAssetResponse{Asset: toProto(asset)}), nil
}

// UpdateAsset replaces an asset's descriptive fields, ownership split, and
// exposure breakdown. It never touches the asset's valuations.
func (s *Service) UpdateAsset(ctx context.Context, req *connect.Request[v1.UpdateAssetRequest]) (*connect.Response[v1.UpdateAssetResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	category, err := fromProtoCategory(req.Msg.GetCategory())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	currency, err := fromProtoCurrency(req.Msg.GetCurrency())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	details, err := detailsFromUpdateRequest(category, req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	owners, err := ownersFromProto(req.Msg.GetOwners())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	exposures, err := exposuresFromProto(req.Msg.GetExposures())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	asset, err := s.assets.Update(ctx, id, category, currency, details, req.Msg.GetIsLiquid(), owners, exposures)
	if err != nil {
		if errors.Is(err, storage.ErrAssetNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.UpdateAssetResponse{Asset: toProto(asset)}), nil
}

// DeleteAsset removes an asset and its owners, exposures, and valuations.
func (s *Service) DeleteAsset(ctx context.Context, req *connect.Request[v1.DeleteAssetRequest]) (*connect.Response[v1.DeleteAssetResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id must not be empty"))
	}

	if err := s.assets.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrAssetNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteAssetResponse{}), nil
}

// ListAssetValuations returns every valuation for an asset, newest first.
func (s *Service) ListAssetValuations(ctx context.Context, req *connect.Request[v1.ListAssetValuationsRequest]) (*connect.Response[v1.ListAssetValuationsResponse], error) {
	assetID := strings.TrimSpace(req.Msg.GetAssetId())
	if assetID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("asset_id must not be empty"))
	}

	valuations, err := s.assets.ListValuations(ctx, assetID)
	if err != nil {
		if errors.Is(err, storage.ErrAssetNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &v1.ListAssetValuationsResponse{
		Valuations: make([]*v1.AssetValuation, 0, len(valuations)),
	}
	for _, valuation := range valuations {
		resp.Valuations = append(resp.Valuations, toProtoValuation(valuation))
	}

	return connect.NewResponse(resp), nil
}

// AddAssetValuation records a new dated valuation for an asset.
func (s *Service) AddAssetValuation(ctx context.Context, req *connect.Request[v1.AddAssetValuationRequest]) (*connect.Response[v1.AddAssetValuationResponse], error) {
	assetID := strings.TrimSpace(req.Msg.GetAssetId())
	if assetID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("asset_id must not be empty"))
	}

	date := strings.TrimSpace(req.Msg.GetDate())
	if err := validateDate(date); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	amount := req.Msg.GetAmount()
	if amount < 0 {
		// Only a capital-balance tracker's valuations may be negative (see
		// AssetValuation's doc comment in asset.proto) - the request only
		// carries the asset ID, so its category has to be looked up before
		// deciding whether to allow this.
		asset, err := s.assets.Get(ctx, assetID)
		if err != nil {
			if errors.Is(err, storage.ErrAssetNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if asset.Category != storage.AssetCategoryCapitalBalance {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("amount must not be negative"))
		}
	}

	valuation, err := s.assets.AddValuation(ctx, assetID, date, amount)
	if err != nil {
		if errors.Is(err, storage.ErrAssetNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.AddAssetValuationResponse{Valuation: toProtoValuation(valuation)}), nil
}

// DeleteAssetValuation removes a single valuation - for correcting a
// mis-entered value, not for normal use.
func (s *Service) DeleteAssetValuation(ctx context.Context, req *connect.Request[v1.DeleteAssetValuationRequest]) (*connect.Response[v1.DeleteAssetValuationResponse], error) {
	valuationID := strings.TrimSpace(req.Msg.GetValuationId())
	if valuationID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valuation_id must not be empty"))
	}

	if err := s.assets.DeleteValuation(ctx, valuationID); err != nil {
		if errors.Is(err, storage.ErrAssetValuationNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&v1.DeleteAssetValuationResponse{}), nil
}

// percentagesSumTo100 validates a set of ownership or geographic shares:
// non-empty, every share strictly positive, and the total exactly 100 -
// SQLite can't express a cross-row sum, so this is the only place it's
// enforced.
func percentagesSumTo100(what string, percentages []int32) error {
	if len(percentages) == 0 {
		return fmt.Errorf("%s must not be empty", what)
	}

	total := int32(0)
	for _, p := range percentages {
		if p <= 0 {
			return fmt.Errorf("%s percentages must be greater than 0", what)
		}
		total += p
	}
	if total != 100 {
		return fmt.Errorf("%s percentages must sum to 100, got %d", what, total)
	}

	return nil
}

func ownersFromProto(shares []*v1.OwnerShare) ([]storage.OwnerInput, error) {
	percentages := make([]int32, 0, len(shares))
	seen := make(map[string]bool, len(shares))
	owners := make([]storage.OwnerInput, 0, len(shares))
	for _, share := range shares {
		userID := strings.TrimSpace(share.GetUserId())
		if userID == "" {
			return nil, errors.New("owners[].user_id must not be empty")
		}
		if seen[userID] {
			return nil, fmt.Errorf("owners must not repeat user_id %q", userID)
		}
		seen[userID] = true

		percentages = append(percentages, share.GetPercentage())
		owners = append(owners, storage.OwnerInput{UserID: userID, Percentage: int(share.GetPercentage())})
	}

	if err := percentagesSumTo100("owners", percentages); err != nil {
		return nil, err
	}

	return owners, nil
}

// exposuresFromProto is deliberately more permissive than ownersFromProto:
// an asset's exposure breakdown is optional (commodities, crypto, and
// liabilities have no meaningful geography), so a zero-length input is
// valid and skips the sum-to-100 check entirely. If any exposures are
// given, they must still sum to exactly 100.
func exposuresFromProto(shares []*v1.ExposureShare) ([]storage.ExposureInput, error) {
	if len(shares) == 0 {
		return nil, nil
	}

	percentages := make([]int32, 0, len(shares))
	seen := make(map[v1.Region]bool, len(shares))
	exposures := make([]storage.ExposureInput, 0, len(shares))
	for _, share := range shares {
		region, err := fromProtoRegion(share.GetRegion())
		if err != nil {
			return nil, err
		}
		if seen[share.GetRegion()] {
			return nil, fmt.Errorf("exposures must not repeat region %q", share.GetRegion())
		}
		seen[share.GetRegion()] = true

		percentages = append(percentages, share.GetPercentage())
		exposures = append(exposures, storage.ExposureInput{Region: region, Percentage: int(share.GetPercentage())})
	}

	if err := percentagesSumTo100("exposures", percentages); err != nil {
		return nil, err
	}

	return exposures, nil
}

func validateDate(date string) error {
	if date == "" {
		return errors.New("date must not be empty")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("date must be YYYY-MM-DD: %w", err)
	}
	return nil
}

// detailsFromCreateRequest validates that req's populated oneof variant
// matches category and converts it to storage.AssetDetails.
func detailsFromCreateRequest(category storage.AssetCategory, req *v1.CreateAssetRequest) (storage.AssetDetails, error) {
	switch category {
	case storage.AssetCategoryStockOrBond:
		d := req.GetStockOrBond()
		if d == nil {
			return storage.AssetDetails{}, errors.New("stock_or_bond details required for category STOCK_OR_BOND")
		}
		return stockOrBondDetails(d)
	case storage.AssetCategoryCash:
		d := req.GetCash()
		if d == nil {
			return storage.AssetDetails{}, errors.New("cash details required for category CASH")
		}
		return cashDetails(d)
	case storage.AssetCategoryEtf:
		d := req.GetEtf()
		if d == nil {
			return storage.AssetDetails{}, errors.New("etf details required for category ETF")
		}
		return etfDetails(d)
	case storage.AssetCategoryRetirement:
		d := req.GetRetirement()
		if d == nil {
			return storage.AssetDetails{}, errors.New("retirement details required for category RETIREMENT")
		}
		return retirementDetails(d)
	case storage.AssetCategoryRealEstate:
		d := req.GetRealEstate()
		if d == nil {
			return storage.AssetDetails{}, errors.New("real_estate details required for category REAL_ESTATE")
		}
		return realEstateDetails(d)
	case storage.AssetCategoryLoan:
		d := req.GetLoan()
		if d == nil {
			return storage.AssetDetails{}, errors.New("loan details required for category LOAN")
		}
		return loanDetails(d)
	case storage.AssetCategoryMortgage:
		d := req.GetMortgage()
		if d == nil {
			return storage.AssetDetails{}, errors.New("mortgage details required for category MORTGAGE")
		}
		return mortgageDetails(d)
	case storage.AssetCategoryCapitalBalance:
		d := req.GetCapitalBalance()
		if d == nil {
			return storage.AssetDetails{}, errors.New("capital_balance details required for category CAPITAL_BALANCE")
		}
		return capitalBalanceDetails(d)
	default:
		return storage.AssetDetails{}, fmt.Errorf("unknown category %q", category)
	}
}

// detailsFromUpdateRequest is detailsFromCreateRequest's counterpart for
// UpdateAssetRequest - identical validation, different generated oneof
// accessor type.
func detailsFromUpdateRequest(category storage.AssetCategory, req *v1.UpdateAssetRequest) (storage.AssetDetails, error) {
	switch category {
	case storage.AssetCategoryStockOrBond:
		d := req.GetStockOrBond()
		if d == nil {
			return storage.AssetDetails{}, errors.New("stock_or_bond details required for category STOCK_OR_BOND")
		}
		return stockOrBondDetails(d)
	case storage.AssetCategoryCash:
		d := req.GetCash()
		if d == nil {
			return storage.AssetDetails{}, errors.New("cash details required for category CASH")
		}
		return cashDetails(d)
	case storage.AssetCategoryEtf:
		d := req.GetEtf()
		if d == nil {
			return storage.AssetDetails{}, errors.New("etf details required for category ETF")
		}
		return etfDetails(d)
	case storage.AssetCategoryRetirement:
		d := req.GetRetirement()
		if d == nil {
			return storage.AssetDetails{}, errors.New("retirement details required for category RETIREMENT")
		}
		return retirementDetails(d)
	case storage.AssetCategoryRealEstate:
		d := req.GetRealEstate()
		if d == nil {
			return storage.AssetDetails{}, errors.New("real_estate details required for category REAL_ESTATE")
		}
		return realEstateDetails(d)
	case storage.AssetCategoryLoan:
		d := req.GetLoan()
		if d == nil {
			return storage.AssetDetails{}, errors.New("loan details required for category LOAN")
		}
		return loanDetails(d)
	case storage.AssetCategoryMortgage:
		d := req.GetMortgage()
		if d == nil {
			return storage.AssetDetails{}, errors.New("mortgage details required for category MORTGAGE")
		}
		return mortgageDetails(d)
	case storage.AssetCategoryCapitalBalance:
		d := req.GetCapitalBalance()
		if d == nil {
			return storage.AssetDetails{}, errors.New("capital_balance details required for category CAPITAL_BALANCE")
		}
		return capitalBalanceDetails(d)
	default:
		return storage.AssetDetails{}, fmt.Errorf("unknown category %q", category)
	}
}

func stockOrBondDetails(d *v1.StockOrBondDetails) (storage.AssetDetails, error) {
	name := strings.TrimSpace(d.GetCompanyOrBondName())
	platform := strings.TrimSpace(d.GetPlatform())
	if name == "" {
		return storage.AssetDetails{}, errors.New("company_or_bond_name must not be empty")
	}
	if platform == "" {
		return storage.AssetDetails{}, errors.New("platform must not be empty")
	}
	return storage.AssetDetails{CompanyOrBondName: &name, Platform: &platform}, nil
}

func cashDetails(d *v1.CashDetails) (storage.AssetDetails, error) {
	accountName := strings.TrimSpace(d.GetAccountName())
	institution := strings.TrimSpace(d.GetInstitution())
	if accountName == "" {
		return storage.AssetDetails{}, errors.New("account_name must not be empty")
	}
	if institution == "" {
		return storage.AssetDetails{}, errors.New("institution must not be empty")
	}
	return storage.AssetDetails{AccountName: &accountName, Institution: &institution}, nil
}

func etfDetails(d *v1.EtfDetails) (storage.AssetDetails, error) {
	etfName := strings.TrimSpace(d.GetEtfName())
	platform := strings.TrimSpace(d.GetPlatform())
	if etfName == "" {
		return storage.AssetDetails{}, errors.New("etf_name must not be empty")
	}
	if platform == "" {
		return storage.AssetDetails{}, errors.New("platform must not be empty")
	}
	securityType, err := fromProtoSecurityType(d.GetSecurityType())
	if err != nil {
		return storage.AssetDetails{}, err
	}
	return storage.AssetDetails{EtfName: &etfName, Platform: &platform, SecurityType: &securityType}, nil
}

func retirementDetails(d *v1.RetirementDetails) (storage.AssetDetails, error) {
	planName := strings.TrimSpace(d.GetPlanName())
	platform := strings.TrimSpace(d.GetPlatform())
	if planName == "" {
		return storage.AssetDetails{}, errors.New("plan_name must not be empty")
	}
	if platform == "" {
		return storage.AssetDetails{}, errors.New("platform must not be empty")
	}
	return storage.AssetDetails{PlanName: &planName, Platform: &platform}, nil
}

func realEstateDetails(d *v1.RealEstateDetails) (storage.AssetDetails, error) {
	propertyName := strings.TrimSpace(d.GetPropertyName())
	if propertyName == "" {
		return storage.AssetDetails{}, errors.New("property_name must not be empty")
	}
	notes := strings.TrimSpace(d.GetNotes())
	return storage.AssetDetails{PropertyName: &propertyName, Notes: &notes}, nil
}

// loanDetails is the one place in this schema where a detail field
// (lender) is allowed to stay blank - an informal/personal loan has no
// institution.
func loanDetails(d *v1.LoanDetails) (storage.AssetDetails, error) {
	name := strings.TrimSpace(d.GetName())
	if name == "" {
		return storage.AssetDetails{}, errors.New("name must not be empty")
	}
	lender := strings.TrimSpace(d.GetLender())
	return storage.AssetDetails{AccountName: &name, Institution: &lender}, nil
}

func capitalBalanceDetails(d *v1.CapitalBalanceDetails) (storage.AssetDetails, error) {
	name := strings.TrimSpace(d.GetName())
	if name == "" {
		return storage.AssetDetails{}, errors.New("name must not be empty")
	}
	return storage.AssetDetails{AccountName: &name}, nil
}

func mortgageDetails(d *v1.MortgageDetails) (storage.AssetDetails, error) {
	propertyName := strings.TrimSpace(d.GetPropertyName())
	institution := strings.TrimSpace(d.GetInstitution())
	if propertyName == "" {
		return storage.AssetDetails{}, errors.New("property_name must not be empty")
	}
	if institution == "" {
		return storage.AssetDetails{}, errors.New("institution must not be empty")
	}
	return storage.AssetDetails{PropertyName: &propertyName, Institution: &institution}, nil
}

func toProto(asset storage.Asset) *v1.Asset {
	pb := &v1.Asset{
		Id:        asset.ID,
		Category:  toProtoCategory(asset.Category),
		Currency:  toProtoCurrency(asset.Currency),
		Owners:    make([]*v1.AssetOwner, 0, len(asset.Owners)),
		Exposures: make([]*v1.AssetExposure, 0, len(asset.Exposures)),
		IsLiquid:  asset.IsLiquid,
		CreatedAt: timestamppb.New(asset.CreatedAt),
	}

	for _, owner := range asset.Owners {
		pb.Owners = append(pb.Owners, toProtoOwner(owner))
	}
	for _, exposure := range asset.Exposures {
		pb.Exposures = append(pb.Exposures, toProtoExposure(exposure))
	}
	if asset.LatestValuation != nil {
		pb.LatestValuation = toProtoValuation(*asset.LatestValuation)
	}

	switch asset.Category {
	case storage.AssetCategoryStockOrBond:
		pb.Details = &v1.Asset_StockOrBond{StockOrBond: &v1.StockOrBondDetails{
			CompanyOrBondName: derefString(asset.Details.CompanyOrBondName),
			Platform:          derefString(asset.Details.Platform),
		}}
	case storage.AssetCategoryCash:
		pb.Details = &v1.Asset_Cash{Cash: &v1.CashDetails{
			AccountName: derefString(asset.Details.AccountName),
			Institution: derefString(asset.Details.Institution),
		}}
	case storage.AssetCategoryEtf:
		securityType := storage.SecurityType("")
		if asset.Details.SecurityType != nil {
			securityType = *asset.Details.SecurityType
		}
		pb.Details = &v1.Asset_Etf{Etf: &v1.EtfDetails{
			EtfName:      derefString(asset.Details.EtfName),
			SecurityType: toProtoSecurityType(securityType),
			Platform:     derefString(asset.Details.Platform),
		}}
	case storage.AssetCategoryRetirement:
		pb.Details = &v1.Asset_Retirement{Retirement: &v1.RetirementDetails{
			PlanName: derefString(asset.Details.PlanName),
			Platform: derefString(asset.Details.Platform),
		}}
	case storage.AssetCategoryRealEstate:
		pb.Details = &v1.Asset_RealEstate{RealEstate: &v1.RealEstateDetails{
			PropertyName: derefString(asset.Details.PropertyName),
			Notes:        derefString(asset.Details.Notes),
		}}
	case storage.AssetCategoryLoan:
		pb.Details = &v1.Asset_Loan{Loan: &v1.LoanDetails{
			Name:   derefString(asset.Details.AccountName),
			Lender: derefString(asset.Details.Institution),
		}}
	case storage.AssetCategoryMortgage:
		pb.Details = &v1.Asset_Mortgage{Mortgage: &v1.MortgageDetails{
			PropertyName: derefString(asset.Details.PropertyName),
			Institution:  derefString(asset.Details.Institution),
		}}
	case storage.AssetCategoryCapitalBalance:
		pb.Details = &v1.Asset_CapitalBalance{CapitalBalance: &v1.CapitalBalanceDetails{
			Name: derefString(asset.Details.AccountName),
		}}
	}

	return pb
}

func toProtoOwner(owner storage.AssetOwner) *v1.AssetOwner {
	return &v1.AssetOwner{
		UserId:     owner.UserID,
		UserName:   owner.UserName,
		Percentage: int32(owner.Percentage),
	}
}

func toProtoExposure(exposure storage.AssetExposure) *v1.AssetExposure {
	return &v1.AssetExposure{
		Region:     toProtoRegionEnum(exposure.Region),
		Percentage: int32(exposure.Percentage),
	}
}

func toProtoValuation(valuation storage.AssetValuation) *v1.AssetValuation {
	return &v1.AssetValuation{
		Id:        valuation.ID,
		AssetId:   valuation.AssetID,
		Date:      valuation.Date,
		Amount:    valuation.Amount,
		CreatedAt: timestamppb.New(valuation.CreatedAt),
	}
}

func toProtoCategory(category storage.AssetCategory) v1.AssetCategory {
	switch category {
	case storage.AssetCategoryStockOrBond:
		return v1.AssetCategory_ASSET_CATEGORY_STOCK_OR_BOND
	case storage.AssetCategoryCash:
		return v1.AssetCategory_ASSET_CATEGORY_CASH
	case storage.AssetCategoryEtf:
		return v1.AssetCategory_ASSET_CATEGORY_ETF
	case storage.AssetCategoryRetirement:
		return v1.AssetCategory_ASSET_CATEGORY_RETIREMENT
	case storage.AssetCategoryRealEstate:
		return v1.AssetCategory_ASSET_CATEGORY_REAL_ESTATE
	case storage.AssetCategoryLoan:
		return v1.AssetCategory_ASSET_CATEGORY_LOAN
	case storage.AssetCategoryMortgage:
		return v1.AssetCategory_ASSET_CATEGORY_MORTGAGE
	case storage.AssetCategoryCapitalBalance:
		return v1.AssetCategory_ASSET_CATEGORY_CAPITAL_BALANCE
	default:
		return v1.AssetCategory_ASSET_CATEGORY_UNSPECIFIED
	}
}

func fromProtoCategory(category v1.AssetCategory) (storage.AssetCategory, error) {
	switch category {
	case v1.AssetCategory_ASSET_CATEGORY_STOCK_OR_BOND:
		return storage.AssetCategoryStockOrBond, nil
	case v1.AssetCategory_ASSET_CATEGORY_CASH:
		return storage.AssetCategoryCash, nil
	case v1.AssetCategory_ASSET_CATEGORY_ETF:
		return storage.AssetCategoryEtf, nil
	case v1.AssetCategory_ASSET_CATEGORY_RETIREMENT:
		return storage.AssetCategoryRetirement, nil
	case v1.AssetCategory_ASSET_CATEGORY_REAL_ESTATE:
		return storage.AssetCategoryRealEstate, nil
	case v1.AssetCategory_ASSET_CATEGORY_LOAN:
		return storage.AssetCategoryLoan, nil
	case v1.AssetCategory_ASSET_CATEGORY_MORTGAGE:
		return storage.AssetCategoryMortgage, nil
	case v1.AssetCategory_ASSET_CATEGORY_CAPITAL_BALANCE:
		return storage.AssetCategoryCapitalBalance, nil
	default:
		return "", errors.New("category must be STOCK_OR_BOND, CASH, ETF, RETIREMENT, REAL_ESTATE, LOAN, MORTGAGE, or CAPITAL_BALANCE")
	}
}

func toProtoCurrency(currency storage.Currency) v1.Currency {
	switch currency {
	case storage.CurrencyUSD:
		return v1.Currency_CURRENCY_USD
	case storage.CurrencyGBP:
		return v1.Currency_CURRENCY_GBP
	default:
		return v1.Currency_CURRENCY_UNSPECIFIED
	}
}

func fromProtoCurrency(currency v1.Currency) (storage.Currency, error) {
	switch currency {
	case v1.Currency_CURRENCY_USD:
		return storage.CurrencyUSD, nil
	case v1.Currency_CURRENCY_GBP:
		return storage.CurrencyGBP, nil
	default:
		return "", errors.New("currency must be USD or GBP")
	}
}

func toProtoRegionEnum(region storage.Region) v1.Region {
	switch region {
	case storage.RegionUS:
		return v1.Region_REGION_US
	case storage.RegionInternational:
		return v1.Region_REGION_INTERNATIONAL
	case storage.RegionUK:
		return v1.Region_REGION_UK
	case storage.RegionBonds:
		return v1.Region_REGION_BONDS
	default:
		return v1.Region_REGION_UNSPECIFIED
	}
}

func fromProtoRegion(region v1.Region) (storage.Region, error) {
	switch region {
	case v1.Region_REGION_US:
		return storage.RegionUS, nil
	case v1.Region_REGION_INTERNATIONAL:
		return storage.RegionInternational, nil
	case v1.Region_REGION_UK:
		return storage.RegionUK, nil
	case v1.Region_REGION_BONDS:
		return storage.RegionBonds, nil
	default:
		return "", errors.New("region must be US, INTERNATIONAL, UK, or BONDS")
	}
}

func toProtoSecurityType(securityType storage.SecurityType) v1.SecurityType {
	switch securityType {
	case storage.SecurityTypeStock:
		return v1.SecurityType_SECURITY_TYPE_STOCK
	case storage.SecurityTypeBond:
		return v1.SecurityType_SECURITY_TYPE_BOND
	case storage.SecurityTypeCommodity:
		return v1.SecurityType_SECURITY_TYPE_COMMODITY
	case storage.SecurityTypeCrypto:
		return v1.SecurityType_SECURITY_TYPE_CRYPTO
	default:
		return v1.SecurityType_SECURITY_TYPE_UNSPECIFIED
	}
}

func fromProtoSecurityType(securityType v1.SecurityType) (storage.SecurityType, error) {
	switch securityType {
	case v1.SecurityType_SECURITY_TYPE_STOCK:
		return storage.SecurityTypeStock, nil
	case v1.SecurityType_SECURITY_TYPE_BOND:
		return storage.SecurityTypeBond, nil
	case v1.SecurityType_SECURITY_TYPE_COMMODITY:
		return storage.SecurityTypeCommodity, nil
	case v1.SecurityType_SECURITY_TYPE_CRYPTO:
		return storage.SecurityTypeCrypto, nil
	default:
		return "", errors.New("security_type must be STOCK, BOND, COMMODITY, or CRYPTO")
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
