"use client";
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { EmptyState } from "@/components/states";
import type { components } from "@/lib/api/schema";
import { fmtCompact, fmtDate, fmtNumber } from "@/lib/format";

type TopSeries = components["schemas"]["ParamReport"]["top_series"];

// Màu theo thứ tự giá trị (top 1 → slot 1…), tối đa 5 chuỗi — không sinh màu mới.
const COLORS = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)"];

export function ParamSeries({ series }: { series: TopSeries }) {
  if (!series.length) return <EmptyState />;
  const data = series[0]!.points.map((p, i) => {
    const row: Record<string, string | number> = { period: p.period };
    for (const s of series) row[s.value] = s.points[i]?.clicks ?? 0;
    return row;
  });
  return (
    <div className="h-64">
      <ResponsiveContainer>
        <LineChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
          <CartesianGrid stroke="var(--border)" vertical={false} />
          <XAxis
            dataKey="period"
            tickFormatter={(v: string) => fmtDate(v).slice(0, 5)}
            tick={{ fontSize: 11, fill: "var(--muted-foreground)" }}
            axisLine={false}
            tickLine={false}
            minTickGap={24}
          />
          <YAxis tickFormatter={fmtCompact} tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} axisLine={false} tickLine={false} width={48} />
          <Tooltip labelFormatter={(v) => fmtDate(String(v))} formatter={(v) => fmtNumber(Number(v))} />
          <Legend wrapperStyle={{ fontSize: 12 }} iconType="plainline" />
          {series.map((s, i) => (
            <Line
              key={s.value}
              dataKey={s.value}
              name={s.value || "(rỗng)"}
              stroke={COLORS[i % COLORS.length]}
              strokeWidth={2}
              dot={false}
              isAnimationActive={false}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
