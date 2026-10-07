"use client";
import Link from "next/link";
import { DeltaCell } from "@/components/report/cells";
import { EmptyState } from "@/components/states";
import { Badge } from "@/components/ui/primitives";
import type { components } from "@/lib/api/schema";
import { useFilterSearch } from "@/lib/filters/use-filter-search";
import { fmtDate, fmtNumber } from "@/lib/format";
import { STATUS_LABEL } from "./status";

type Row = components["schemas"]["LinkRow"];


/** Bảng link (top / tăng trưởng / link chết / chưa định danh / link của CTV). */
export function LinkRows({ rows, mode = "clicks", showOwner = true }: { rows: Row[]; mode?: "clicks" | "growth" | "dead"; showOwner?: boolean }) {
  const search = useFilterSearch();
  if (!rows.length) return <EmptyState />;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-xs text-muted-foreground">
          <tr>
            <th className="px-2 py-1.5 text-left font-normal">Link</th>
            {showOwner && <th className="px-2 py-1.5 text-left font-normal">Tài khoản</th>}
            <th className="px-2 py-1.5 text-left font-normal">Chiến dịch / CTV</th>
            <th className="px-2 py-1.5 text-left font-normal">Trạng thái</th>
            {mode !== "dead" && <th className="px-2 py-1.5 text-right font-normal">Click</th>}
            {mode === "clicks" && <th className="px-2 py-1.5 text-right font-normal">Khách</th>}
            {mode === "growth" && <th className="px-2 py-1.5 text-right font-normal">Kỳ trước</th>}
            {mode === "growth" && <th className="px-2 py-1.5 text-right font-normal">Tăng</th>}
            <th className="px-2 py-1.5 text-right font-normal">{mode === "dead" ? "Click cuối" : "Ngày tạo"}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => {
            const st = STATUS_LABEL[r.link.status] ?? STATUS_LABEL.active!;
            return (
              <tr key={r.link.code} className="border-t align-top">
                <td className="max-w-72 px-2 py-1.5">
                  <Link href={`/links/${encodeURIComponent(r.link.code)}${search}`} className="font-medium text-primary hover:underline">
                    /{r.link.prefix}/{r.link.code}
                  </Link>
                  <p className="truncate text-xs text-muted-foreground" title={r.link.long_url}>
                    {r.link.long_url}
                  </p>
                </td>
                {showOwner && <td className="px-2 py-1.5">{r.link.owner}</td>}
                <td className="px-2 py-1.5">
                  {r.link.campaign_code || "—"}
                  <p className="text-xs text-muted-foreground">{r.link.ctv_display}</p>
                </td>
                <td className="px-2 py-1.5">
                  <Badge tone={st.tone}>{st.label}</Badge>
                </td>
                {mode !== "dead" && <td className="px-2 py-1.5 text-right tabular-nums">{fmtNumber(r.clicks)}</td>}
                {mode === "clicks" && <td className="px-2 py-1.5 text-right tabular-nums text-muted-foreground">{fmtNumber(r.unique_clicks)}</td>}
                {mode === "growth" && <td className="px-2 py-1.5 text-right tabular-nums text-muted-foreground">{fmtNumber(r.previous_clicks)}</td>}
                {mode === "growth" && (
                  <td className="px-2 py-1.5 text-right">
                    <DeltaCell value={r.growth} />
                  </td>
                )}
                <td className="px-2 py-1.5 text-right tabular-nums text-muted-foreground">
                  {mode === "dead" ? (r.last_click_date ? fmtDate(r.last_click_date) : "Chưa có") : fmtDate(r.link.created_at)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
