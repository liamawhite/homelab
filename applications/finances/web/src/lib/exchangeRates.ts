import { useQuery } from "@tanstack/react-query";

import { exchangeRateClient } from "./client";
import { Currency } from "@/gen/finances/v1/asset_pb";

// GBP->USD rate per date, as returned by the server (see
// exchangerateservice.GetRates) - already resolved to the nearest earlier
// stored rate for any date with no exact one, so this is a direct lookup,
// no client-side fallback needed.
export function useExchangeRates(dates: string[]) {
  const uniqueDates = Array.from(new Set(dates)).sort();

  return useQuery({
    queryKey: ["exchangeRates", uniqueDates],
    queryFn: async () => (await exchangeRateClient.getRates({ dates: uniqueDates })).usdPerGbpByDate,
    enabled: uniqueDates.length > 0,
  });
}

// Converts amountMinor from `from` currency to `to` currency using date's
// rate. Returns null if no rate is available for that date - callers
// should fall back to the unconverted stored value rather than a made-up
// number.
export function convertAmount(
  amountMinor: bigint,
  from: Currency,
  to: Currency,
  date: string,
  ratesByDate: Record<string, number>,
): bigint | null {
  if (from === to) return amountMinor;

  const usdPerGbp = ratesByDate[date];
  if (usdPerGbp === undefined) return null;

  if (from === Currency.GBP && to === Currency.USD) {
    return BigInt(Math.round(Number(amountMinor) * usdPerGbp));
  }
  if (from === Currency.USD && to === Currency.GBP) {
    return BigInt(Math.round(Number(amountMinor) / usdPerGbp));
  }

  return amountMinor;
}
