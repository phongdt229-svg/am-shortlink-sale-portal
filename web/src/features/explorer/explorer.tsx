"use client";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Filter, Loader2, X } from "lucide-react";
import Link from "next/link";
import { parseAsString, useQueryState, useQueryStates } from "nuqs";
import { useMemo, useState } from "react";
import { DeltaCell } from "@/components/report/cells";
import { EmptyState, ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Badge, Card, CardContent, CardHeader, CardTitle, Select } from "@/components/ui/primitives";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import type { components } from "@/lib/api/schema";
import { filterParsers, toApiQuery, type Filters } from "@/lib/filters";
import { fmtDate, fmtDecimal, fmtNumber, fmtPercent } from "@/lib/format";
import type { Role } from "@/lib/session/session";
import { cn } from "@/lib/utils";
import { ExportButton } from "@/features/exports/export-button";
import { DIM_GROUPS, dimLabel, TIME_DIMS } from "./dims";
import { ExplorerChart, resolveChart } from "./explorer-chart";
import { FilterPanel } from "./filter-panel";
import { Pivot } from "./pivot";
import { SavedMenu } from "./saved-menu";
import { buildQuery, decodeState, drillDownHref, encodeState, type ChartKind, type ExplorerState, type Lock, type SortBy } from "./state";

type Result = components["schemas"]["ExplorerResult"];

const SORTS: { key: SortBy; label: string }[] = [
  { key: "clicks", label: "Lượt click" },
  { key: "unique_clicks", label: "Khách duy nhất" },
  { key: "active_links", label: "Link có click" },
  { key: "suspicious_clicks", label: "Nghi vấn" },
  { key: "bot_clicks", label: "Bot" },
  { key: "clicks_per_link", label: "Click / link" },
  { key: "key", label: "Theo tên nhóm" },
];
const CHARTS: { key: ChartKind; label: string }[] = [
  { key: "auto", label: "Tự chọn" },
  { key: "line", label: "Đường" },
  { key: "stacked", label: "Cột chồng" },
  { key: "bar", label: "Thanh ngang" },
  { key: "heatmap", label: "Heatmap giờ × thứ" },
  { key: "pie", label: "Tròn (tỉ trọng)" },
];

function DimSelect({
  value,
  onChange,
  params,
  optional,
  label,
}: {
  value?: string;
  onChange: (v?: string) => void;
  params: { key: string; label: string }[];
  optional?: boolean;
  label: string;
}) {
  return (
    <Select aria-label={label} className="h-8 max-w-44" value={value ?? ""} onChange={(e) => onChange(e.target.value || undefined)}>
      {optional && <option value="">— không —</option>}
      {DIM_GROUPS.map((g) => (
        <optgroup key={g.label} label={g.label}>
          {g.dims.map((d) => (
            <option key={d.key} value={d.key}>
              {d.label}
            </option>
          ))}
        </optgroup>
      ))}
      {params.length > 0 && (
        <optgroup label="Tham số URL">
          {params.map((p) => (
            <option key={p.key} value={`param.${p.key}`}>
              {p.label}
            </option>
          ))}
        </optgroup>
      )}
    </Select>
  );
}

function Total({ label, value, prev, hint }: { label: string; value: number; prev?: number; hint?: string }) {
  const ch = prev === undefined ? undefined : prev === 0 ? null : (value - prev) / prev;
  return (
    <div className="rounded-[var(--radius)] border bg-card p-3" title={hint}>
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-lg font-semibold tabular-nums">{fmtNumber(value)}</p>
      {ch !== undefined && <DeltaCell value={ch} />}
    </div>
  );
}

/**
 * Click Explorer (§4.1). `lock` = khoá theo đối tượng khi dùng làm tab "Lượt click" trong màn hình chi tiết.
 * Trạng thái (chiều, tiêu chí, biểu đồ) nằm trên URL `ex` → chia sẻ / lưu được.
 */
export function Explorer({ role, lock, compact }: { role: Role; lock?: Lock; compact?: boolean }) {
  const qc = useQueryClient();
  const [g] = useQueryStates(filterParsers);
  const [raw, setRaw] = useQueryState("ex", parseAsString);
  const st = useMemo(() => decodeState(raw), [raw]);
  const setSt = (patch: Partial<ExplorerState>) => void setRaw(encodeState({ ...st, ...patch }));
  const [showFilters, setShowFilters] = useState(false);

  const base = toApiQuery(g as Filters);
  const body = buildQuery(st, { ...base }, lock);
  const accounts = body.account ?? [];

  const params = useQuery({
    queryKey: ["params-registry-light"],
    queryFn: async () => {
      const today = new Date().toISOString().slice(0, 10);
      return unwrap(await browserApi.GET("/v1/reports/params", { params: { query: { from: today, to: today } } }));
    },
    staleTime: 10 * 60_000,
  });
  const paramDims = (params.data?.items ?? []).map((p) => ({ key: p.key, label: p.label }));

  const res = useQuery({
    queryKey: ["explorer", body],
    queryFn: async ({ signal }) => unwrap(await browserApi.POST("/v1/reports/clicks/query", { body, signal })),
    staleTime: 60_000,
  });
  const data: Result | undefined = res.data;
  const chart = resolveChart(st.c, st.g);
  const filterCount = Object.keys(body.filters ?? {}).length - (lock?.links ? 1 : 0);

  return (
    <div className="space-y-4">
      <Card>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <span className="text-muted-foreground">Nhóm theo</span>
            {[0, 1, 2]
              .map((i) => (
                <DimSelect
                  key={i}
                  label={`Chiều ${i + 1}`}
                  optional={i > 0}
                  params={paramDims}
                  value={st.g[i]}
                  onChange={(v) => {
                    const next = [...st.g];
                    if (v) next[i] = v;
                    else next.splice(i);
                    setSt({ g: [...new Set(next.filter(Boolean))] });
                  }}
                />
              ))
              .slice(0, Math.min(3, st.g.length + 1))}
            <span className="ml-2 text-muted-foreground">Sắp xếp</span>
            <Select className="h-8" aria-label="Sắp xếp theo" value={st.s ?? "clicks"} onChange={(e) => setSt({ s: e.target.value as SortBy })}>
              {SORTS.map((s) => (
                <option key={s.key} value={s.key}>
                  {s.label}
                </option>
              ))}
            </Select>
            <Select className="h-8" aria-label="Số nhóm" value={st.l ?? 100} onChange={(e) => setSt({ l: Number(e.target.value) })}>
              {[10, 20, 50, 100, 500, 1000].map((n) => (
                <option key={n} value={n}>
                  Top {n}
                </option>
              ))}
            </Select>
            <Select className="h-8" aria-label="Biểu đồ" value={st.c ?? "auto"} onChange={(e) => setSt({ c: e.target.value as ChartKind })}>
              {CHARTS.map((c) => (
                <option key={c.key} value={c.key}>
                  {c.label}
                </option>
              ))}
            </Select>
            <Button variant={showFilters ? "secondary" : "outline"} size="sm" onClick={() => setShowFilters((v) => !v)} aria-expanded={showFilters}>
              <Filter /> Lọc chi tiết {filterCount > 0 && <Badge tone="primary">{filterCount}</Badge>}
            </Button>
            {filterCount > 0 && (
              <Button variant="ghost" size="sm" onClick={() => setSt({ f: {} })}>
                <X /> Bỏ lọc chi tiết
              </Button>
            )}
            {!compact && <SavedMenu state={st} globals={base} />}
            {!compact && <ExportButton kind="explorer" />}
          </div>
          {showFilters && (
            <div className="border-t pt-3">
              <FilterPanel value={st.f} onChange={(f) => setSt({ f })} accounts={accounts} isAdmin={role === "admin"} lockLinks={!!lock?.links} />
            </div>
          )}
        </CardContent>
      </Card>

      {res.isFetching && (
        <div className="flex items-center gap-3 text-sm text-muted-foreground" role="status">
          <Loader2 className="size-4 animate-spin" /> Đang tính…
          <Button variant="ghost" size="sm" onClick={() => void qc.cancelQueries({ queryKey: ["explorer"] })}>
            Huỷ
          </Button>
        </div>
      )}
      {res.isError && <ErrorState error={res.error} />}

      {data && (
        <>
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <Badge tone={data.source === "stats" ? "success" : "primary"}>
              {data.source === "stats" ? "Số liệu tổng hợp (≤ 12 tháng)" : "Click thô (≤ 3 tháng)"}
            </Badge>
            <span>
              {fmtDate(data.period.from)} – {fmtDate(data.period.to)} · {fmtNumber(data.group_count ?? data.rows.length)} nhóm
            </span>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-7">
            <Total label="Lượt click" value={data.totals.metrics.clicks} prev={data.previous_totals?.metrics.clicks} hint="Không tính bot" />
            <Total label="Khách duy nhất" value={data.totals.metrics.unique_clicks} prev={data.previous_totals?.metrics.unique_clicks} />
            <Total label="Click bot" value={data.totals.metrics.bot_clicks} prev={data.previous_totals?.metrics.bot_clicks} />
            <Total label="Nghi vấn" value={data.totals.metrics.suspicious_clicks} prev={data.previous_totals?.metrics.suspicious_clicks} />
            <Total label="Link có click" value={data.totals.metrics.active_links} prev={data.previous_totals?.metrics.active_links} />
            <Total label="Tài khoản" value={data.totals.accounts} prev={data.previous_totals?.accounts} />
            <Total label="CTV" value={data.totals.ctvs} prev={data.previous_totals?.ctvs} />
          </div>
          {data.rows.length === 0 ? (
            <EmptyState hint="Thử nới bộ lọc hoặc khoảng ngày" />
          ) : (
            <>
              <Card>
                <CardHeader>
                  <CardTitle>{st.g.map(dimLabel).join(" × ")}</CardTitle>
                </CardHeader>
                <CardContent>
                  <ExplorerChart rows={data.rows} dims={data.group_by} kind={chart} />
                </CardContent>
              </Card>
              {data.group_by.length >= 2 && (
                <Card>
                  <CardHeader>
                    <CardTitle>Bảng pivot</CardTitle>
                    <span className="text-xs text-muted-foreground">Bấm một ô để xem click thô</span>
                  </CardHeader>
                  <CardContent className="pt-2">
                    <Pivot rows={data.rows} dims={data.group_by} query={body} />
                  </CardContent>
                </Card>
              )}
              <ResultTable data={data} query={body} />
            </>
          )}
        </>
      )}
    </div>
  );
}

function ResultTable({ data, query }: { data: Result; query: ReturnType<typeof buildQuery> }) {
  const compare = !!data.previous_totals;
  const rows = data.other ? [...data.rows, data.other] : data.rows;
  return (
    <Card>
      <CardHeader>
        <CardTitle>Chi tiết theo nhóm</CardTitle>
        {data.truncated && <span className="text-xs text-muted-foreground">Phần còn lại gộp vào &quot;Khác&quot;</span>}
      </CardHeader>
      <CardContent className="overflow-x-auto pt-2">
        <table className="w-full text-sm">
          <thead className="text-xs text-muted-foreground">
            <tr>
              {data.group_by.map((d) => (
                <th key={d} className="px-2 py-1.5 text-left font-normal">
                  {dimLabel(d)}
                </th>
              ))}
              {["Lượt click", "% tổng", "Khách", "Bot", "Nghi vấn", "Link có click", "Click/link", "Click cuối"].map((h) => (
                <th key={h} className="px-2 py-1.5 text-right font-normal">
                  {h}
                </th>
              ))}
              {compare && <th className="px-2 py-1.5 text-right font-normal">So kỳ trước</th>}
              <th />
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => {
              const isOther = r.keys[0]?.dim === "other";
              return (
                <tr key={i} className={cn("border-t", isOther && "text-muted-foreground")}>
                  {isOther ? (
                    <td colSpan={data.group_by.length} className="px-2 py-1.5 italic">
                      {r.keys[0]!.label}
                    </td>
                  ) : (
                    r.keys.map((k) => (
                      <td key={k.dim} className="max-w-60 truncate px-2 py-1.5" title={k.label}>
                        {TIME_DIMS.has(k.dim) ? fmtDate(k.label) : k.label}
                      </td>
                    ))
                  )}
                  <td className="px-2 py-1.5 text-right font-medium tabular-nums">{fmtNumber(r.metrics.clicks)}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-muted-foreground">{fmtPercent(r.share)}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.unique_clicks)}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.bot_clicks)}</td>
                  <td className={cn("px-2 py-1.5 text-right tabular-nums", r.metrics.suspicious_clicks > 0 && !isOther && "text-destructive")}>
                    {fmtNumber(r.metrics.suspicious_clicks)}
                  </td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.active_links)}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums">{fmtDecimal(r.metrics.clicks_per_link)}</td>
                  <td className="px-2 py-1.5 text-right tabular-nums text-muted-foreground">
                    {r.metrics.last_click_at ? fmtDate(r.metrics.last_click_at) : "–"}
                  </td>
                  {compare && (
                    <td className="px-2 py-1.5 text-right">
                      <DeltaCell value={r.change_clicks} />
                    </td>
                  )}
                  <td className="px-2 py-1.5 text-right">
                    {!isOther && (
                      <Link href={drillDownHref(query, r.keys)} className="text-xs text-primary hover:underline">
                        Click thô →
                      </Link>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}
