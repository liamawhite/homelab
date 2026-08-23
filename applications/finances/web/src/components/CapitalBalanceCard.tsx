import type { Asset } from "@/gen/finances/v1/asset_pb";
import { useCapitalBalance } from "@/lib/capitalBalance";
import { useDisplayCurrency } from "@/lib/displayCurrency";
import { formatCurrency } from "@/lib/money";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface CapitalBalanceCardProps {
  assets: Asset[] | undefined;
}

// Shows the household's shared-capital IOU (see
// AssetCategory.CAPITAL_BALANCE) - always visible, independent of the
// navbar's user filter, showing the raw current fact plainly. Plain text
// only, no bar/gauge - a fill or marker reads ambiguously as either "owed
// to" or "owed by" depending on the viewer's mental model, and there's no
// natural reference point for "how big" this number is anyway, so a
// positioned mark carries no real information over the dollar figure.
export function CapitalBalanceCard({ assets }: CapitalBalanceCardProps) {
  const balance = useCapitalBalance(assets);
  const { displayCurrency } = useDisplayCurrency();

  if (!balance) return null;

  const { name, ownerName, otherName, amount } = balance;
  const isEven = amount === 0;
  const owingName = amount > 0 ? otherName : ownerName;

  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle>{name}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">
          {isEven ? "Even" : `${owingName} owes ${formatCurrency(Math.abs(amount), displayCurrency)}`}
        </p>
      </CardContent>
    </Card>
  );
}
