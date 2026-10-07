"use client";
import { Bar, BarChart, CartesianGrid, Cell, Legend, Line, LineChart, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Heatmap } from "@/components/charts/heatmap";
import { fmtCompact, fmtDate, fmtNumber, fmtPercent } from "@/lib/format";
import { TIME_DIMS } from "./dims";
import type { ChartKind, ExplorerRow } from "./state";

// Màu theo slot cố định; quá 5 chuỗi gộp "Khác" (không sinh màu mới).
const COLORS = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)"];
const OTHER = "var(--chart-muted)";

export function resolveChart(kind: ChartKind | undefined, dims: string[]): Exclude<ChartKind, "auto"> {
  if (kind && kind !== "auto") return kind;
  const d0 = dims[0] ?? "";
  if (dims.length === 2 && dims.includes("hour") && dims.includes("weekday")) return "heatmap";
  if (TIME_DIMS.has(d0)) return dims.length > 1 ? "stacked" : "line";
  return "bar";
}

const tick = { fontSize: 11, fill: "var(--muted-foreground)" };

function labelOf(dim: string, label: string) {
  return TIME_DIMS.has(dim) ? fmtDate(label) : label;
}

/** Biểu đồ tự chọn theo nhóm (§4.1c): line / stacked bar / bar ngang / heatmap / pie (≤ 6 phần). */
export function ExplorerChart({ rows, dims, kind }: { rows: ExplorerRow[]; dims: string[]; kind: Exclude<ChartKind, "auto"> }) {
  if (!rows.length) return null;

  if (kind === "heatmap") {
    const hi = dims.indexOf("hour");
    const wi = dims.indexOf("weekday");
    const cells = rows.map((r) => ({ hour: Number(r.keys[hi]!.value), weekday: Number(r.keys[wi]!.value), clicks: r.metrics.clicks }));
    return <Heatmap cells={cells} />;
  }

  if (kind === "pie") {
    const top = rows.slice(0, 5);
    const rest = rows.slice(5).reduce((a, r) => a + r.metrics.clicks, 0);
    const data = top.map((r) => ({ name: r.keys.map((k) => labelOf(k.dim, k.label)).join(" · "), value: r.metrics.clicks }));
    if (rest > 0) data.push({ name: "Khác", value: rest });
    const total = data.reduce((a, d) => a + d.value, 0);
    return (
      <div className="h-72">
        <ResponsiveContainer>
          <PieChart>
            <Pie data={data} dataKey="value" nameKey="name" innerRadius="45%" outerRadius="80%" stroke="var(--card)" strokeWidth={2} isAnimationActive={false}>
              {data.map((d, i) => (
                <Cell key={d.name} fill={d.name === "Khác" ? OTHER : COLORS[i % COLORS.length]} />
              ))}
            </Pie>
            <Tooltip formatter={(v: number) => `${fmtNumber(v)} (${fmtPercent(v / total)})`} />
            <Legend wrapperStyle={{ fontSize: 12 }} />
          </PieChart>
        </ResponsiveContainer>
      </div>
    );
  }

  if (kind === "line" || kind === "stacked") {
    // Trục x = chiều 1 (thời gian); chuỗi = chiều 2 (top 5 + Khác) nếu có.
    const xDim = dims[0]!;
    const xs = [...new Set(rows.map((r) => r.keys[0]!.value))].sort();
    const series: string[] = [];
    const totals = new Map<string, number>();
    if (dims.length > 1) {
      for (const r of rows) totals.set(r.keys[1]!.label, (totals.get(r.keys[1]!.label) ?? 0) + r.metrics.clicks);
      series.push(...[...totals.entries()].sort((a, b) => b[1] - a[1]).slice(0, 5).map(([k]) => k));
    }
    const data = xs.map((x) => {
      const row: Record<string, string | number> = { x };
      for (const r of rows.filter((r) => r.keys[0]!.value === x)) {
        const s = dims.length > 1 ? (series.includes(r.keys[1]!.label) ? r.keys[1]!.label : "Khác") : "Lượt click";
        row[s] = ((row[s] as number) ?? 0) + r.metrics.clicks;
      }
      return row;
    });
    const keys = dims.length > 1 ? [...series, ...(data.some((d) => d["Khác"]) ? ["Khác"] : [])] : ["Lượt click"];
    const fmtX = (v: string) => (TIME_DIMS.has(xDim) ? fmtDate(v).slice(0, 5) : v);
    const common = (
      <>
        <CartesianGrid stroke="var(--border)" vertical={false} />
        <XAxis dataKey="x" tickFormatter={fmtX} tick={tick} axisLine={false} tickLine={false} minTickGap={20} />
        <YAxis tickFormatter={fmtCompact} tick={tick} axisLine={false} tickLine={false} width={48} />
        <Tooltip labelFormatter={(v) => (TIME_DIMS.has(xDim) ? fmtDate(String(v)) : String(v))} formatter={(v: number) => fmtNumber(v)} />
        {keys.length > 1 && <Legend wrapperStyle={{ fontSize: 12 }} />}
      </>
    );
    return (
      <div className="h-72">
        <ResponsiveContainer>
          {kind === "line" ? (
            <LineChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
              {common}
              {keys.map((k, i) => (
                <Line key={k} dataKey={k} stroke={k === "Khác" ? OTHER : COLORS[i % COLORS.length]} strokeWidth={2} dot={false} isAnimationActive={false} />
              ))}
            </LineChart>
          ) : (
            <BarChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
              {common}
              {keys.map((k, i) => (
                <Bar key={k} dataKey={k} stackId="a" fill={k === "Khác" ? OTHER : COLORS[i % COLORS.length]} stroke="var(--card)" strokeWidth={1} isAnimationActive={false} />
              ))}
            </BarChart>
          )}
        </ResponsiveContainer>
      </div>
    );
  }

  // Bar ngang top 15 — một màu (độ lớn), nhãn đầy đủ trên trục.
  const top = rows.slice(0, 15).map((r) => ({ name: r.keys.map((k) => labelOf(k.dim, k.label)).join(" · "), clicks: r.metrics.clicks }));
  return (
    <div style={{ height: Math.max(160, top.length * 26 + 40) }}>
      <ResponsiveContainer>
        <BarChart data={top} layout="vertical" margin={{ top: 4, right: 40, left: 8, bottom: 4 }}>
          <CartesianGrid stroke="var(--border)" horizontal={false} />
          <XAxis type="number" tickFormatter={fmtCompact} tick={tick} axisLine={false} tickLine={false} />
          <YAxis type="category" dataKey="name" width={180} tick={tick} axisLine={false} tickLine={false} />
          <Tooltip formatter={(v: number) => fmtNumber(v)} cursor={{ fill: "var(--muted)" }} />
          <Bar dataKey="clicks" name="Lượt click" fill="var(--chart-1)" radius={[0, 4, 4, 0]} isAnimationActive={false} label={{ position: "right", fontSize: 11, fill: "var(--muted-foreground)", formatter: (v: number) => fmtCompact(v) }} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
