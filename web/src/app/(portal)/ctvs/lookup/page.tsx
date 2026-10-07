import type { Metadata } from "next";
import { PageHeader } from "@/components/report/page-header";
import { CTVLookup } from "@/features/ctvs/ctv-lookup";

export const metadata: Metadata = { title: "Tra cứu CTV" };

export default function CTVLookupPage() {
  return (
    <div className="space-y-4">
      <PageHeader title="Tra cứu CTV" subtitle="Nhập SĐT (mọi định dạng: 09…, +84…, có khoảng trắng) hoặc mã hash — khớp chính xác" />
      <CTVLookup />
    </div>
  );
}
