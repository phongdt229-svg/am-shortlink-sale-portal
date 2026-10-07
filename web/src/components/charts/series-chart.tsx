"use client";
import { useState } from "react";
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { components } from "@/lib/api/schema";
import { fmtCompact, fmtDate, fmtNumber } from "@/lib/format";
import { cn } from "@/lib/utils";

type Series = components["schemas"]["Series"];
type Key = "clicks" | "unique_clicks" | "new_links" | "active_links" | "bot_clicks";

const OPTIONS: { key: Key; label: string }[] = [
  { key: "clicks", label: "Lượt click" },
  { key: "unique_clicks", label: "Khách duy nhất" },
  { key: "active_links", label: "Link có click" },
  { key: "new_links", label: "Link mới" },
  { key: "bot_clicks", label: "Click bot" },
];

const GRAN_LABEL = { day: "ngày", week: "tuần", month: "tháng" } as const;

/**
 * Timeline 1 chỉ số (chọn bằng nút) — không dùng 2 trục y. Kỳ trước: nét đứt màu trung tính, cùng vị trí bucket.
 */
export function SeriesChart({ series, initial = "clicks" }: { series: Series; initial?: Key }) {
  const [key, setKey] = useState<Key>(initial);
  const label = OPTIONS.find((o) => o.key === key)!.label;
  const prev = series.previous;
  const data = series.points.map((p, i) => ({
    period: p.period,
    cur: p[key],
    prev: prev?.[i]?.[key],
    prevPeriod: prev?.[i]?.period,
  }));

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1" role="group" aria-label="Chỉ số">
        {OPTIONS.map((o) => (
          <button
            key={o.key}
            type="button"
            onClick={() => setKey(o.key)}
            aria-pressed={key === o.key}
            className={cn("rounded-md px-2 py-1 text-xs", key === o.key ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:bg-muted")}
          >
            {o.label}
          </button>
        ))}
      </div>
      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
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
            <Tooltip
              cursor={{ stroke: "var(--muted-foreground)", strokeDasharray: "3 3" }}
              content={({ active, payload }) => {
                if (!active || !payload?.length) return null;
                const d = payload[0]!.payload as (typeof data)[number];
                return (
                  <div className="rounded-md border bg-card px-3 py-2 text-xs shadow-md">
                    <p className="font-medium">
                      {GRAN_LABEL[series.granularity]} {fmtDate(d.period)}
                    </p>
                    <p className="flex items-center gap-2">
                      <span className="inline-block h-0.5 w-3 bg-[var(--chart-1)]" /> {label}: <b className="tabular-nums">{fmtNumber(d.cur)}</b>
                    </p>
                    {prev && (
                      <p className="flex items-center gap-2 text-muted-foreground">
                        <span className="inline-block h-0 w-3 border-t-2 border-dashed border-[var(--chart-muted)]" /> Kỳ trước ({fmtDate(d.prevPeriod)}):{" "}
                        <span className="tabular-nums">{fmtNumber(d.prev)}</span>
                      </p>
                    )}
                  </div>
                );
              }}
            />
            {prev && <Legend iconType="plainline" wrapperStyle={{ fontSize: 12 }} />}
            {prev && (
              <Line name="Kỳ trước" dataKey="prev" stroke="var(--chart-muted)" strokeDasharray="5 4" strokeWidth={2} dot={false} isAnimationActive={false} />
            )}
            <Line name={label} dataKey="cur" stroke="var(--chart-1)" strokeWidth={2} dot={false} activeDot={{ r: 4, strokeWidth: 2, stroke: "var(--card)" }} isAnimationActive={false} />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
