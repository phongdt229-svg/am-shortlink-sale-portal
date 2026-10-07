import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { SummaryView } from "@/components/report/summary-view";
import { TopList } from "@/components/report/top-list";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { ClickLog } from "@/features/clicks/click-log";
import { DetailTabs } from "@/features/explorer/detail-tabs";
import { CampaignList } from "@/features/campaigns/campaigns-table";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";

type Props = { params: Promise<{ username: string }>; searchParams: SearchParams };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  return { title: `Tài khoản ${decodeURIComponent((await params).username)}` };
}

export default async function AccountDetailPage({ params, searchParams }: Props) {
  const username = decodeURIComponent((await params).username);
  const { api: query, search } = await pageContext(searchParams);
  const api = await serverApi();
  const r = data(
    await api.GET("/v1/reports/accounts/{username}", {
      params: { path: { username }, query: { ...query, account: undefined } },
    }),
  );
  return (
    <div className="space-y-4">
      <PageHeader title={`Tài khoản ${r.username}`} crumbs={[{ href: `/accounts${search}`, label: "Tài khoản" }]} />
      <DetailTabs clicks={<ClickLog locked={{ base: { account: [r.username] } }} />}>
      <SummaryView summary={r.summary}>
        <div className="grid gap-4 xl:grid-cols-3">
          <Card className="xl:col-span-3">
            <CardHeader>
              <CardTitle>Chiến dịch của tài khoản</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              <CampaignList rows={r.campaigns} />
            </CardContent>
          </Card>
          <Card className="xl:col-span-3 2xl:col-span-1">
            <CardHeader>
              <CardTitle>Top CTV</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              <TopList items={r.top_ctvs} href={(i) => `/ctvs/${encodeURIComponent(i.key)}`} search={search} />
            </CardContent>
          </Card>
          <Card className="xl:col-span-3 2xl:col-span-2">
            <CardHeader>
              <CardTitle>Top link</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              <TopList items={r.top_links} href={(i) => `/links/${encodeURIComponent(i.key)}`} search={search} />
            </CardContent>
          </Card>
        </div>
      </SummaryView>
      </DetailTabs>
    </div>
  );
}
