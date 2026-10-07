import { GitCompare } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { PageHeader } from "@/components/report/page-header";
import { Button } from "@/components/ui/button";
import { CampaignsTable } from "@/features/campaigns/campaigns-table";
import { ExportButton } from "@/features/exports/export-button";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";

export const metadata: Metadata = { title: "Chiến dịch" };

export default async function CampaignsPage({ searchParams }: { searchParams: SearchParams }) {
  const { api: query, table, filters, search } = await pageContext(searchParams);
  const api = await serverApi();
  const page = data(
    await api.GET("/v1/reports/campaigns", {
      params: {
        query: {
          ...query,
          granularity: undefined,
          page: table.page,
          page_size: 50,
          sort: table.sort,
          order: table.order,
          q: table.q,
        },
      },
    }),
  );
  return (
    <div className="space-y-4">
      <PageHeader
        title="Báo cáo theo chiến dịch"
        actions={
          <>
            <ExportButton kind="campaigns" />
            <Button variant="outline" size="sm" asChild>
              <Link href={`/campaigns/compare${search}`}>
                <GitCompare /> So sánh chiến dịch
              </Link>
            </Button>
          </>
        }
      />
      <CampaignsTable page={page} compare={filters.compare} />
    </div>
  );
}
