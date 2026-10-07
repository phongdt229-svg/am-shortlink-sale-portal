"use client";
import { useQuery } from "@tanstack/react-query";
import { Download, Loader2 } from "lucide-react";
import { EmptyState, ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Badge, Card, CardContent } from "@/components/ui/primitives";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import { fmtDateTime, fmtNumber } from "@/lib/format";

const KIND: Record<string, string> = {
  accounts: "Tài khoản",
  campaigns: "Chiến dịch",
  ctvs: "CTV",
  links_top: "Link",
  clicks: "Click thô",
  explorer: "Phân tích click",
};
const STATUS: Record<string, { label: string; tone: "default" | "primary" | "success" | "danger" | "warning" }> = {
  queued: { label: "Chờ xử lý", tone: "default" },
  running: { label: "Đang chạy", tone: "primary" },
  done: { label: "Hoàn tất", tone: "success" },
  failed: { label: "Lỗi", tone: "danger" },
  expired: { label: "Hết hạn", tone: "warning" },
};

function fmtSize(b?: number) {
  if (!b) return "–";
  return b > 1 << 20 ? `${(b / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(b / 1024))} KB`;
}

export function ExportList() {
  const q = useQuery({
    queryKey: ["exports"],
    queryFn: async () => unwrap(await browserApi.GET("/v1/exports")),
    // Tự làm mới khi còn job đang chạy.
    refetchInterval: (query) => (query.state.data?.items.some((j) => j.status === "queued" || j.status === "running") ? 3000 : false),
    staleTime: 0,
  });
  if (q.isError) return <ErrorState error={q.error} />;
  if (q.isPending)
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" /> Đang tải…
      </p>
    );
  if (!q.data.items.length)
    return (
      <EmptyState
        title="Chưa có job xuất dữ liệu"
        hint="Bấm “Xuất dữ liệu” trên màn hình Tài khoản / Chiến dịch / CTV / Link / Nhật ký click / Phân tích click"
      />
    );
  return (
    <Card>
      <CardContent className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="text-xs text-muted-foreground">
            <tr>
              {["Tên", "Loại", "Định dạng", "Trạng thái", "Số dòng", "Dung lượng", "Người tạo", "Tạo lúc", "Hết hạn", ""].map((h) => (
                <th key={h} className="px-2 py-1.5 text-left font-normal">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {q.data.items.map((j) => {
              const st = STATUS[j.status] ?? STATUS.queued!;
              return (
                <tr key={j.id} className="border-t">
                  <td className="px-2 py-2 font-medium">{j.name}</td>
                  <td className="px-2 py-2">{KIND[j.kind] ?? j.kind}</td>
                  <td className="px-2 py-2 uppercase">{j.format}</td>
                  <td className="px-2 py-2">
                    <Badge tone={st.tone}>
                      {j.status === "running" && <Loader2 className="mr-1 size-3 animate-spin" />}
                      {st.label}
                    </Badge>
                    {j.error && <p className="mt-1 max-w-64 text-xs text-destructive">{j.error}</p>}
                  </td>
                  <td className="px-2 py-2 tabular-nums">{j.rows ? fmtNumber(j.rows) : "–"}</td>
                  <td className="px-2 py-2 tabular-nums">{fmtSize(j.size_bytes)}</td>
                  <td className="px-2 py-2">{j.created_by}</td>
                  <td className="px-2 py-2 tabular-nums">{fmtDateTime(j.created_at)}</td>
                  <td className="px-2 py-2 tabular-nums text-muted-foreground">{fmtDateTime(j.expires_at)}</td>
                  <td className="px-2 py-2 text-right">
                    {j.status === "done" && (
                      <Button asChild size="sm" variant="outline">
                        <a href={`/api/bff/v1/exports/${j.id}/download`} download>
                          <Download /> Tải
                        </a>
                      </Button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}
