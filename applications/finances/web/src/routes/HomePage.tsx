import { useAssets } from "@/lib/assets";
import { useSelectedUserIds } from "@/lib/selectedUsers";
import { useNetWorthSeries } from "@/lib/netWorth";
import { formatDate } from "@/lib/dates";
import { NetWorthChart } from "@/components/NetWorthChart";
import { ExposureChart } from "@/components/ExposureChart";
import { CapitalBalanceCard } from "@/components/CapitalBalanceCard";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function HomePage() {
  const { data: assets, isLoading: assetsLoading, isError: assetsError } = useAssets();
  const [selectedUserIds] = useSelectedUserIds();

  const { series, isLoading, isError } = useNetWorthSeries(assets, selectedUserIds);
  const latestPoint = series && series.points.length > 0 ? series.points[series.points.length - 1] : null;

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-4 p-4">
      <Card size="sm">
        <CardHeader>
          <CardTitle>Net worth</CardTitle>
        </CardHeader>
        <CardContent>
          {(assetsLoading || isLoading) && (
            <p className="py-16 text-center text-sm text-muted-foreground">Loading…</p>
          )}
          {(assetsError || isError) && (
            <p className="py-16 text-center text-sm text-destructive">Couldn't load net worth history.</p>
          )}
          {!assetsLoading && !isLoading && !assetsError && !isError && series && series.points.length === 0 && (
            <p className="py-16 text-center text-sm text-muted-foreground">
              No valuation history yet - add an asset to see net worth over time.
            </p>
          )}
          {series && series.points.length > 0 && <NetWorthChart series={series} />}
        </CardContent>
      </Card>

      {latestPoint && (
        <Card size="sm">
          <CardHeader>
            <CardTitle>Exposure</CardTitle>
            <CardDescription>As of {formatDate(latestPoint.date)}</CardDescription>
          </CardHeader>
          <CardContent>
            <ExposureChart point={latestPoint} />
          </CardContent>
        </Card>
      )}

      <CapitalBalanceCard assets={assets} />
    </div>
  );
}
