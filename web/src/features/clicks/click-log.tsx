"use client";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { useQueryStates } from "nuqs";
import { useSearchParams } from "next/navigation";
import { ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/problem";
import type { components } from "@/lib/api/schema";
import { filterParsers, toApiQuery, type Filters } from "@/lib/filters";
import { clicksQuery, decodeClickFilters, type ClickFilters } from "@/lib/filters/click-filters";
import { ClickTable } from "./click-table";

type ClickPage = components["schemas"]["ClickPage"];

async function fetchClicks(qs: string): Promise<ClickPage> {
  const res = await fetch(`/api/bff/v1/reports/clicks?${qs}`, { headers: { accept: "application/json" } });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError({ status: res.status, ...body });
  return body as ClickPage;
}

/**
 * Nhật ký click thô, phân trang con trỏ ("Tải thêm"). Bộ lọc chung lấy từ URL; tiêu chí chi tiết từ `cf`
 * (hoặc khoá cứng bởi màn hình cha, vd chi tiết link).
 */
export function ClickLog({ locked, pageSize = 50 }: { locked?: { filters?: ClickFilters; base?: Record<string, string | string[]> }; pageSize?: number }) {
  const [f] = useQueryStates(filterParsers);
  const sp = useSearchParams();
  const extra = decodeClickFilters(sp.get("cf"));
  const filters: ClickFilters = { ...extra, ...(locked?.filters ?? {}) };
  const api = toApiQuery(f as Filters);
  const base = { from: api.from, to: api.to, account: api.account, campaign: api.campaign, ctv: api.ctv, prefix: api.prefix, ...(locked?.base ?? {}) };

  const q = useInfiniteQuery({
    queryKey: ["clicks", base, filters, pageSize],
    queryFn: ({ pageParam }) => fetchClicks(clicksQuery(base, filters, pageParam, pageSize)),
    initialPageParam: "",
    getNextPageParam: (last) => last.next_cursor || undefined,
    staleTime: 30_000,
  });

  if (q.isError) return <ErrorState error={q.error} />;
  const rows = q.data?.pages.flatMap((p) => p.items) ?? [];
  return (
    <div className="space-y-3">
      {q.isPending ? (
        <div className="flex items-center gap-2 p-6 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" /> Đang tải…
        </div>
      ) : (
        <ClickTable rows={rows} />
      )}
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>Đang hiển thị {rows.length.toLocaleString("vi-VN")} lượt click (mới nhất trước)</span>
        {q.hasNextPage && (
          <Button variant="outline" size="sm" onClick={() => void q.fetchNextPage()} disabled={q.isFetchingNextPage}>
            {q.isFetchingNextPage && <Loader2 className="animate-spin" />} Tải thêm
          </Button>
        )}
      </div>
    </div>
  );
}
