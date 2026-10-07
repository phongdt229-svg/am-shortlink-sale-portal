import type { Metadata } from "next";
import { createSearchParamsCache, parseAsInteger } from "nuqs/server";
import { PageHeader } from "@/components/report/page-header";
import { KpiCards } from "@/components/report/kpi-cards";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { LinkRows } from "@/features/links/link-rows";
import { Pager } from "@/features/links/pager";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";
import { fmtPercent } from "@/lib/format";

export const metadata: Metadata = { title: "CTV chưa định danh" };

const cache = createSearchParamsCache({ page: parseAsInteger.withDefault(1) });

export default async function UnidentifiedPage({ searchParams }: { searchParams: SearchParams }) {
  const { api: query, search } = await pageContext(searchParams);
  const { page } = cache.parse(await searchParams);
  const api = await serverApi();
  const r = data(
    await api.GET("/v1/reports/ctvs/unidentified", {
      params: { query: { from: query.from, to: query.to, account: query.account, campaign: query.campaign, prefix: query.prefix, page, page_size: 50 } },
    }),
  );
  return (
    <div className="space-y-4">
      <PageHeader
        title="CTV chưa định danh"
        subtitle={`Link không có utm_extra_ctv · chiếm ${fmtPercent(r.share_of_clicks)} lượt click trong kỳ`}
        crumbs={[{ href: `/ctvs${search}`, label: "CTV" }]}
      />
      <KpiCards kpis={{ period: { from: query.from, to: query.to }, current: r.metrics }} keys={["total_links", "new_links", "active_links", "clicks", "unique_clicks", "suspicious_clicks"]} />
      <Card>
        <CardHeader>
          <CardTitle>Link cần bổ sung CTV</CardTitle>
          <span className="text-xs text-muted-foreground">Mới tạo trước</span>
        </CardHeader>
        <CardContent className="space-y-3 pt-2">
          <LinkRows rows={r.links.items} />
          <Pager total={r.links.total} page={page} pageSize={50} />
        </CardContent>
      </Card>
    </div>
  );
}
