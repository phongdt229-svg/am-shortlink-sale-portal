import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { AccountsTable } from "@/features/accounts/accounts-table";
import { ExportButton } from "@/features/exports/export-button";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";

export const metadata: Metadata = { title: "Tài khoản" };

export default async function AccountsPage({ searchParams }: { searchParams: SearchParams }) {
  const { api: query, table, filters } = await pageContext(searchParams);
  const api = await serverApi();
  const page = data(
    await api.GET("/v1/reports/accounts", {
      params: { query: { ...query, granularity: undefined, page: table.page, page_size: 50, sort: table.sort, order: table.order, q: table.q } },
    }),
  );
  return (
    <div className="space-y-4">
      <PageHeader title="Báo cáo theo tài khoản" subtitle="Bấm vào tài khoản để xem chi tiết" actions={<ExportButton kind="accounts" />} />
      <AccountsTable page={page} compare={filters.compare} />
    </div>
  );
}
