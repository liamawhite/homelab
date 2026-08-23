import { useQueries } from "@tanstack/react-query";

import { assetClient } from "./client";
import { valuationsQueryKey, assetDisplayName } from "./assets";
import { useExchangeRates, convertAmount } from "./exchangeRates";
import { useDisplayCurrency } from "./displayCurrency";
import { liquidityMatches } from "./liquidityFilter";
import type { LiquidityFilter } from "./liquidityFilter";
import type { Asset, AssetValuation } from "@/gen/finances/v1/asset_pb";
import { AssetCategory, Currency, Region, SecurityType } from "@/gen/finances/v1/asset_pb";

// One asset's contribution to a single exposure band - an asset with a
// blended allocation (e.g. a retirement account split across US/
// International/Bonds) shows up as a separate row, amount-only, under each
// band it contributes to, rather than once under some single "main" band.
export interface ExposureDetailRow {
  name: string;
  owner: string; // first name of the sole owner, or "Joint" for a shared asset
  amount: number; // dollars, signed
}

export interface NetWorthPoint {
  date: string;
  netWorth: number; // dollars, signed
  byExposure: Record<string, number>; // dollars, signed - only bands present in exposureBands
  byExposureDetail: Record<string, ExposureDetailRow[]>; // same bands, descending by |amount|
}

export interface NetWorthSeries {
  exposureBands: string[]; // stable order, only bands that are ever nonzero
  points: NetWorthPoint[]; // sorted by date ascending
}

const UNALLOCATED_LABEL = "Unallocated";
const CASH_LABEL = "Cash";
const COMMODITY_LABEL = "Commodity";
const CRYPTO_LABEL = "Crypto";

// The closed set of every label exposureLabel()/liabilityLabel()/
// noExposureLabel() can produce - real estate only ever reports
// US/International/UK exposure (never Bonds), so this enumeration is
// exhaustive, not just a default order. Plain "US"/"International"/"UK"
// (no ETF/Individual Stock suffix) is exposureLabel()'s defensive fallback
// for a category with no vehicle-type identity of its own - every current
// category maps to a more specific band, so these don't appear today, but
// they're kept here in case a future category needs the plain form.
const EXPOSURE_BAND_ORDER = [
  "US",
  "US ETF",
  "US Individual Stock",
  "International",
  "International ETF",
  "International Individual Stock",
  "UK",
  "UK ETF",
  "UK Individual Stock",
  "Bonds",
  CASH_LABEL,
  "US Real Estate",
  "International Real Estate",
  "UK Real Estate",
  COMMODITY_LABEL,
  CRYPTO_LABEL,
  UNALLOCATED_LABEL,
  "Loan",
  "Mortgage",
];

function isLiabilityCategory(category: AssetCategory): boolean {
  return category === AssetCategory.LOAN || category === AssetCategory.MORTGAGE;
}

// A capital-balance asset isn't a real liability or a market exposure - it's
// a household-internal ledger of which of two people has contributed more
// shared capital (see AssetCategory.CAPITAL_BALANCE's proto doc comment).
// It's handled entirely separately from every other category: excluded from
// byExposure, and only contributes to netWorth when exactly one individual
// is selected (see the capital-balance branch in computeNetWorthSeries).
function isCapitalBalanceCategory(category: AssetCategory): boolean {
  return category === AssetCategory.CAPITAL_BALANCE;
}

// Liabilities have no geography - each keeps its own category name as its
// band (Loan, Mortgage) instead of folding into a generic "Liabilities"
// bucket, so the two stay distinguishable in the hover breakdown.
function liabilityLabel(category: AssetCategory): string {
  return category === AssetCategory.LOAN ? "Loan" : "Mortgage";
}

function regionName(region: Region): string {
  switch (region) {
    case Region.US:
      return "US";
    case Region.INTERNATIONAL:
      return "International";
    case Region.UK:
      return "UK";
    case Region.BONDS:
      return "Bonds";
    default:
      return UNALLOCATED_LABEL;
  }
}

// Real estate's geography is tracked separately from financial-asset
// geography - a house counted as plain "US" exposure would conflate two
// very different kinds of risk, so real estate gets its own "<region> Real
// Estate" bands instead of folding into the shared US/International/UK ones.
// Cash collapses to one flat "Cash" band regardless of which region it's
// tagged with - a cash account sitting in a US bank isn't "US market
// exposure" the way a US stock is, so lumping the two under "US" would
// overstate actual market risk. Within a region, a diversified holding and
// an individual stock carry different risk (spread vs single-name), so
// each gets its own "<region> ETF"/"<region> Individual Stock" band -
// Retirement counts as "ETF" here too, since a 401k's underlying funds are
// the same diversified-fund risk profile as holding the ETF directly, not
// a single-name bet - except Region.BONDS itself, which stays one flat
// "Bonds" band regardless of vehicle (an individual bond and a bond ETF are
// the same asset-class risk).
function exposureLabel(category: AssetCategory, region: Region): string {
  if (category === AssetCategory.CASH) return CASH_LABEL;
  const name = regionName(region);
  if (category === AssetCategory.REAL_ESTATE) return `${name} Real Estate`;
  if (region === Region.BONDS) return name;
  if (category === AssetCategory.ETF || category === AssetCategory.RETIREMENT) return `${name} ETF`;
  if (category === AssetCategory.STOCK_OR_BOND) return `${name} Individual Stock`;
  return name;
}

// The band for an asset with no AssetExposure rows at all - commodities and
// crypto (gold, bitcoin) genuinely have no geography, but lumping them into
// a bare "Unallocated" bucket alongside anything else with a data gap loses
// real information the ETF's own securityType already has. Anything else
// with no exposures set (a stock/bond, or a commodity/crypto asset held
// outside an ETF wrapper - not possible in the schema today, but not
// asserted against either) still falls back to Unallocated.
function noExposureLabel(asset: Asset): string {
  if (asset.details.case === "etf") {
    if (asset.details.value.securityType === SecurityType.COMMODITY) return COMMODITY_LABEL;
    if (asset.details.value.securityType === SecurityType.CRYPTO) return CRYPTO_LABEL;
  }
  return UNALLOCATED_LABEL;
}

// First name of an asset's sole owner, or "Joint" for a shared asset - many
// asset names collide across the household (both Liam and Tia hold a
// "SPAXX" cash sweep and a joint account holds one too), so the exposure
// tooltip's per-asset breakdown needs this to stay legible/unambiguous.
function ownerLabel(owners: Asset["owners"]): string {
  if (owners.length !== 1) return "Joint";
  return owners[0].userName.split(" ")[0];
}

// Percentage (0-100) of an asset attributable to selectedUserIds - an empty
// selection means "everyone", matching the navbar's user filter.
function ownedShare(owners: Asset["owners"], selectedUserIds: string[]): number {
  if (selectedUserIds.length === 0) return 100;
  return owners
    .filter((owner) => selectedUserIds.includes(owner.userId))
    .reduce((sum, owner) => sum + owner.percentage, 0);
}

interface AssetHistory {
  asset: Asset;
  share: number;
  valuations: AssetValuation[]; // ascending by date
}

// The value of an asset "as of" date - its most recent valuation with
// date <= the target, or 0n if it had none yet. valuations must already be
// sorted ascending by date.
function valueAsOf(valuations: AssetValuation[], date: string): bigint {
  let amount = 0n;
  for (const valuation of valuations) {
    if (valuation.date > date) break;
    amount = valuation.amount;
  }
  return amount;
}

// Pure computation, kept separate from the data-fetching hook below so it's
// easy to reason about (and could be unit tested) independent of React
// Query. Every asset is converted to displayCurrency using ratesByDate
// (see useExchangeRates) before summing - a date with no available rate
// simply drops that asset's contribution for that point rather than
// corrupting the total by mixing currencies. Loan and Mortgage categories
// subtract from the total. The chart itself only plots netWorth -
// byExposure rides along per point purely for the hover tooltip's
// breakdown, which is exposure totals (what's the money actually invested
// in / geographically exposed to), not asset category.
export function computeNetWorthSeries(
  assets: Asset[],
  valuationsByAssetId: Record<string, AssetValuation[]>,
  selectedUserIds: string[],
  liquidityFilter: LiquidityFilter,
  displayCurrency: Currency,
  ratesByDate: Record<string, number>,
): NetWorthSeries {
  const histories: AssetHistory[] = [];
  const dateSet = new Set<string>();

  for (const asset of assets) {
    // A capital-balance asset's inclusion/sign depends on which individual
    // is selected, not on percentage ownership (see the dedicated branch
    // below) - it must never be dropped here just because the currently
    // selected person isn't its stored owner. It also always ignores the
    // liquidity filter, same reasoning: it's a household ledger entry, not
    // a market asset with a liquidity profile.
    if (!isCapitalBalanceCategory(asset.category) && !liquidityMatches(asset.isLiquid, liquidityFilter)) continue;
    const share = isCapitalBalanceCategory(asset.category) ? 100 : ownedShare(asset.owners, selectedUserIds);
    if (share <= 0) continue;

    const valuations = [...(valuationsByAssetId[asset.id] ?? [])].sort((a, b) =>
      a.date < b.date ? -1 : a.date > b.date ? 1 : 0,
    );
    if (valuations.length === 0) continue;

    for (const valuation of valuations) dateSet.add(valuation.date);
    histories.push({ asset, share, valuations });
  }

  const dates = Array.from(dateSet).sort();

  const rawPoints: {
    date: string;
    netWorthMinor: bigint;
    byExposure: Record<string, bigint>;
    byExposureDetail: Record<string, { name: string; owner: string; amount: bigint }[]>;
  }[] = [];
  const bandsSeen = new Set<string>();

  for (const date of dates) {
    const byExposure: Record<string, bigint> = {};
    const byExposureDetail: Record<string, { name: string; owner: string; amount: bigint }[]> = {};
    let netWorthMinor = 0n;

    const addToBand = (label: string, name: string, owner: string, amount: bigint) => {
      byExposure[label] = (byExposure[label] ?? 0n) + amount;
      (byExposureDetail[label] ??= []).push({ name, owner, amount });
    };

    for (const history of histories) {
      const amount = valueAsOf(history.valuations, date);
      if (amount === 0n) continue;

      const converted = convertAmount(amount, history.asset.currency, displayCurrency, date, ratesByDate);
      if (converted === null) continue;

      if (isCapitalBalanceCategory(history.asset.category)) {
        // Only affects the asset's own owner's individual net worth - it's
        // an internal ledger entry against that owner's side, not money the
        // other household member actually holds, so it never appears in
        // their individual view either. "Everyone" (or more than one
        // person) cancels out entirely by definition, since it's an IOU
        // within the household.
        if (selectedUserIds.length !== 1) continue;
        const ownerID = history.asset.owners[0]?.userId;
        if (selectedUserIds[0] !== ownerID) continue;
        netWorthMinor += converted;
        continue;
      }

      const contribution = (converted * BigInt(history.share)) / 100n;
      const signed = isLiabilityCategory(history.asset.category) ? -contribution : contribution;
      netWorthMinor += signed;

      const name = assetDisplayName(history.asset);
      const owner = ownerLabel(history.asset.owners);

      if (isLiabilityCategory(history.asset.category)) {
        addToBand(liabilityLabel(history.asset.category), name, owner, signed);
        continue;
      }

      const exposures = history.asset.exposures;
      if (exposures.length === 0) {
        addToBand(noExposureLabel(history.asset), name, owner, signed);
        continue;
      }

      for (const exposure of exposures) {
        const label = exposureLabel(history.asset.category, exposure.region);
        const portion = (signed * BigInt(exposure.percentage)) / 100n;
        addToBand(label, name, owner, portion);
      }
    }

    for (const [label, amount] of Object.entries(byExposure)) {
      if (amount !== 0n) bandsSeen.add(label);
    }

    rawPoints.push({ date, netWorthMinor, byExposure, byExposureDetail });
  }

  const exposureBands = EXPOSURE_BAND_ORDER.filter((label) => bandsSeen.has(label));

  const points: NetWorthPoint[] = rawPoints.map(({ date, netWorthMinor, byExposure, byExposureDetail }) => ({
    date,
    netWorth: Number(netWorthMinor) / 100,
    byExposure: Object.fromEntries(exposureBands.map((label) => [label, Number(byExposure[label] ?? 0n) / 100])),
    byExposureDetail: Object.fromEntries(
      exposureBands.map((label) => [
        label,
        (byExposureDetail[label] ?? [])
          .filter((row) => row.amount !== 0n)
          .sort((a, b) => Math.abs(Number(b.amount)) - Math.abs(Number(a.amount)))
          .map((row) => ({ name: row.name, owner: row.owner, amount: Number(row.amount) / 100 })),
      ]),
    ),
  }));

  return { exposureBands, points };
}

// Fetches every asset's full valuation history in parallel (the shared
// ["assets", assetId, "valuations"] query key means this cache is the same
// one AddAssetValuation/DeleteAssetValuation already invalidate, so the
// chart updates itself after either), plus exchange rates for every date
// involved, and computes the net worth series in the navbar's selected
// display currency.
export function useNetWorthSeries(
  assets: Asset[] | undefined,
  selectedUserIds: string[],
  liquidityFilter: LiquidityFilter,
) {
  const { displayCurrency } = useDisplayCurrency();

  const valuationQueries = useQueries({
    queries: (assets ?? []).map((asset) => ({
      queryKey: valuationsQueryKey(asset.id),
      queryFn: async () => (await assetClient.listAssetValuations({ assetId: asset.id })).valuations,
    })),
  });

  const isLoading = assets === undefined || valuationQueries.some((q) => q.isLoading);
  const isError = valuationQueries.some((q) => q.isError);

  const allDates = valuationQueries.flatMap((q) => (q.data ?? []).map((v) => v.date));
  const { data: ratesByDate } = useExchangeRates(allDates);

  if (isLoading || isError || !assets) {
    return { series: null, isLoading, isError };
  }

  const valuationsByAssetId: Record<string, AssetValuation[]> = {};
  assets.forEach((asset, i) => {
    valuationsByAssetId[asset.id] = valuationQueries[i]?.data ?? [];
  });

  const series = computeNetWorthSeries(
    assets,
    valuationsByAssetId,
    selectedUserIds,
    liquidityFilter,
    displayCurrency,
    ratesByDate ?? {},
  );
  return { series, isLoading: false, isError: false };
}

// A point's exposure bands, nonzero only, descending by signed value - so
// assets (positive) sort biggest-first and liabilities (negative) sink to
// the bottom, worst first. Shared by the Net Worth hover tooltip and the
// Exposure chart so both ever show the same order for the same point.
export function sortedExposureRows(point: NetWorthPoint): [string, number][] {
  return Object.entries(point.byExposure)
    .filter(([, value]) => value !== 0)
    .sort(([, a], [, b]) => b - a);
}
