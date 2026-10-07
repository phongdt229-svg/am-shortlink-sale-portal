import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { ExportList } from "@/features/exports/export-list";

export const metadata: Metadata = { title: "Xuất dữ liệu" };

export default function ExportsPage() {
  return (
    <div className="space-y-4">
      <PageHeader title="Xuất dữ liệu" subtitle="Tạo job bằng nút “Xuất dữ liệu” trên các màn hình báo cáo · file giữ 7 ngày" />
      <ExportList />
    </div>
  );
}
