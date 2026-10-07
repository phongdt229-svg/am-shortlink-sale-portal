"use client";
import { useMutation } from "@tanstack/react-query";
import { Check, Download, Loader2 } from "lucide-react";
import Link from "next/link";
import { useQueryStates } from "nuqs";
import { useSearchParams } from "next/navigation";
import { ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import type { components } from "@/lib/api/schema";
import { filterParsers, toApiQuery, type Filters } from "@/lib/filters";
import { compactFilters, decodeClickFilters } from "@/lib/filters/click-filters";
import { buildQuery, decodeState } from "@/features/explorer/state";

type Kind = components["schemas"]["ExportRequest"]["kind"];

/**
 * Nút "Xuất CSV / XLSX": tạo job nền R8 với ĐÚNG bộ lọc đang xem (toàn bộ dữ liệu, không chỉ trang hiện tại).
 * `extra` bổ sung tham số riêng của màn hình (vd mode / dead_days của top link).
 */
export function ExportButton({ kind, extra, label = "Xuất dữ liệu" }: { kind: Kind; extra?: Record<string, unknown>; label?: string }) {
  const [f] = useQueryStates(filterParsers);
  const sp = useSearchParams();
  const create = useMutation({
    mutationFn: async (format: "csv" | "xlsx") => {
      const api = toApiQuery(f as Filters);
      let params: Record<string, unknown> = { ...api, ...(extra ?? {}) };
      if (kind === "clicks") params.filters = compactFilters(decodeClickFilters(sp.get("cf")));
      if (kind === "explorer") params = { ...buildQuery(decodeState(sp.get("ex")), api) };
      return unwrap(await browserApi.POST("/v1/exports", { body: { kind, format, params } }));
    },
  });

  return (
    <Popover onOpenChange={(o) => !o && create.reset()}>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm">
          <Download /> {label}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 space-y-2 text-sm">
        {create.isSuccess ? (
          <div className="space-y-2">
            <p className="flex items-center gap-2">
              <Check className="size-4 text-success" /> Đã tạo job xuất dữ liệu.
            </p>
            <p className="text-xs text-muted-foreground">File chạy nền, giữ 7 ngày. Theo dõi và tải ở trang Xuất dữ liệu.</p>
            <Button asChild size="sm" className="w-full">
              <Link href="/exports">Mở trang Xuất dữ liệu</Link>
            </Button>
          </div>
        ) : (
          <>
            <p className="text-xs text-muted-foreground">Xuất toàn bộ dữ liệu theo bộ lọc hiện tại (tối đa 1 triệu dòng).</p>
            <div className="flex gap-2">
              {(["xlsx", "csv"] as const).map((fmt) => (
                <Button
                  key={fmt}
                  size="sm"
                  variant={fmt === "xlsx" ? "default" : "outline"}
                  className="flex-1"
                  disabled={create.isPending}
                  onClick={() => create.mutate(fmt)}
                >
                  {create.isPending && create.variables === fmt && <Loader2 className="animate-spin" />}
                  {fmt.toUpperCase()}
                </Button>
              ))}
            </div>
            {create.isError && <ErrorState error={create.error} />}
          </>
        )}
      </PopoverContent>
    </Popover>
  );
}
