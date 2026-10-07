"use client";
import { flexRender, getCoreRowModel, useReactTable, type ColumnDef } from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, ChevronsUpDown, Search } from "lucide-react";
import { parseAsInteger, parseAsString, parseAsStringLiteral, useQueryStates } from "nuqs";
import { useEffect, useState, useTransition } from "react";
import { EmptyState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/primitives";
import { fmtNumber } from "@/lib/format";
import { cn } from "@/lib/utils";

export const tableParsers = {
  page: parseAsInteger.withDefault(1),
  sort: parseAsString,
  order: parseAsStringLiteral(["asc", "desc"] as const).withDefault("desc"),
  q: parseAsString.withDefault(""),
};

export interface ColumnMeta {
  align?: "right";
  /** Trường sort phía server; không có = không sort được. */
  sortKey?: string;
  className?: string;
}

interface Props<T> {
  columns: ColumnDef<T, unknown>[];
  data: T[];
  total: number;
  pageSize: number;
  searchPlaceholder?: string;
  footer?: React.ReactNode;
  defaultSort?: string;
  rowKey: (row: T) => string;
}

/** Bảng phân trang / sắp xếp / tìm ở server — trạng thái trên URL (?page, sort, order, q). */
export function DataTable<T>({ columns, data, total, pageSize, searchPlaceholder, footer, defaultSort = "clicks", rowKey }: Props<T>) {
  const [pending, startTransition] = useTransition();
  const [st, setSt] = useQueryStates(tableParsers, { shallow: false, startTransition });
  const [q, setQ] = useState(st.q);
  const sort = st.sort ?? defaultSort;

  // Debounce ô tìm kiếm.
  useEffect(() => {
    if (q === st.q) return;
    const t = setTimeout(() => void setSt({ q: q || null, page: null }), 350);
    return () => clearTimeout(t);
  }, [q, st.q, setSt]);

  const table = useReactTable({ data, columns, getCoreRowModel: getCoreRowModel(), manualSorting: true, manualPagination: true, getRowId: rowKey });
  const pages = Math.max(1, Math.ceil(total / pageSize));
  const page = Math.min(st.page, pages);

  return (
    <div className={cn("space-y-2", pending && "opacity-70 transition-opacity")}>
      {searchPlaceholder && (
        <div className="relative max-w-xs">
          <Search className="pointer-events-none absolute left-2 top-2.5 size-4 text-muted-foreground" />
          <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder={searchPlaceholder} className="h-9 pl-8" aria-label={searchPlaceholder} />
        </div>
      )}
      <div className="overflow-x-auto rounded-md border">
        <table className="w-full text-sm">
          <thead className="bg-muted/60">
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id}>
                {hg.headers.map((h) => {
                  const meta = (h.column.columnDef.meta ?? {}) as ColumnMeta;
                  const active = meta.sortKey && meta.sortKey === sort;
                  const Icon = active ? (st.order === "asc" ? ArrowUp : ArrowDown) : ChevronsUpDown;
                  return (
                    <th
                      key={h.id}
                      className={cn("whitespace-nowrap px-3 py-2 text-left text-xs font-medium text-muted-foreground", meta.align === "right" && "text-right", meta.className)}
                      aria-sort={active ? (st.order === "asc" ? "ascending" : "descending") : undefined}
                    >
                      {meta.sortKey ? (
                        <button
                          type="button"
                          className={cn("inline-flex items-center gap-1 hover:text-foreground", active && "text-foreground")}
                          onClick={() =>
                            void setSt({ sort: meta.sortKey!, order: active && st.order === "desc" ? "asc" : "desc", page: null })
                          }
                        >
                          {flexRender(h.column.columnDef.header, h.getContext())}
                          <Icon className={cn("size-3", !active && "opacity-40")} />
                        </button>
                      ) : (
                        flexRender(h.column.columnDef.header, h.getContext())
                      )}
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((r) => (
              <tr key={r.id} className="border-t hover:bg-muted/40">
                {r.getVisibleCells().map((c) => {
                  const meta = (c.column.columnDef.meta ?? {}) as ColumnMeta;
                  return (
                    <td key={c.id} className={cn("whitespace-nowrap px-3 py-2", meta.align === "right" && "text-right tabular-nums", meta.className)}>
                      {flexRender(c.column.columnDef.cell, c.getContext())}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
          {footer && data.length > 0 && <tfoot className="border-t-2 bg-muted/40 font-medium">{footer}</tfoot>}
        </table>
        {data.length === 0 && <EmptyState />}
      </div>
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>
          {fmtNumber(total)} dòng · trang {page}/{pages}
        </span>
        <div className="flex gap-1">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => void setSt({ page: page - 1 === 1 ? null : page - 1 })} aria-label="Trang trước">
            <ChevronLeft />
          </Button>
          <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => void setSt({ page: page + 1 })} aria-label="Trang sau">
            <ChevronRight />
          </Button>
        </div>
      </div>
    </div>
  );
}
