"use client";
import Link from "next/link";
import { EmptyState } from "@/components/states";
import type { components } from "@/lib/api/schema";
import { useFilterSearch } from "@/lib/filters/use-filter-search";
import { fmtNumber } from "@/lib/format";

type Row = components["schemas"]["CTVRankRow"];

/** Bảng xếp hạng CTV (SĐT đã được portal-api che theo vai trò). */
export function CTVRanking({ rows }: { rows: Row[] }) {
  const search = useFilterSearch(["ctv"]);
  if (!rows.length) return <EmptyState />;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-xs text-muted-foreground">
          <tr>
            <th className="py-1 text-left font-normal">Hạng</th>
            <th className="py-1 text-left font-normal">CTV</th>
            <th className="py-1 text-right font-normal">Link mới</th>
            <th className="py-1 text-right font-normal">Link có click</th>
            <th className="py-1 text-right font-normal">Lượt click</th>
            <th className="py-1 text-right font-normal">Khách</th>
            <th className="py-1 text-right font-normal">Nghi vấn</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.ctv_ref || "-"} className="border-t">
              <td className="py-1.5 text-muted-foreground tabular-nums">{r.rank}</td>
              <td className="py-1.5">
                {r.ctv_ref ? (
                  <Link href={`/ctvs/${encodeURIComponent(r.ctv_ref)}${search}`} className="text-primary hover:underline">
                    {r.ctv_display}
                  </Link>
                ) : (
                  <Link href={`/ctvs/unidentified${search}`} className="italic text-muted-foreground hover:underline">
                    {r.ctv_display}
                  </Link>
                )}
                {r.name && <span className="ml-2 text-xs text-muted-foreground">{r.name}</span>}
              </td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.new_links)}</td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.active_links)}</td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.clicks)}</td>
              <td className="py-1.5 text-right tabular-nums">{fmtNumber(r.metrics.unique_clicks)}</td>
              <td className={`py-1.5 text-right tabular-nums ${r.metrics.suspicious_clicks > 0 ? "text-destructive" : ""}`}>
                {fmtNumber(r.metrics.suspicious_clicks)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
