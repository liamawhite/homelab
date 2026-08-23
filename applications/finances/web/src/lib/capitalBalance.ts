import { AssetCategory } from "@/gen/finances/v1/asset_pb";
import type { Asset } from "@/gen/finances/v1/asset_pb";
import { useUsers } from "./users";
import { useDisplayCurrency } from "./displayCurrency";
import { useExchangeRates, convertAmount } from "./exchangeRates";

export interface CapitalBalance {
  name: string; // the tracker's own display name, e.g. "Shared Capital"
  ownerName: string;
  otherName: string;
  amount: number; // dollars, signed, in displayCurrency - positive means ownerName is owed
}

// Finds the singleton capital-balance asset (if any exists yet) and
// resolves it into the two names and the current signed amount its card
// needs. Deliberately independent of the navbar's selectedUserIds filter -
// unlike the net worth/exposure computations, this always shows the raw
// current fact regardless of who's selected.
export function useCapitalBalance(assets: Asset[] | undefined): CapitalBalance | null {
  const { data: users } = useUsers();
  const { displayCurrency } = useDisplayCurrency();

  const asset = (assets ?? []).find((a) => a.category === AssetCategory.CAPITAL_BALANCE);
  const date = asset?.latestValuation?.date ?? "";
  const { data: ratesByDate } = useExchangeRates(date ? [date] : []);

  if (!asset || asset.details.case !== "capitalBalance" || !asset.latestValuation || !users) {
    return null;
  }

  const owner = asset.owners[0];
  if (!owner) return null;
  const other = users.find((u) => u.id !== owner.userId);
  if (!other) return null;

  const converted = convertAmount(
    asset.latestValuation.amount,
    asset.currency,
    displayCurrency,
    date,
    ratesByDate ?? {},
  );
  if (converted === null) return null;

  return {
    name: asset.details.value.name,
    ownerName: owner.userName,
    otherName: other.name,
    amount: Number(converted) / 100,
  };
}
