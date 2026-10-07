"use client";
import type { ColumnDef } from "@tanstack/react-table";
import Link from "next/link";
import { DataTable } from "@/components/data-table/data-table";
import { DeltaCell } from "@/components/report/cells";
import { EmptyState } from "@/components/states";
import type { components } from "@/lib/api/schema";
import { useFilterSearch } from "@/lib/filters/use-filter-search";
import { fmtDate, fmtNumber } from "@/lib/format";

type Page = components["schemas"]["CampaignRowPage"];
type Row = components["schemas"]["CampaignRow"];

function useColumns(compare: boolean): ColumnDef<Row, unknown>[] {
  const search = useFilterSearch(["campaign"]);
  return [
    {
      id: "code",
      header: "Chiến dịch",
      cell: ({ row: { original: r } }) => (
        <Link href={`/campaigns/${encodeURIComponent(r.code || "-")}${search}`} className="hover:underline">
          <span className="font-medium text-primary">{r.code || "—"}</span>
          {r.name !== r.code && <span className="ml-2 text-xs text-muted-foreground">{r.name}</span>}
        </Link>
      ),
      meta: { sortKey: "code" },
    },
    { id: "new_links", header: "Link mới", cell: ({ row }) => fmtNumber(row.original.metrics.new_links), meta: { align: "right", sortKey: "new_links" } },
    {
      id: "active_links",
      header: "Link có click",
      cell: ({ row }) => fmtNumber(row.original.metrics.active_links),
      meta: { align: "right", sortKey: "active_links" },
    },
    { id: "clicks", header: "Lượt click", cell: ({ row }) => fmtNumber(row.original.metrics.clicks), meta: { align: "right", sortKey: "clicks" } },
    {
      id: "unique_clicks",
      header: "Khách",
      cell: ({ row }) => fmtNumber(row.original.metrics.unique_clicks),
      meta: { align: "right", sortKey: "unique_clicks" },
    },
    {
      id: "suspicious",
      header: "Nghi vấn",
      cell: ({ row }) => fmtNumber(row.original.metrics.suspicious_clicks),
      meta: { align: "right", sortKey: "suspicious_clicks" },
    },
    { id: "ctv_count", header: "Số CTV", cell: ({ row }) => fmtNumber(row.original.ctv_count), meta: { align: "right", sortKey: "ctv_count" } },
    { id: "first_date", header: "Bắt đầu", cell: ({ row }) => fmtDate(row.original.first_date), meta: { align: "right" } },
    { id: "last_click", header: "Click cuối", cell: ({ row }) => fmtDate(row.original.last_click_date), meta: { align: "right" } },
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
}

export function CampaignsTable({ page, compare }: { page: Page; compare: boolean }) {
  const columns = useColumns(compare);
  return (
    <DataTable
      columns={columns}
      data={page.items}
      total={page.total}
      pageSize={50}
      rowKey={(r) => r.code || "-"}
      searchPlaceholder="Tìm mã / tên chiến dịch…"
    />
  );
}

/** Bảng chiến dịch gọn (không phân trang) trong chi tiết tài khoản. */
export function CampaignList({ rows }: { rows: Row[] }) {
  const search = useFilterSearch(["campaign"]);
  if (!rows.length) return <EmptyState />;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-xs text-muted-foreground">
          <tr>
            <th className="py-1 text-left font-normal">Chiến dịch</th>
            <th className="py-1 text-right font-normal">Link mới</th>
            <th className="py-1 text-right font-normal">Link có click</th>
            <th className="py-1 text-right font-normal">Lượt click</th>
            <th className="py-1 text-right font-normal">Khách</th>
            <th className="py-1 text-right font-normal">Số CTV</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.code || "-"} className="border-t">
              <td className="py-1.5">
                <Link href={`/campaigns/${encodeURIComponent(r.code || "-")}${search}`} className="text-primary hover:underline">
                  {r.code || r.name}
                </Link>
              </td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.new_links)}</td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.active_links)}</td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.clicks)}</td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.unique_clicks)}</td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.ctv_count)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
