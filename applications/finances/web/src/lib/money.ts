import { Currency } from "@/gen/finances/v1/asset_pb";

// Shared currency-aware formatting, used by NetWorthChart and ExposureChart
// (both already-converted-dollars presentational charts) so the two stay
// identical rather than drifting via separate copies.
export function currencyCode(currency: Currency): string {
  return currency === Currency.GBP ? "GBP" : "USD";
}

export function formatCurrency(value: number, currency: Currency): string {
  return new Intl.NumberFormat(undefined, { style: "currency", currency: currencyCode(currency) }).format(value);
}

export function formatCompactCurrency(value: number, currency: Currency): string {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: currencyCode(currency),
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}
