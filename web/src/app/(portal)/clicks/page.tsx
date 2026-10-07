import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { ActiveClickFilters } from "@/features/clicks/active-filters";
import { ClickLog } from "@/features/clicks/click-log";
import { ExportButton } from "@/features/exports/export-button";
import { getSession } from "@/lib/session/server";

export const metadata: Metadata = { title: "Nhật ký click" };

export default async function ClicksPage() {
  const s = await getSession();
  return (
    <div className="space-y-4">
      <PageHeader
        title="Nhật ký click"
        subtitle="Click thô theo bộ lọc, tối đa 3 tháng · IP hiển thị đầy đủ chỉ với admin"
        // Vai trò xem báo cáo không được xuất click thô (§2).
        actions={s?.user.role !== "viewer" ? <ExportButton kind="clicks" /> : undefined}
      />
      <ActiveClickFilters />
      <ClickLog />
    </div>
  );
}
