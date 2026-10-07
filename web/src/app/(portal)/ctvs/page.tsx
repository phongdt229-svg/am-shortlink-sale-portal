import { Search, UserX } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { PageHeader } from "@/components/report/page-header";
import { Button } from "@/components/ui/button";
import { CTVsTable } from "@/features/ctvs/ctvs-table";
import { ExportButton } from "@/features/exports/export-button";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";
import { getSession } from "@/lib/session/server";

export const metadata: Metadata = { title: "CTV" };

export default async function CTVsPage({ searchParams }: { searchParams: SearchParams }) {
  const { api: query, table, search } = await pageContext(searchParams);
  const [api, session] = await Promise.all([serverApi(), getSession()]);
  const page = data(
    await api.GET("/v1/reports/ctvs", {
      params: { query: { ...query, granularity: undefined, page: table.page, page_size: 50, sort: table.sort, order: table.order, q: table.q } },
    }),
  );
  return (
    <div className="space-y-4">
      <PageHeader
        title="Bảng xếp hạng CTV"
        subtitle="Xếp theo lượt click trong kỳ — dùng cho đối soát / trả thưởng"
        actions={
          <>
            <ExportButton kind="ctvs" label="Xuất đối soát" />
            <Button variant="outline" size="sm" asChild>
              <Link href="/ctvs/lookup">
                <Search /> Tra cứu CTV
              </Link>
            </Button>
            {session?.user.role !== "viewer" && (
              <Button variant="outline" size="sm" asChild>
                <Link href={`/ctvs/unidentified${search}`}>
                  <UserX /> CTV chưa định danh
                </Link>
              </Button>
            )}
          </>
        }
      />
      <CTVsTable page={page} />
    </div>
  );
}
