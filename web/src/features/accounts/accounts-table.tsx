"use client";
import type { ColumnDef } from "@tanstack/react-table";
import Link from "next/link";
import { DataTable } from "@/components/data-table/data-table";
import { DeltaCell } from "@/components/report/cells";
import type { components } from "@/lib/api/schema";
import { useFilterSearch } from "@/lib/filters/use-filter-search";
import { fmtDecimal, fmtNumber } from "@/lib/format";

type Page = components["schemas"]["AccountRowPage"];
type Row = components["schemas"]["AccountRow"];
type Metrics = components["schemas"]["Metrics"];

const num = (k: keyof Metrics, header: string, sortKey = k): ColumnDef<Row, unknown> => ({
  id: k,
  header,
  cell: ({ row }) => (k === "ctr_per_link" ? fmtDecimal(row.original.metrics[k]) : fmtNumber(row.original.metrics[k] as number)),
  meta: { align: "right", sortKey },
});

export function AccountsTable({ page, compare }: { page: Page; compare: boolean }) {
  const search = useFilterSearch(["account"]);
  const columns: ColumnDef<Row, unknown>[] = [
    {
      id: "username",
      header: "Tài khoản",
      cell: ({ row }) => (
        <Link href={`/accounts/${encodeURIComponent(row.original.username)}${search}`} className="font-medium text-primary hover:underline">
          {row.original.username}
        </Link>
      ),
      meta: { sortKey: "username" },
    },
    num("total_links", "Tổng link"),
    num("new_links", "Link mới"),
    num("active_links", "Link có click"),
    num("clicks", "Lượt click"),
    num("unique_clicks", "Khách"),
    num("bot_clicks", "Bot"),
    num("suspicious_clicks", "Nghi vấn"),
    num("ctr_per_link", "Click/link"),
    ...(compare
      ? [{ id: "change", header: "So kỳ trước", cell: ({ row }) => <DeltaCell value={row.original.change_clicks} />, meta: { align: "right", sortKey: "change_clicks" } } satisfies ColumnDef<Row, unknown>]
      : []),
  ];
  const t = page.totals;
  return (
    <DataTable
      columns={columns}
      data={page.items}
      total={page.total}
      pageSize={50}
      rowKey={(r) => r.username}
      searchPlaceholder="Tìm tài khoản…"
      footer={
        <tr>
          <td className="px-3 py-2">Tổng</td>
          {[t.total_links ?? 0, t.new_links, t.active_links, t.clicks, t.unique_clicks, t.bot_clicks, t.suspicious_clicks].map((v, i) => (
            <td key={i} className="px-3 py-2 text-right tabular-nums">
              {fmtNumber(v)}
            </td>
          ))}
          <td className="px-3 py-2 text-right tabular-nums">{fmtDecimal(t.ctr_per_link)}</td>
          {compare && <td />}
        </tr>
      }
    />
  );
}
