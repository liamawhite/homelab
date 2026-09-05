import { ChevronDown, Droplets } from "lucide-react";

import { useLiquidityFilter } from "@/lib/liquidityFilter";
import type { LiquidityFilter as LiquidityFilterValue } from "@/lib/liquidityFilter";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";

const LABELS: Record<LiquidityFilterValue, string> = {
  all: "All",
  liquid: "Liquid",
  invested: "Invested",
  illiquid: "Illiquid",
};

// Shown under each tier's label in the dropdown so the (somewhat
// judgment-call) boundaries are explicit rather than left to guess from the
// name alone.
const DESCRIPTIONS: Record<Exclude<LiquidityFilterValue, "all">, string> = {
  liquid: "Cash & cash-equivalents - SPAXX, bond ETFs like VBIL",
  invested: "Accessible, but with tax implications - stocks, ETFs",
  illiquid: "Locked up - real estate, retirement accounts",
};

// Single-select (radio), unlike UserFilter's multi-select checkboxes -
// liquid/invested/illiquid/all are mutually exclusive views, not a set of
// people to combine.
export function LiquidityFilter() {
  const [value, setValue] = useLiquidityFilter();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="gap-1.5">
          <Droplets />
          {LABELS[value]}
          <ChevronDown className="text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuLabel>Show</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuRadioGroup value={value} onValueChange={(v) => setValue(v as LiquidityFilterValue)}>
          <DropdownMenuRadioItem value="all">All</DropdownMenuRadioItem>
          {(["liquid", "invested", "illiquid"] as const).map((tier) => (
            <DropdownMenuRadioItem key={tier} value={tier} className="items-start">
              <div className="grid gap-0.5 py-0.5">
                <span>{LABELS[tier]}</span>
                <span className="text-xs whitespace-normal text-muted-foreground">{DESCRIPTIONS[tier]}</span>
              </div>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
