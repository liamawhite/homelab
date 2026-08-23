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
  illiquid: "Illiquid",
};

// Single-select (radio), unlike UserFilter's multi-select checkboxes -
// liquid/illiquid/all are mutually exclusive views, not a set of people to
// combine.
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
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Show</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuRadioGroup value={value} onValueChange={(v) => setValue(v as LiquidityFilterValue)}>
          <DropdownMenuRadioItem value="all">All</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="liquid">Liquid</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="illiquid">Illiquid</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
