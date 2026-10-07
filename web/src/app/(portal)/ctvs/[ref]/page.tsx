import { AlertTriangle, Info, ShieldAlert } from "lucide-react";
import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { SummaryView } from "@/components/report/summary-view";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { CampaignList } from "@/features/campaigns/campaigns-table";
import { ClickTab } from "@/features/explorer/click-tab";
import { getSession } from "@/lib/session/server";
import { DetailTabs } from "@/features/explorer/detail-tabs";
import { LinkRows } from "@/features/links/link-rows";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";
import { cn } from "@/lib/utils";

export const metadata: Metadata = { title: "Chi tiết CTV" };

type Props = { params: Promise<{ ref: string }>; searchParams: SearchParams };

const ALERT = {
  info: { icon: Info, cls: "border-border bg-muted/40" },
  warning: { icon: AlertTriangle, cls: "border-warning/50 bg-warning/10" },
  critical: { icon: ShieldAlert, cls: "border-destructive/40 bg-destructive/5 text-destructive" },
} as const;

export default async function CTVDetailPage({ params, searchParams }: Props) {
  const ref = decodeURIComponent((await params).ref);
  const { api: query, search } = await pageContext(searchParams);
  const [api, session] = await Promise.all([serverApi(), getSession()]);
  const r = data(await api.GET("/v1/reports/ctvs/{ref}", { params: { path: { ref }, query: { ...query, ctv: undefined } } }));
  return (
    <div className="space-y-4">
      <PageHeader
        title={`CTV ${r.ctv_display}`}
        subtitle={[r.name, r.owners?.length ? `Tài khoản: ${r.owners.join(", ")}` : null].filter(Boolean).join(" · ")}
        crumbs={[{ href: `/ctvs${search}`, label: "CTV" }]}
      />
      {r.alerts.length > 0 && (
        <div className="space-y-2" role="status">
          {r.alerts.map((a) => {
            const A = ALERT[a.level];
            return (
              <div key={a.code} className={cn("flex items-center gap-2 rounded-md border px-3 py-2 text-sm", A.cls)}>
                <A.icon className="size-4 shrink-0" />
                {a.message}
              </div>
            );
          })}
        </div>
      )}
      <DetailTabs clicks={<ClickTab role={session!.user.role} lock={{ ctv: [ref] }} />}>
        <SummaryView summary={r.summary}>
          <div className="grid gap-4 xl:grid-cols-3">
            <Card>
              <CardHeader>
                <CardTitle>Chiến dịch tham gia</CardTitle>
              </CardHeader>
              <CardContent className="pt-2">
                <CampaignList rows={r.campaigns} />
              </CardContent>
            </Card>
            <Card className="xl:col-span-2">
              <CardHeader>
                <CardTitle>Link của CTV</CardTitle>
                <span className="text-xs text-muted-foreground">Top {r.links.length} theo lượt click</span>
              </CardHeader>
              <CardContent className="max-h-[32rem] overflow-y-auto pt-2">
                <LinkRows rows={r.links} />
              </CardContent>
            </Card>
          </div>
        </SummaryView>
      </DetailTabs>
    </div>
  );
}
