import { Bar, BarChart, Cell, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { useTheme } from "@/lib/theme";
import { useDisplayCurrency } from "@/lib/displayCurrency";
import { EXPOSURE_COLORS } from "@/lib/exposureColors";
import { formatCurrency, formatCompactCurrency } from "@/lib/money";
import { sortedExposureRows } from "@/lib/netWorth";
import type { NetWorthPoint, ExposureDetailRow } from "@/lib/netWorth";
import type { Currency } from "@/gen/finances/v1/asset_pb";

interface ExposureRow {
  band: string;
  value: number;
  detail: ExposureDetailRow[];
}

interface ExposureTooltipProps {
  active?: boolean;
  payload?: { payload: ExposureRow }[];
  currency: Currency;
}

function ExposureTooltip({ active, payload, currency }: ExposureTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const { band, value, detail } = payload[0].payload;

  return (
    <div className="grid w-48 gap-2 rounded-lg border bg-popover px-3 py-2 text-xs shadow-md">
      <div className="flex items-center justify-between gap-4">
        <span className="font-medium text-popover-foreground">{band}</span>
        <span className="tabular-nums font-medium text-popover-foreground">{formatCurrency(value, currency)}</span>
      </div>
      {detail.length > 0 && (
        <div className="grid max-h-48 gap-1 overflow-y-auto border-t pt-2">
          {detail.map((row, i) => (
            <div key={`${row.name}-${row.owner}-${i}`} className="flex items-center justify-between gap-4">
              <span className="text-foreground">
                {row.name} <span className="text-muted-foreground">{row.owner}</span>
              </span>
              <span className="tabular-nums text-muted-foreground">{formatCurrency(row.amount, currency)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

interface ValueLabelProps {
  x?: number;
  y?: number;
  width?: number;
  height?: number;
  value?: number;
  textColor: string;
  currency: Currency;
}

// recharts always anchors `x` at the zero baseline for a horizontal bar and
// gives `width` a sign (negative for a negative value, extending left) -
// it does not normalize to a left-edge/positive-width pair. That makes
// `x + width` the bar's "tip" (the end away from the baseline) regardless
// of sign, which is what dataviz's "value at the tip" rule wants the label
// anchored to.
function ValueLabel({ x, y, width, height, value, textColor, currency }: ValueLabelProps) {
  if (x === undefined || y === undefined || width === undefined || height === undefined || value === undefined) {
    return null;
  }
  const isNegative = width < 0;
  const tipX = x + width;
  const labelX = isNegative ? tipX - 6 : tipX + 6;

  return (
    <text
      x={labelX}
      y={y + height / 2}
      dy={4}
      textAnchor={isNegative ? "end" : "start"}
      fontSize={12}
      fill={textColor}
      className="tabular-nums"
    >
      {formatCompactCurrency(value, currency)}
    </text>
  );
}

interface ExposureChartProps {
  point: NetWorthPoint;
}

// A snapshot of the latest point's exposure breakdown - one horizontal bar
// per band, extending right for assets and left for liabilities from a
// zero baseline. Not a stacked/part-to-whole bar: liabilities are negative,
// so there's no meaningful 100% for the bars to sum to (see the dataviz
// skill's form guidance - this is closer to a diverging-bar shape, just
// keeping each band's own categorical color instead of a semantic
// diverging red/green scale). Colors and row order match NetWorthChart's
// hover tooltip exactly (shared EXPOSURE_COLORS + sortedExposureRows), so
// the two sections read as one consistent picture of the same data.
export function ExposureChart({ point }: ExposureChartProps) {
  const { theme } = useTheme();
  const { displayCurrency } = useDisplayCurrency();
  const mode = theme === "dark" ? "dark" : "light";

  const axisColor = mode === "dark" ? "#9aa0a6" : "#5b6169";
  const gridColor = mode === "dark" ? "#2b2f33" : "#eae8e2";

  const rows: ExposureRow[] = sortedExposureRows(point).map(([band, value]) => ({
    band,
    value,
    detail: point.byExposureDetail[band] ?? [],
  }));

  if (rows.length === 0) {
    return <p className="py-16 text-center text-sm text-muted-foreground">No exposure yet.</p>;
  }

  const values = rows.map((r) => r.value);
  const min = Math.min(0, ...values);
  const max = Math.max(0, ...values);
  const padding = (max - min) * 0.15 || 1;
  const domain: [number, number] = [Math.floor(min - padding), Math.ceil(max + padding)];

  // Tall enough for every row at a fixed height, rather than a fixed chart
  // height that squeezes rows as bands are added/removed.
  const height = rows.length * 36 + 16;

  return (
    <ResponsiveContainer width="100%" height={height}>
      <BarChart data={rows} layout="vertical" margin={{ top: 4, right: 56, bottom: 4, left: 8 }}>
        <XAxis type="number" domain={domain} hide />
        <YAxis
          type="category"
          dataKey="band"
          tick={{ fill: axisColor, fontSize: 12 }}
          axisLine={false}
          tickLine={false}
          width={150}
        />
        <ReferenceLine x={0} stroke={gridColor} strokeWidth={1} />
        <Tooltip
          content={<ExposureTooltip currency={displayCurrency} />}
          cursor={{ fill: gridColor, opacity: 0.5 }}
        />
        <Bar
          dataKey="value"
          barSize={20}
          radius={4}
          isAnimationActive={false}
          label={(props: ValueLabelProps) => (
            <ValueLabel {...props} textColor={axisColor} currency={displayCurrency} />
          )}
        >
          {rows.map((row) => (
            <Cell key={row.band} fill={EXPOSURE_COLORS[row.band]?.[mode] ?? axisColor} />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  );
}
