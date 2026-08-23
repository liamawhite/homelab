import { Area, CartesianGrid, ComposedChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { useTheme } from "@/lib/theme";
import { useDisplayCurrency } from "@/lib/displayCurrency";
import { EXPOSURE_COLORS } from "@/lib/exposureColors";
import { formatCurrency, formatCompactCurrency } from "@/lib/money";
import { sortedExposureRows } from "@/lib/netWorth";
import { Currency } from "@/gen/finances/v1/asset_pb";
import type { NetWorthPoint, NetWorthSeries } from "@/lib/netWorth";

// Sequential default hue (see dataviz skill's palette.md) - the chart's
// one plotted series carries its own identity color.
const LINE_COLOR = { light: "#2a78d6", dark: "#3987e5" };

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
}

function NetWorthTooltip({ active, label, payload, mode, mutedColor, currency }: NetWorthTooltipProps) {
  if (!active || !payload || payload.length === 0 || !label) return null;
  const point = payload[0].payload;
  const rows = sortedExposureRows(point);

  return (
    <div className="grid w-56 gap-2 rounded-lg border bg-popover p-3 text-xs shadow-md">
      <div className="flex items-center justify-between gap-4">
        <span className="font-medium text-popover-foreground">{formatDateTick(label)}</span>
        <span className="tabular-nums font-medium text-popover-foreground">
          {formatCurrency(point.netWorth, currency)}
        </span>
      </div>

      <div className="grid gap-1 border-t pt-2">
        {rows.map(([band, value]) => (
          <div key={band} className="flex items-center justify-between gap-4">
            <span className="flex items-center gap-1.5" style={{ color: mutedColor }}>
              <span
                className="size-2 rounded-full"
                style={{ backgroundColor: EXPOSURE_COLORS[band]?.[mode] ?? mutedColor }}
              />
              {band}
            </span>
            <span className="tabular-nums text-popover-foreground">{formatCurrency(value, currency)}</span>
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

  const gridColor = mode === "dark" ? "#2b2f33" : "#eae8e2";
  const axisColor = mode === "dark" ? "#9aa0a6" : "#5b6169";
  const lineColor = LINE_COLOR[mode];

  const values = series.points.map((p) => p.netWorth);
  const min = Math.min(0, ...values);
  const max = Math.max(0, ...values);
  const padding = (max - min) * 0.1 || 1;
  // Anchor the axis at 0 unless the data actually dips negative (a
  // liability-heavy household) - otherwise the 10% padding above pushes the
  // floor below zero for a purely positive series, which reads as if net
  // worth dips negative when it never does.
  const domain: [number, number] = [min < 0 ? Math.floor(min - padding) : 0, Math.ceil(max + padding)];

  return (
    <ResponsiveContainer width="100%" height={300}>
      <ComposedChart data={series.points} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
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
          content={<NetWorthTooltip mode={mode} mutedColor={axisColor} currency={displayCurrency} />}
          cursor={{ stroke: gridColor, strokeWidth: 1 }}
        />
        <Area
          dataKey="netWorth"
          name="Net worth"
          stroke={lineColor}
          strokeWidth={2}
          fill={lineColor}
          fillOpacity={0.1}
          dot={{ r: 4, fill: lineColor, strokeWidth: 2, stroke: mode === "dark" ? "#1b1f23" : "#ffffff" }}
          activeDot={{ r: 5 }}
          isAnimationActive={false}
        />
      </ComposedChart>
    </ResponsiveContainer>
  );
}
