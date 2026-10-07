import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { PageHeader } from "@/components/report/page-header";
import { SOURCE_LABELS } from "@/components/report/metric-defs";
import { EmptyState } from "@/components/states";
import { Badge, Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";
import { fmtDateTime, fmtNumber, fmtPercent } from "@/lib/format";
import { getSession } from "@/lib/session/server";
import type { components } from "@/lib/api/schema";

export const metadata: Metadata = { title: "Chất lượng traffic" };

type RepeatRow = components["schemas"]["RepeatRow"];

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-[var(--radius)] border bg-card p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-xl font-semibold tabular-nums">{value}</p>
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  );
}

function RepeatTable({ rows, href }: { rows: RepeatRow[]; href: (r: RepeatRow) => string }) {
  if (!rows.length) return <EmptyState className="py-6" hint="Cần ≥ 20 click để tính tỉ lệ" />;
  return (
    <table className="w-full text-sm">
      <thead className="text-xs text-muted-foreground">
        <tr>
          <th className="py-1 text-left font-normal">Tên</th>
          <th className="py-1 text-right font-normal">Click</th>
          <th className="py-1 text-right font-normal">Tỉ lệ lặp</th>
          <th className="py-1 text-right font-normal">Nghi vấn</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.key} className="border-t">
            <td className="max-w-48 truncate py-1.5">
              <Link href={href(r)} className="text-primary hover:underline">
                {r.label}
              </Link>
            </td>
            <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.clicks)}</td>
            <td className="py-1.5 text-right tabular-nums">{fmtPercent(r.repeat_rate)}</td>
            <td className={`py-1.5 text-right tabular-nums ${r.suspicious_clicks ? "text-destructive" : ""}`}>{fmtNumber(r.suspicious_clicks)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export default async function TrafficQualityPage({ searchParams }: { searchParams: SearchParams }) {
  const s = await getSession();
  if (s?.user.role !== "admin") notFound();
  const { api: query, search } = await pageContext(searchParams);
  const api = await serverApi();
  const r = data(
    await api.GET("/v1/reports/traffic-quality", {
      params: { query: { from: query.from, to: query.to, account: query.account, campaign: query.campaign, ctv: query.ctv, prefix: query.prefix } },
    }),
  );
  const t = r.totals;
  const all = t.clicks + t.bot_clicks;
  return (
    <div className="space-y-4">
      <PageHeader title="Chất lượng traffic" subtitle="Soát gian lận CTV · click thô tối đa 3 tháng" />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Lượt click hợp lệ" value={fmtNumber(t.clicks)} />
        <Stat label="Click bot" value={fmtNumber(t.bot_clicks)} hint={`${fmtPercent(all ? t.bot_clicks / all : 0)} lượt truy cập`} />
        <Stat label="Click nghi vấn" value={fmtNumber(t.suspicious_clicks)} hint={`${fmtPercent(t.clicks ? t.suspicious_clicks / t.clicks : 0)} lượt click`} />
        <Stat label="Click lặp lại" value={fmtNumber(t.repeat_clicks)} hint={`${fmtPercent(t.clicks ? t.repeat_clicks / t.clicks : 0)} lượt click`} />
      </div>
      <div className="grid gap-4 xl:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>Tỉ lệ click lặp theo CTV</CardTitle>
          </CardHeader>
          <CardContent className="pt-2">
            <RepeatTable rows={r.repeat_by_ctv} href={(x) => `/ctvs/${encodeURIComponent(x.key)}${search}`} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Tỉ lệ click lặp theo chiến dịch</CardTitle>
          </CardHeader>
          <CardContent className="pt-2">
            <RepeatTable rows={r.repeat_by_campaign} href={(x) => `/campaigns/${encodeURIComponent(x.key)}${search}`} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Tỉ lệ bot theo nguồn</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 pt-2">
            {r.bot_by_source.map((b) => (
              <div
                key={b.source_group}
                className="grid grid-cols-[6rem_1fr_6rem] items-center gap-2 text-sm"
                title={`${fmtNumber(b.bot_clicks)} / ${fmtNumber(b.total)}`}
              >
                <span>{SOURCE_LABELS[b.source_group] ?? b.source_group}</span>
                <span className="h-3 rounded-r bg-muted">
                  <span className="block h-3 rounded-r bg-[var(--chart-1)]" style={{ width: `${Math.min(100, b.bot_rate * 100)}%` }} />
                </span>
                <span className="text-right text-xs tabular-nums">
                  {fmtPercent(b.bot_rate)} <span className="text-muted-foreground">({fmtNumber(b.bot_clicks)})</span>
                </span>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Top IP</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto pt-2">
          <table className="w-full text-sm">
            <thead className="text-xs text-muted-foreground">
              <tr>
                {["IP", "Quốc gia", "Click hợp lệ", "Bot", "Nghi vấn", "Số link", "Số tài khoản", "Lần cuối"].map((h) => (
                  <th key={h} className="px-2 py-1.5 text-left font-normal">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {r.top_ips.slice(0, 20).map((ip) => (
                <tr key={ip.ip} className="border-t">
                  <td className="px-2 py-1.5 font-mono text-xs">{ip.ip}</td>
                  <td className="px-2 py-1.5">{ip.country}</td>
                  <td className="px-2 py-1.5 tabular-nums">{fmtNumber(ip.clicks)}</td>
                  <td className="px-2 py-1.5 tabular-nums">{fmtNumber(ip.bot_clicks)}</td>
                  <td className={`px-2 py-1.5 tabular-nums ${ip.suspicious_clicks ? "text-destructive" : ""}`}>{fmtNumber(ip.suspicious_clicks)}</td>
                  <td className="px-2 py-1.5 tabular-nums">{fmtNumber(ip.links)}</td>
                  <td className="px-2 py-1.5 tabular-nums">{fmtNumber(ip.accounts)}</td>
                  <td className="px-2 py-1.5 tabular-nums text-muted-foreground">{fmtDateTime(ip.last_seen)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Click bị gắn cờ gần nhất</CardTitle>
          <span className="text-xs text-muted-foreground">{r.flagged.length} lượt</span>
        </CardHeader>
        <CardContent className="overflow-x-auto pt-2">
          <table className="w-full text-sm">
            <thead className="text-xs text-muted-foreground">
              <tr>
                {["Thời gian", "Link", "Tài khoản", "CTV", "IP", "Lý do"].map((h) => (
                  <th key={h} className="px-2 py-1.5 text-left font-normal">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {r.flagged.map((c, i) => (
                <tr key={i} className="border-t align-top">
                  <td className="whitespace-nowrap px-2 py-1.5 tabular-nums">{fmtDateTime(c.ts, true)}</td>
                  <td className="px-2 py-1.5">
                    <Link href={`/links/${encodeURIComponent(c.code)}`} className="text-primary hover:underline">
                      {c.code}
                    </Link>
                  </td>
                  <td className="px-2 py-1.5">{c.owner}</td>
                  <td className="px-2 py-1.5">{c.ctv_display}</td>
                  <td className="px-2 py-1.5 font-mono text-xs">{c.ip}</td>
                  <td className="space-y-1 px-2 py-1.5">
                    {c.reasons.map((x) => (
                      <Badge key={x} tone={c.is_suspicious ? "danger" : "warning"} className="mr-1">
                        {x}
                      </Badge>
                    ))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </CardContent>
      </Card>
    </div>
  );
}
