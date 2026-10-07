import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { SummaryView } from "@/components/report/summary-view";
import { TopList } from "@/components/report/top-list";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";
import { getSession } from "@/lib/session/server";

export const metadata: Metadata = { title: "Tổng quan" };

export default async function OverviewPage({ searchParams }: { searchParams: SearchParams }) {
  const { api: query, search } = await pageContext(searchParams);
  const [api, session] = await Promise.all([serverApi(), getSession()]);
  const r = data(await api.GET("/v1/reports/overview", { params: { query } }));
  const isUser = session?.user.role === "user";

  return (
    <div className="space-y-4">
      <PageHeader title="Tổng quan" subtitle={isUser ? `Tài khoản ${session?.user.username}` : undefined} />
      <SummaryView summary={r.summary}>
        <div className={`grid gap-4 md:grid-cols-2 ${isUser ? "2xl:grid-cols-3" : "2xl:grid-cols-4"}`}>
          {!isUser && (
            <Card>
              <CardHeader>
                <CardTitle>Top tài khoản</CardTitle>
              </CardHeader>
              <CardContent className="pt-2">
                <TopList items={r.top_accounts} href={(i) => `/accounts/${encodeURIComponent(i.key)}`} search={search} />
              </CardContent>
            </Card>
          )}
          <Card>
            <CardHeader>
              <CardTitle>Top chiến dịch</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              <TopList items={r.top_campaigns} href={(i) => `/campaigns/${encodeURIComponent(i.key)}`} search={search} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Top CTV</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              <TopList items={r.top_ctvs} href={(i) => `/ctvs/${encodeURIComponent(i.key)}`} search={search} showOwner={!isUser} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Top link</CardTitle>
            </CardHeader>
            <CardContent className="pt-2">
              <TopList items={r.top_links} href={(i) => `/links/${encodeURIComponent(i.key)}`} search={search} showOwner={!isUser} />
            </CardContent>
          </Card>
        </div>
      </SummaryView>
    </div>
  );
}
