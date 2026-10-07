import { AlertTriangle, Info } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { createSearchParamsCache, parseAsString } from "nuqs/server";
import { PageHeader } from "@/components/report/page-header";
import { EmptyState } from "@/components/states";
import { Badge, Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { encodeState } from "@/features/explorer/state";
import { ParamSeries } from "@/features/params/param-series";
import { ParamValuesTable } from "@/features/params/param-values-table";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";
import { fmtDate, fmtNumber } from "@/lib/format";
import { cn } from "@/lib/utils";

export const metadata: Metadata = { title: "Tham số URL" };

const keyCache = createSearchParamsCache({ key: parseAsString });

export default async function ParamsPage({ searchParams }: { searchParams: SearchParams }) {
  const { api: query, table, search } = await pageContext(searchParams);
  const { key } = keyCache.parse(await searchParams);
  const api = await serverApi();
  const list = data(
    await api.GET("/v1/reports/params", { params: { query: { from: query.from, to: query.to, account: query.account, campaign: query.campaign } } }),
  );
  const selected = key ?? list.items[0]?.key;
  const report = selected
    ? data(
        await api.GET("/v1/reports/params/{key}", {
          params: {
            path: { key: selected },
            query: { ...query, ctv: undefined, prefix: undefined, page: table.page, page_size: 50, sort: table.sort, order: table.order, q: table.q },
          },
        }),
      )
    : null;
  const sep = search ? "&" : "?";
  const exploreHref = (g: string[]) => `/clicks/explore${search}${sep}ex=${encodeState({ g, f: {} })}`;
  const others = list.items.filter((p) => p.key !== selected).slice(0, 4);

  return (
    <div className="space-y-4">
      <PageHeader title="Báo cáo theo tham số URL" subtitle="Tham số đang theo dõi trên long URL (utm_*, ref, promo…) — tham số PII không hiển thị" />
      {list.items.length === 0 ? (
        <EmptyState title="Chưa có tham số nào được theo dõi" />
      ) : (
        <div className="flex flex-wrap gap-2" role="tablist" aria-label="Tham số">
          {list.items.map((p) => (
            <Link
              key={p.key}
              href={`/params${search}${sep}key=${encodeURIComponent(p.key)}`}
              role="tab"
              aria-selected={p.key === selected}
              className={cn("rounded-md border px-3 py-2 text-left text-sm hover:bg-muted", p.key === selected && "border-primary bg-accent")}
            >
              <span className="font-medium">{p.label}</span>
              <span className="block text-xs text-muted-foreground">
                {fmtNumber(p.values)} giá trị · {fmtNumber(p.links)} link · {fmtNumber(p.clicks)} click
              </span>
            </Link>
          ))}
        </div>
      )}
      {report && (
        <>
          {report.warnings.map((w) => (
            <div key={w.code} className="flex items-center gap-2 rounded-md border border-warning/50 bg-warning/10 px-3 py-2 text-sm">
              {w.level === "info" ? <Info className="size-4" /> : <AlertTriangle className="size-4" />}
              {w.message}
            </div>
          ))}
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <span className="text-muted-foreground">Kết hợp trong Click Explorer:</span>
            {others.map((o) => (
              <Link key={o.key} href={exploreHref([`param.${selected}`, `param.${o.key}`])} className="text-primary hover:underline">
                × {o.label}
              </Link>
            ))}
            {[
              ["account", "tài khoản"],
              ["campaign", "chiến dịch"],
              ["ctv", "CTV"],
              ["device", "thiết bị"],
              ["source_group", "nhóm nguồn"],
            ].map(([d, l]) => (
              <Link key={d} href={exploreHref([`param.${selected}`, d!])} className="text-primary hover:underline">
                × {l}
              </Link>
            ))}
            {report.param.tracked_since && <Badge>Theo dõi từ {fmtDate(report.param.tracked_since)}</Badge>}
            {report.param.status === "high_cardinality" && <Badge tone="warning">Nhiều giá trị</Badge>}
          </div>
          <Card>
            <CardHeader>
              <CardTitle>Top 5 giá trị theo thời gian</CardTitle>
            </CardHeader>
            <CardContent>
              <ParamSeries series={report.top_series} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Giá trị của {report.param.label}</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              <ParamValuesTable report={report} paramKey={selected!} compare={!!query.compare} />
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}
