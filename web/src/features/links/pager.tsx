"use client";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { parseAsInteger, useQueryState } from "nuqs";
import { useTransition } from "react";
import { Button } from "@/components/ui/button";
import { fmtNumber } from "@/lib/format";

export function Pager({ total, page, pageSize }: { total: number; page: number; pageSize: number }) {
  const [pending, startTransition] = useTransition();
  const [, setPage] = useQueryState("page", parseAsInteger.withOptions({ shallow: false, startTransition }));
  const pages = Math.max(1, Math.ceil(total / pageSize));
  return (
    <div className="flex items-center justify-between text-xs text-muted-foreground">
      <span>
        {fmtNumber(total)} dòng · trang {Math.min(page, pages)}/{pages}
      </span>
      <div className="flex gap-1">
        <Button variant="outline" size="sm" disabled={page <= 1 || pending} onClick={() => void setPage(page - 1 <= 1 ? null : page - 1)} aria-label="Trang trước">
          <ChevronLeft />
        </Button>
        <Button variant="outline" size="sm" disabled={page >= pages || pending} onClick={() => void setPage(page + 1)} aria-label="Trang sau">
          <ChevronRight />
        </Button>
      </div>
    </div>
  );
}
