"use client";
import { useQuery } from "@tanstack/react-query";
import { parseAsArrayOf, parseAsString, useQueryState } from "nuqs";
import { useState, useTransition } from "react";
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { MultiSelect } from "@/components/filters/multi-select";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import type { components } from "@/lib/api/schema";
import { fmtCompact, fmtDate, fmtDecimal, fmtNumber } from "@/lib/format";
import { cn } from "@/lib/utils";

type Compare = components["schemas"]["CampaignCompare"];
type Key = "clicks" | "unique_clicks" | "new_links" | "active_links";

// Màu theo THỨ TỰ CHỌN (thực thể), không theo hạng — bỏ 1 chiến dịch không đổi màu các chiến dịch còn lại.
const COLORS = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)"];
const METRICS: { key: Key; label: string }[] = [
  { key: "clicks", label: "Lượt click" },
  { key: "unique_clicks", label: "Khách duy nhất" },
  { key: "active_links", label: "Link có click" },
  { key: "new_links", label: "Link mới" },
];

export function CompareView({ data, accounts }: { data: Compare | null; accounts: string[] }) {
  const [pending, startTransition] = useTransition();
  const [codes, setCodes] = useQueryState("codes", parseAsArrayOf(parseAsString).withDefault([]).withOptions({ shallow: false, startTransition }));
  const [metric, setMetric] = useState<Key>("clicks");
  const [q, setQ] = useState("");
  const options = useQuery({
    queryKey: ["filters", "campaigns", q, accounts],
    queryFn: async () =>
      unwrap(await browserApi.GET("/v1/filters/campaigns", { params: { query: { q: q || undefined, limit: 100, account: accounts.length ? accounts : undefined } } })),
  });

  const items = data?.items ?? [];
  const color = (code: string) => COLORS[Math.max(0, codes.indexOf(code)) % COLORS.length];
  const rows = (items[0]?.series ?? []).map((p, i) => {
    const r: Record<string, string | number> = { period: p.period };
    for (const it of items) r[it.code] = it.series[i]?.[metric] ?? 0;
    return r;
  });

  return (
    <div className={cn("space-y-4", pending && "opacity-70")}>
      <div className="flex flex-wrap items-center gap-2">
        <MultiSelect
          label="Chiến dịch so sánh"
          value={codes}
          onChange={(v) => void setCodes(v.slice(0, 5).length ? v.slice(0, 5) : null)}
          options={(options.data?.items ?? []).map((c) => ({ value: c.code, label: c.code, hint: c.name !== c.code ? c.name : undefined }))}
          loading={options.isFetching}
          onSearch={setQ}
        />
        <span className="text-xs text-muted-foreground">Chọn 2–5 chiến dịch</span>
      </div>
      {codes.length < 2 ? (
        <p className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">Chọn ít nhất 2 chiến dịch để so sánh.</p>
      ) : (
        <>
          <Card>
            <CardHeader>
              <CardTitle>Diễn biến</CardTitle>
              <div className="flex gap-1" role="group" aria-label="Chỉ số">
                {METRICS.map((m) => (
                  <button
                    key={m.key}
                    type="button"
                    aria-pressed={metric === m.key}
                    onClick={() => setMetric(m.key)}
                    className={cn("rounded-md px-2 py-1 text-xs", metric === m.key ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:bg-muted")}
                  >
                    {m.label}
                  </button>
                ))}
              </div>
            </CardHeader>
            <CardContent className="h-72">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={rows} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
                  <CartesianGrid stroke="var(--border)" vertical={false} />
                  <XAxis dataKey="period" tickFormatter={(v: string) => fmtDate(v).slice(0, 5)} tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} axisLine={false} tickLine={false} minTickGap={24} />
                  <YAxis tickFormatter={fmtCompact} tick={{ fontSize: 11, fill: "var(--muted-foreground)" }} axisLine={false} tickLine={false} width={48} />
                  <Tooltip
                    content={({ active, payload, label }) =>
                      active && payload?.length ? (
                        <div className="rounded-md border bg-card px-3 py-2 text-xs shadow-md">
                          <p className="font-medium">{fmtDate(String(label))}</p>
                          {[...payload].sort((a, b) => Number(b.value) - Number(a.value)).map((p) => (
                            <p key={String(p.dataKey)} className="flex items-center gap-2">
                              <span className="inline-block h-0.5 w-3" style={{ background: p.color }} /> {String(p.dataKey)}: <b className="tabular-nums">{fmtNumber(Number(p.value))}</b>
                            </p>
                          ))}
                        </div>
                      ) : null
                    }
                  />
                  <Legend iconType="plainline" wrapperStyle={{ fontSize: 12 }} />
                  {items.map((it) => (
                    <Line key={it.code} dataKey={it.code} stroke={color(it.code)} strokeWidth={2} dot={false} isAnimationActive={false} />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Chỉ số cạnh nhau</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto pt-2">
              <table className="w-full text-sm">
                <thead className="text-xs text-muted-foreground">
                  <tr>
                    <th className="py-1 text-left font-normal">Chiến dịch</th>
                    {["Link mới", "Link có click", "Lượt click", "Khách", "Bot", "Nghi vấn", "Click/link"].map((h) => (
                      <th key={h} className="py-1 text-right font-normal">
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {items.map((it) => (
                    <tr key={it.code} className="border-t">
                      <td className="py-1.5">
                        <span className="mr-2 inline-block size-2.5 rounded-full align-middle" style={{ background: color(it.code) }} />
                        {it.code} <span className="text-xs text-muted-foreground">{it.name !== it.code ? it.name : ""}</span>
                      </td>
                      {[it.metrics.new_links, it.metrics.active_links, it.metrics.clicks, it.metrics.unique_clicks, it.metrics.bot_clicks, it.metrics.suspicious_clicks].map((v, i) => (
                        <td key={i} className="py-1.5 text-right tabular-nums">
                          {fmtNumber(v)}
                        </td>
                      ))}
                      <td className="py-1.5 text-right tabular-nums">{fmtDecimal(it.metrics.ctr_per_link)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}
