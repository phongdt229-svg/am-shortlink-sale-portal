"use client";
import type { ColumnDef } from "@tanstack/react-table";
import Link from "next/link";
import { DataTable } from "@/components/data-table/data-table";
import type { components } from "@/lib/api/schema";
import { useFilterSearch } from "@/lib/filters/use-filter-search";
import { fmtDecimal, fmtNumber } from "@/lib/format";
import { cn } from "@/lib/utils";

type Page = components["schemas"]["CTVRankPage"];
type Row = components["schemas"]["CTVRankRow"];

export function CTVsTable({ page }: { page: Page }) {
  const search = useFilterSearch(["ctv"]);
  const n = (k: keyof Row["metrics"], header: string): ColumnDef<Row, unknown> => ({
    id: k,
    header,
    cell: ({ row }) => (k === "ctr_per_link" ? fmtDecimal(row.original.metrics[k]) : fmtNumber(row.original.metrics[k] as number)),
    meta: { align: "right", sortKey: k },
  });
  const columns: ColumnDef<Row, unknown>[] = [
    { id: "rank", header: "Hạng", cell: ({ row }) => <span className="text-muted-foreground">{row.original.rank}</span>, meta: { sortKey: "rank" } },
    {
      id: "ctv",
      header: "CTV",
      cell: ({ row: { original: r } }) => (
        <Link href={`/ctvs/${encodeURIComponent(r.ctv_ref)}${search}`} className="text-primary hover:underline">
          <span className="font-mono">{r.ctv_display}</span>
          {r.name && <span className="ml-2 font-sans text-xs text-muted-foreground">{r.name}</span>}
        </Link>
      ),
    },
    n("new_links", "Link mới"),
    n("active_links", "Link có click"),
    n("clicks", "Lượt click"),
    n("unique_clicks", "Khách"),
    {
      id: "suspicious_clicks",
      header: "Nghi vấn",
      cell: ({ row }) => (
        <span className={cn(row.original.metrics.suspicious_clicks > 0 && "text-destructive")}>{fmtNumber(row.original.metrics.suspicious_clicks)}</span>
      ),
      meta: { align: "right", sortKey: "suspicious_clicks" },
    },
    n("ctr_per_link", "Click/link"),
  ];
  return (
    <DataTable
      columns={columns}
      data={page.items}
      total={page.total}
      pageSize={50}
      defaultSort="rank"
      rowKey={(r) => r.ctv_ref}
      searchPlaceholder="Tìm SĐT / hash / tên CTV…"
    />
  );
}
