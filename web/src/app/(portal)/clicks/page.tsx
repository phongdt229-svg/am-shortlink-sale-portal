import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { ClickLog } from "@/features/clicks/click-log";
import { ActiveClickFilters } from "@/features/clicks/active-filters";

export const metadata: Metadata = { title: "Nhật ký click" };

export default function ClicksPage() {
  return (
    <div className="space-y-4">
      <PageHeader title="Nhật ký click" subtitle="Click thô theo bộ lọc, tối đa 3 tháng · IP hiển thị đầy đủ chỉ với admin" />
      <ActiveClickFilters />
      <ClickLog />
    </div>
  );
}
