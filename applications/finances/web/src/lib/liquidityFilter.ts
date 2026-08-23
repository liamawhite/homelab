import { useNavigate, useSearch } from "@tanstack/react-router";

export type LiquidityFilter = "all" | "liquid" | "illiquid";

// Whether the navbar's liquidity filter is showing everything, only liquid
// assets, or only illiquid assets. Lives in the `liquidity` URL search param
// (root route, see router.tsx) rather than localStorage, matching
// useSelectedUserIds's reasoning: a bookmarked/shared link should reproduce
// the same filtered view.
export function useLiquidityFilter(): [LiquidityFilter, (filter: LiquidityFilter) => void] {
  const { liquidity } = useSearch({ from: "__root__" });
  const value: LiquidityFilter = liquidity === "liquid" || liquidity === "illiquid" ? liquidity : "all";

  const navigate = useNavigate();
  const setValue = (next: LiquidityFilter) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, liquidity: next === "all" ? undefined : next }),
      replace: true,
    });
  };

  return [value, setValue];
}

// Whether an asset's isLiquid flag passes the current filter - "all" always
// passes.
export function liquidityMatches(isLiquid: boolean, filter: LiquidityFilter): boolean {
  if (filter === "all") return true;
  return filter === "liquid" ? isLiquid : !isLiquid;
}
