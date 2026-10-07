import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { SummaryView } from "@/components/report/summary-view";
import { TopList } from "@/components/report/top-list";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { ClickTab } from "@/features/explorer/click-tab";
import { getSession } from "@/lib/session/server";
import { DetailTabs } from "@/features/explorer/detail-tabs";
import { CTVRanking } from "@/features/ctvs/ctv-ranking";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";

type Props = { params: Promise<{ code: string }>; searchParams: SearchParams };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  return { title: `Chiến dịch ${decodeURIComponent((await params).code)}` };
}

export default async function CampaignDetailPage({ params, searchParams }: Props) {
  const code = decodeURIComponent((await params).code);
  const { api: query, search } = await pageContext(searchParams);
  const [api, session] = await Promise.all([serverApi(), getSession()]);
  const r = data(
    await api.GET("/v1/reports/campaigns/{code}", {
      params: { path: { code }, query: { ...query, campaign: undefined } },
    }),
  );
  return (
    <div className="space-y-4">
      <PageHeader
        title={r.code ? `Chiến dịch ${r.code}` : r.name}
        subtitle={r.code && r.name !== r.code ? r.name : undefined}
        crumbs={[{ href: `/campaigns${search}`, label: "Chiến dịch" }]}
      />
      <DetailTabs clicks={<ClickTab role={session!.user.role} lock={{ campaign: [r.code || "-"] }} />}>
        <SummaryView summary={r.summary}>
          <div className="grid gap-4 xl:grid-cols-3">
            <Card className="xl:col-span-2">
              <CardHeader>
                <CardTitle>Xếp hạng CTV trong chiến dịch</CardTitle>
                <span className="text-xs text-muted-foreground">Top {r.ctv_ranking.length} theo lượt click</span>
              </CardHeader>
              <CardContent className="max-h-[32rem] overflow-y-auto pt-2">
                <CTVRanking rows={r.ctv_ranking} />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Top link</CardTitle>
              </CardHeader>
              <CardContent className="pt-2">
                <TopList items={r.top_links} href={(i) => `/links/${encodeURIComponent(i.key)}`} search={search} showOwner />
              </CardContent>
            </Card>
          </div>
        </SummaryView>
      </DetailTabs>
    </div>
  );
}
