import { useState } from "react";
import {
  Area,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { useTheme } from "@/lib/theme";
import { useDisplayCurrency } from "@/lib/displayCurrency";
import { EXPOSURE_COLORS } from "@/lib/exposureColors";
import { formatCurrency, formatCompactCurrency } from "@/lib/money";
import { sortedExposureRows } from "@/lib/netWorth";
import { Currency } from "@/gen/finances/v1/asset_pb";
import type { NetWorthPoint, NetWorthSeries } from "@/lib/netWorth";

// Sequential default hue (see dataviz skill's palette.md) - the bold
// overlay line showing the true net-worth total, distinct from the
// per-band EXPOSURE_COLORS underneath it.
const LINE_COLOR = { light: "#1b1f23", dark: "#f2f0ec" };

function formatDateTick(date: string): string {
  const [year, month] = date.split("-");
  return new Date(Number(year), Number(month) - 1, 1).toLocaleDateString(undefined, {
    month: "short",
    year: "numeric",
  });
}

interface NetWorthTooltipProps {
  active?: boolean;
  label?: string;
  payload?: { payload: NetWorthPoint }[];
  mode: "light" | "dark";
  mutedColor: string;
  currency: Currency;
  hovered: string | null;
}

function NetWorthTooltip({ active, label, payload, mode, mutedColor, currency, hovered }: NetWorthTooltipProps) {
  if (!active || !payload || payload.length === 0 || !label) return null;
  const point = payload[0].payload;
  const rows = sortedExposureRows(point);

  return (
    <div className="grid w-56 gap-2 rounded-lg border bg-popover p-3 text-xs shadow-md">
      <div className="flex items-center justify-between gap-4">
        <span
          className="text-popover-foreground"
          style={{ fontWeight: hovered === "netWorth" ? 700 : 500 }}
        >
          {formatDateTick(label)}
        </span>
        <span
          className="tabular-nums text-popover-foreground"
          style={{ fontWeight: hovered === "netWorth" ? 700 : 500 }}
        >
          {formatCurrency(point.netWorth, currency)}
        </span>
      </div>

      <div className="grid gap-1 border-t pt-2">
        {rows.map(([band, value]) => (
          <div key={band} className="flex items-center justify-between gap-4">
            <span
              className={hovered === band ? "flex items-center gap-1.5 text-popover-foreground" : "flex items-center gap-1.5"}
              style={{ color: hovered === band ? undefined : mutedColor, fontWeight: hovered === band ? 700 : 400 }}
            >
              <span
                className="size-2 rounded-full"
                style={{ backgroundColor: EXPOSURE_COLORS[band]?.[mode] ?? mutedColor }}
              />
              {band}
            </span>
            <span
              className="tabular-nums text-popover-foreground"
              style={{ fontWeight: hovered === band ? 700 : 400 }}
            >
              {formatCurrency(value, currency)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

interface NetWorthChartProps {
  series: NetWorthSeries;
}

export function NetWorthChart({ series }: NetWorthChartProps) {
  const { theme } = useTheme();
  const { displayCurrency } = useDisplayCurrency();
  const mode = theme === "dark" ? "dark" : "light";
  const [hovered, setHovered] = useState<string | null>(null);

  const gridColor = mode === "dark" ? "#2b2f33" : "#eae8e2";
  const axisColor = mode === "dark" ? "#9aa0a6" : "#5b6169";
  const lineColor = LINE_COLOR[mode];
  // Thin background-colored stroke between stacked segments (see dataviz
  // skill's "2px surface gap between fills") - simulated with a stroke in
  // the card's own surface color, since Recharts renders it as a plain SVG
  // attribute rather than a CSS custom property.
  const surfaceColor = mode === "dark" ? "#1b1f23" : "#ffffff";

  // Each exposure band becomes its own stacked series - byExposure/
  // byExposureDetail stay on the object too (spread first, band keys can't
  // collide with NetWorthPoint's own field names), so NetWorthTooltip's
  // existing sortedExposureRows(point) lookup keeps working unchanged.
  const chartData = series.points.map((point) => ({ ...point, ...point.byExposure }));

  // Domain must cover both the overlaid net-worth line and the full
  // stacked height in each direction - the two can differ (e.g. a single
  // person's view includes a Capital Balance contribution to netWorth that
  // never enters byExposure/the stack by design), so neither alone is a
  // safe bound.
  const netWorthValues = series.points.map((p) => p.netWorth);
  const positiveSums = series.points.map((p) =>
    Object.values(p.byExposure).reduce((sum, v) => (v > 0 ? sum + v : sum), 0),
  );
  const negativeSums = series.points.map((p) =>
    Object.values(p.byExposure).reduce((sum, v) => (v < 0 ? sum + v : sum), 0),
  );
  const min = Math.min(0, ...netWorthValues, ...negativeSums);
  const max = Math.max(0, ...netWorthValues, ...positiveSums);
  const padding = (max - min) * 0.1 || 1;
  // Anchor the axis at 0 unless the data actually dips negative (a
  // liability-heavy household) - otherwise the 10% padding above pushes the
  // floor below zero for a purely positive series, which reads as if net
  // worth dips negative when it never does.
  const domain: [number, number] = [min < 0 ? Math.floor(min - padding) : 0, Math.ceil(max + padding)];

  // Stacking order is smallest-magnitude first (bottom of the stack),
  // ranked by each band's size at the most recent point - a fixed order is
  // required across the whole series (stacking can't reorder mid-chart),
  // and "current size" is the ranking people actually care about, matching
  // sortedExposureRows' own use of the latest point for the same reason.
  const latestPoint = series.points[series.points.length - 1];
  const orderedBands = [...series.exposureBands].sort(
    (a, b) => Math.abs(latestPoint?.byExposure[a] ?? 0) - Math.abs(latestPoint?.byExposure[b] ?? 0),
  );

  return (
    <ResponsiveContainer width="100%" height={360}>
      <ComposedChart data={chartData} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
        <CartesianGrid vertical={false} stroke={gridColor} strokeWidth={1} />
        <XAxis
          dataKey="date"
          tickFormatter={formatDateTick}
          tick={{ fill: axisColor, fontSize: 12 }}
          axisLine={{ stroke: gridColor }}
          tickLine={false}
          minTickGap={32}
        />
        <YAxis
          tickFormatter={(value: number) => formatCompactCurrency(value, displayCurrency)}
          tick={{ fill: axisColor, fontSize: 12 }}
          axisLine={false}
          tickLine={false}
          width={56}
          domain={domain}
        />
        <Tooltip
          content={
            <NetWorthTooltip mode={mode} mutedColor={axisColor} currency={displayCurrency} hovered={hovered} />
          }
          cursor={{ stroke: gridColor, strokeWidth: 1 }}
        />
        <Legend
          wrapperStyle={{ fontSize: 12, color: axisColor, cursor: "pointer" }}
          iconType="circle"
          iconSize={8}
          formatter={(value: string, entry) => {
            const key = entry.dataKey as string;
            const isHovered = hovered === key;
            return (
              <span style={{ color: isHovered ? lineColor : axisColor, fontWeight: isHovered ? 700 : 400 }}>
                {value}
              </span>
            );
          }}
          onMouseEnter={(entry) => setHovered((entry.dataKey as string) ?? null)}
          onMouseLeave={() => setHovered(null)}
        />
        {orderedBands.map((band) => {
          const color = EXPOSURE_COLORS[band]?.[mode] ?? axisColor;
          const isDimmed = hovered !== null && hovered !== band;
          return (
            <Area
              key={band}
              dataKey={band}
              name={band}
              stackId="bands"
              stroke={hovered === band ? color : surfaceColor}
              strokeWidth={hovered === band ? 2.5 : 1.5}
              fill={color}
              fillOpacity={isDimmed ? 0.15 : 0.75}
              isAnimationActive={false}
              onMouseEnter={() => setHovered(band)}
              onMouseLeave={() => setHovered(null)}
            />
          );
        })}
        <Line
          dataKey="netWorth"
          name="Net worth"
          stroke={lineColor}
          strokeWidth={hovered === "netWorth" ? 3.5 : 2.5}
          strokeOpacity={hovered !== null && hovered !== "netWorth" ? 0.25 : 1}
          dot={{ r: 4, fill: surfaceColor, strokeWidth: 2, stroke: lineColor }}
          activeDot={{ r: 6, fill: surfaceColor, strokeWidth: 2.5, stroke: lineColor }}
          isAnimationActive={false}
          onMouseEnter={() => setHovered("netWorth")}
          onMouseLeave={() => setHovered(null)}
        />
      </ComposedChart>
    </ResponsiveContainer>
  );
}
