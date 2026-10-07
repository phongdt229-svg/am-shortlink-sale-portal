"use client";
import type { ColumnDef } from "@tanstack/react-table";
import Link from "next/link";
import { DataTable } from "@/components/data-table/data-table";
import { DeltaCell } from "@/components/report/cells";
import type { components } from "@/lib/api/schema";
import { encodeClickFilters } from "@/lib/filters/click-filters";
import { useFilterSearch } from "@/lib/filters/use-filter-search";
import { fmtNumber, fmtPercent } from "@/lib/format";

type Report = components["schemas"]["ParamReport"];
type Row = components["schemas"]["ParamValueRow"];

export function ParamValuesTable({ report, paramKey, compare }: { report: Report; paramKey: string; compare: boolean }) {
  const search = useFilterSearch();
  const sep = search ? "&" : "?";
  const drill = (v: string) => `/clicks${search}${sep}cf=${encodeClickFilters({ params: [{ key: paramKey, op: "in", values: [v] }] })}`;
  const columns: ColumnDef<Row, unknown>[] = [
    {
      id: "value",
      header: "Giá trị",
      cell: ({ row }) => (
        <Link href={drill(row.original.value)} className="font-medium text-primary hover:underline" title="Xem click thô có giá trị này">
          {row.original.value || "(rỗng)"}
        </Link>
      ),
      meta: { sortKey: "value" },
    },
    { id: "clicks", header: "Lượt click", cell: ({ row }) => fmtNumber(row.original.metrics.clicks), meta: { align: "right", sortKey: "clicks" } },
    { id: "share", header: "% tổng", cell: ({ row }) => fmtPercent(row.original.share), meta: { align: "right" } },
    { id: "unique", header: "Khách", cell: ({ row }) => fmtNumber(row.original.metrics.unique_clicks), meta: { align: "right", sortKey: "unique_clicks" } },
    { id: "links", header: "Link có click", cell: ({ row }) => fmtNumber(row.original.links), meta: { align: "right", sortKey: "links" } },
    { id: "new", header: "Link mới", cell: ({ row }) => fmtNumber(row.original.metrics.new_links), meta: { align: "right", sortKey: "new_links" } },
    { id: "accounts", header: "Tài khoản", cell: ({ row }) => fmtNumber(row.original.accounts), meta: { align: "right", sortKey: "accounts" } },
    { id: "campaigns", header: "Chiến dịch", cell: ({ row }) => fmtNumber(row.original.campaigns), meta: { align: "right", sortKey: "campaigns" } },
    ...(compare
      ? [
          {
            id: "change",
            header: "So kỳ trước",
            cell: ({ row }) => <DeltaCell value={row.original.change_clicks} />,
            meta: { align: "right", sortKey: "change_clicks" },
          } satisfies ColumnDef<Row, unknown>,
        ]
      : []),
  ];
  return <DataTable columns={columns} data={report.rows} total={report.total} pageSize={50} rowKey={(r) => r.value} searchPlaceholder="Tìm giá trị…" />;
}
