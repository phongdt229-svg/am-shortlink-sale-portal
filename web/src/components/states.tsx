import { AlertTriangle, Inbox } from "lucide-react";
import { ApiError } from "@/lib/api/problem";
import { cn } from "@/lib/utils";

/** Trạng thái lỗi — luôn hiện request_id để người dùng báo hỗ trợ. */
export function ErrorState({ error, className }: { error: unknown; className?: string }) {
  const e = error instanceof ApiError ? error : undefined;
  let message = e?.message ?? "Đã có lỗi xảy ra";
  if (e?.status === 403) message = e.message || "Bạn không có quyền xem dữ liệu này";
  if (e?.status === 504 || e?.code === "query_timeout") message = "Truy vấn quá lâu — hãy thu hẹp khoảng ngày hoặc bộ lọc";
  return (
    <div role="alert" className={cn("flex items-start gap-3 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm", className)}>
      <AlertTriangle className="mt-0.5 size-4 shrink-0 text-destructive" />
      <div className="space-y-1">
        <p>{message}</p>
        {e?.fields?.length ? (
          <ul className="list-disc pl-4 text-muted-foreground">
            {e.fields.map((f) => (
              <li key={f.field + f.message}>
                {f.field}: {f.message}
              </li>
            ))}
          </ul>
        ) : null}
        {e?.requestId ? <p className="text-xs text-muted-foreground">Mã hỗ trợ: {e.requestId}</p> : null}
      </div>
    </div>
  );
}

export function EmptyState({ title = "Không có dữ liệu", hint, className }: { title?: string; hint?: string; className?: string }) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-1 py-10 text-center text-sm text-muted-foreground", className)}>
      <Inbox className="size-6" />
      <p className="font-medium">{title}</p>
      {hint ? <p className="text-xs">{hint}</p> : null}
    </div>
  );
}
