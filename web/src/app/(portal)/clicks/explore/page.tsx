import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { PageHeader } from "@/components/report/page-header";
import { ErrorState } from "@/components/states";
import { Explorer } from "@/features/explorer/explorer";
import { savedHref } from "@/features/explorer/saved-href";
import { serverApi } from "@/lib/api/server";
import { ApiError } from "@/lib/api/problem";
import type { SearchParams } from "@/lib/filters/page";
import { requireSession } from "@/lib/session/server";

export const metadata: Metadata = { title: "Phân tích lượt click" };

export default async function ExplorePage({ searchParams }: { searchParams: SearchParams }) {
  const s = await requireSession();
  const shared = (await searchParams).shared;
  if (typeof shared === "string" && shared) {
    // Link chia sẻ: nạp trạng thái đã lưu rồi chạy với phạm vi của người mở.
    const api = await serverApi();
    const r = await api.GET("/v1/saved-reports/shared/{token}", { params: { path: { token: shared } } });
    if (r.data) redirect(savedHref(r.data.query));
    return <ErrorState error={new ApiError({ status: r.response.status, title: "Link chia sẻ không còn hiệu lực" })} />;
  }
  return (
    <div className="space-y-4">
      <PageHeader title="Phân tích lượt click" subtitle="Lọc nhiều tiêu chí · nhóm 1–3 chiều · bấm vào nhóm để xem click thô" />
      <Explorer role={s.user.role} />
    </div>
  );
}
