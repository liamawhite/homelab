import { useDisplayCurrency } from "@/lib/displayCurrency";
import { Currency } from "@/gen/finances/v1/asset_pb";
import { Button } from "@/components/ui/button";

export function DisplayCurrencyToggle() {
  const { displayCurrency, toggleDisplayCurrency } = useDisplayCurrency();

  return (
    <Button variant="outline" size="sm" onClick={toggleDisplayCurrency}>
      {displayCurrency === Currency.GBP ? "GBP" : "USD"}
    </Button>
  );
}
