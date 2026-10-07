import type { Metadata } from "next";
import { createSearchParamsCache, parseAsInteger, parseAsStringLiteral } from "nuqs/server";
import { PageHeader } from "@/components/report/page-header";
import { Card, CardContent } from "@/components/ui/primitives";
import { LinkRows } from "@/features/links/link-rows";
import { ModeTabs } from "@/features/links/mode-tabs";
import { Pager } from "@/features/links/pager";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";

export const metadata: Metadata = { title: "Link" };

const MODES = ["clicks", "growth", "dead"] as const;
const cache = createSearchParamsCache({
  mode: parseAsStringLiteral(MODES).withDefault("clicks"),
  dead_days: parseAsInteger.withDefault(14),
  page: parseAsInteger.withDefault(1),
});

export default async function TopLinksPage({ searchParams }: { searchParams: SearchParams }) {
  const { api: query } = await pageContext(searchParams);
  const { mode, dead_days, page } = cache.parse(await searchParams);
  const api = await serverApi();
  const r = data(
    await api.GET("/v1/reports/links/top", {
      params: {
        query: { from: query.from, to: query.to, account: query.account, campaign: query.campaign, ctv: query.ctv, prefix: query.prefix, mode, dead_days, page, page_size: 50 },
      },
    }),
  );
  return (
    <div className="space-y-4">
      <PageHeader title="Báo cáo theo link" />
      <ModeTabs mode={mode} deadDays={dead_days} />
      <Card>
        <CardContent className="space-y-3">
          <LinkRows rows={r.items} mode={mode} />
          <Pager total={r.total} page={page} pageSize={50} />
        </CardContent>
      </Card>
    </div>
  );
}
