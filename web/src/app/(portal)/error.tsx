"use client";
import { RotateCcw } from "lucide-react";
import { ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/problem";

// Lỗi khi render Server Component: Next chỉ chuyển `digest`; ApiError được dựng lại để hiện mã hỗ trợ nếu có.
export default function PortalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const e = new ApiError({ status: 500, title: "Không tải được dữ liệu", request_id: error.digest });
  return (
    <div className="space-y-3">
      <ErrorState error={e} />
      <Button variant="outline" size="sm" onClick={reset}>
        <RotateCcw /> Thử lại
      </Button>
    </div>
  );
}
