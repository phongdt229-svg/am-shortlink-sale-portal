import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { PageHeader } from "@/components/report/page-header";
import { RegistryTable } from "@/features/params/registry-table";
import { getSession } from "@/lib/session/server";

export const metadata: Metadata = { title: "Quản lý tham số" };

export default async function ParamRegistryPage() {
  const s = await getSession();
  if (s?.user.role !== "admin") notFound();
  return (
    <div className="space-y-4">
      <PageHeader
        title="Quản lý tham số theo dõi"
        subtitle="Tham số đã xuất hiện trên link. Bật theo dõi → Service chạy backfill từ click thô còn giữ; tham số PII (hash / drop) không được theo dõi theo giá trị."
      />
      <RegistryTable />
    </div>
  );
}
