import type { Metadata } from "next";
import Link from "next/link";
import { PageHeader } from "@/components/report/page-header";
import { SummaryView } from "@/components/report/summary-view";
import { Badge, Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { ClickTab } from "@/features/explorer/click-tab";
import { getSession } from "@/lib/session/server";
import { ClickTable } from "@/features/clicks/click-table";
import { DetailTabs } from "@/features/explorer/detail-tabs";
import { CopyButton } from "@/features/links/copy-button";
import { STATUS_LABEL } from "@/features/links/status";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";
import { fmtDateTime } from "@/lib/format";

type Props = { params: Promise<{ code: string }>; searchParams: SearchParams };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  return { title: `Link ${decodeURIComponent((await params).code)}` };
}

export default async function LinkDetailPage({ params, searchParams }: Props) {
  const code = decodeURIComponent((await params).code);
  const { api: query, search } = await pageContext(searchParams);
  const [api, session] = await Promise.all([serverApi(), getSession()]);
  const r = data(
    await api.GET("/v1/reports/links/{code}", {
      params: { path: { code }, query: { from: query.from, to: query.to, compare: query.compare, granularity: query.granularity } },
    }),
  );
  const l = r.link;
  const st = STATUS_LABEL[l.status] ?? STATUS_LABEL.active!;
  return (
    <div className="space-y-4">
      <PageHeader
        title={`Link /${l.prefix}/${l.code}`}
        crumbs={[{ href: `/links/top${search}`, label: "Link" }]}
        actions={<Badge tone={st.tone}>{st.label}</Badge>}
      />
      <Card>
        <CardContent className="grid gap-4 md:grid-cols-[1fr_auto]">
          <dl className="grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-[9rem_1fr]">
            <dt className="text-muted-foreground">Short URL</dt>
            <dd className="flex items-center gap-2 break-all font-medium">
              {l.short_url} <CopyButton value={l.short_url} />
            </dd>
            <dt className="text-muted-foreground">Long URL</dt>
            <dd className="break-all">{l.long_url}</dd>
            <dt className="text-muted-foreground">Tài khoản</dt>
            <dd>
              <Link href={`/accounts/${encodeURIComponent(l.owner)}${search}`} className="text-primary hover:underline">
                {l.owner}
              </Link>
            </dd>
            <dt className="text-muted-foreground">Chiến dịch</dt>
            <dd>
              {l.campaign_code ? (
                <Link href={`/campaigns/${encodeURIComponent(l.campaign_code)}${search}`} className="text-primary hover:underline">
                  {l.campaign_code}
                </Link>
              ) : (
                "—"
              )}
            </dd>
            <dt className="text-muted-foreground">CTV</dt>
            <dd>
              {l.ctv_ref ? (
                <Link href={`/ctvs/${encodeURIComponent(l.ctv_ref)}${search}`} className="text-primary hover:underline">
                  {l.ctv_display}
                </Link>
              ) : (
                <span className="italic text-muted-foreground">{l.ctv_display}</span>
              )}
            </dd>
            <dt className="text-muted-foreground">Tạo lúc</dt>
            <dd>
              {fmtDateTime(l.created_at)} · API {l.api_version ?? "—"}
              {l.is_custom ? " · mã tuỳ chỉnh" : ""}
            </dd>
          </dl>
          {/* eslint-disable-next-line @next/next/no-img-element -- ảnh QR động qua BFF (cookie phiên) */}
          <img
            src={`/api/bff/v1/links/${encodeURIComponent(l.code)}/qrcode?size=256`}
            alt={`Mã QR ${l.short_url}`}
            width={160}
            height={160}
            className="rounded-md border bg-white p-1"
          />
        </CardContent>
      </Card>
      <DetailTabs clicks={<ClickTab role={session!.user.role} lock={{ links: [l.code], account: [l.owner] }} />}>
        <SummaryView summary={r.summary}>
          <Card>
            <CardHeader>
              <CardTitle>100 lượt click gần nhất</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              <ClickTable rows={r.recent_clicks} showLink={false} />
            </CardContent>
          </Card>
        </SummaryView>
      </DetailTabs>
    </div>
  );
}
