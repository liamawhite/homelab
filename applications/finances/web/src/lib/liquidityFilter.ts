import { useNavigate, useSearch } from "@tanstack/react-router";

import { AssetLiquidity } from "@/gen/finances/v1/asset_pb";

export type LiquidityFilter = "all" | "liquid" | "invested" | "illiquid";

const URL_VALUES: Record<Exclude<LiquidityFilter, "all">, string> = {
  liquid: "liquid",
  invested: "invested",
  illiquid: "illiquid",
};

// Whether the navbar's liquidity filter is showing everything, or only one
// tier. Lives in the `liquidity` URL search param (root route, see
// router.tsx) rather than localStorage, matching useSelectedUserIds's
// reasoning: a bookmarked/shared link should reproduce the same filtered
// view.
export function useLiquidityFilter(): [LiquidityFilter, (filter: LiquidityFilter) => void] {
  const { liquidity } = useSearch({ from: "__root__" });
  const value: LiquidityFilter =
    liquidity === "liquid" || liquidity === "invested" || liquidity === "illiquid" ? liquidity : "all";

  const navigate = useNavigate();
  const setValue = (next: LiquidityFilter) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, liquidity: next === "all" ? undefined : URL_VALUES[next] }),
      replace: true,
    });
  };

  return [value, setValue];
}

// Whether an asset's liquidity tier passes the current filter - "all"
// always passes.
export function liquidityMatches(liquidity: AssetLiquidity, filter: LiquidityFilter): boolean {
  if (filter === "all") return true;
  return liquidity === liquidityFilterToEnum(filter);
}

export function liquidityFilterToEnum(filter: Exclude<LiquidityFilter, "all">): AssetLiquidity {
  switch (filter) {
    case "liquid":
      return AssetLiquidity.LIQUID;
    case "invested":
      return AssetLiquidity.INVESTED;
    case "illiquid":
      return AssetLiquidity.ILLIQUID;
  }
}
