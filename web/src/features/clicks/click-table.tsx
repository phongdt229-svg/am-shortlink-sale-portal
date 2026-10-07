"use client";
import Link from "next/link";
import { EmptyState } from "@/components/states";
import { Badge } from "@/components/ui/primitives";
import type { components } from "@/lib/api/schema";
import { fmtDateTime } from "@/lib/format";
import { SOURCE_LABELS } from "@/components/report/metric-defs";

type ClickRow = components["schemas"]["ClickRow"];

/** Bảng click thô — IP / SĐT đã được portal-api che theo vai trò. */
export function ClickTable({ rows, showLink = true }: { rows: ClickRow[]; showLink?: boolean }) {
  if (!rows.length) return <EmptyState title="Không có lượt click nào" />;
  return (
    <div className="overflow-x-auto rounded-md border">
      <table className="w-full text-sm">
        <thead className="bg-muted/60 text-xs text-muted-foreground">
          <tr>
            <th className="px-3 py-2 text-left font-medium">Thời gian</th>
            {showLink && <th className="px-3 py-2 text-left font-medium">Link</th>}
            {showLink && <th className="px-3 py-2 text-left font-medium">Tài khoản / chiến dịch</th>}
            <th className="px-3 py-2 text-left font-medium">CTV</th>
            <th className="px-3 py-2 text-left font-medium">Thiết bị</th>
            <th className="px-3 py-2 text-left font-medium">Nguồn</th>
            <th className="px-3 py-2 text-left font-medium">Vị trí</th>
            <th className="px-3 py-2 text-left font-medium">IP</th>
            <th className="px-3 py-2 text-left font-medium">Cờ</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={`${r.ts}-${i}`} className="border-t align-top hover:bg-muted/40">
              <td className="whitespace-nowrap px-3 py-2 tabular-nums">{fmtDateTime(r.ts, true)}</td>
              {showLink && (
                <td className="max-w-56 px-3 py-2">
                  <Link href={`/links/${encodeURIComponent(r.code)}`} className="font-medium text-primary hover:underline">
                    /{r.access_prefix || r.prefix}/{r.code}
                  </Link>
                  {r.access_prefix && r.prefix && r.access_prefix !== r.prefix && (
                    <span className="ml-1 text-xs text-muted-foreground" title="Link cấp prefix khác với prefix truy cập">
                      (cấp /{r.prefix})
                    </span>
                  )}
                  <p className="truncate text-xs text-muted-foreground" title={r.long_url}>
                    {r.long_url}
                  </p>
                </td>
              )}
              {showLink && (
                <td className="whitespace-nowrap px-3 py-2">
                  {r.owner}
                  <p className="text-xs text-muted-foreground">{r.campaign_code || "—"}</p>
                </td>
              )}
              <td className="whitespace-nowrap px-3 py-2">
                {r.ctv_ref ? (
                  <Link href={`/ctvs/${encodeURIComponent(r.ctv_ref)}`} className="hover:underline">
                    {r.ctv_display}
                  </Link>
                ) : (
                  <span className="italic text-muted-foreground">{r.ctv_display}</span>
                )}
              </td>
              <td className="whitespace-nowrap px-3 py-2">
                {r.device}
                <p className="text-xs text-muted-foreground">
                  {r.os} · {r.browser}
                </p>
              </td>
              <td className="whitespace-nowrap px-3 py-2">
                {SOURCE_LABELS[r.source_group] ?? r.source_group}
                {r.referer_host && <p className="text-xs text-muted-foreground">{r.referer_host}</p>}
              </td>
              <td className="whitespace-nowrap px-3 py-2">
                {r.country}
                {r.province && <p className="text-xs text-muted-foreground">{r.province}</p>}
              </td>
              <td className="whitespace-nowrap px-3 py-2 font-mono text-xs">{r.ip}</td>
              <td className="space-x-1 whitespace-nowrap px-3 py-2">
                {r.is_bot && <Badge tone="warning">Bot</Badge>}
                {r.is_suspicious && <Badge tone="danger">Nghi vấn</Badge>}
                {r.is_repeat && <Badge>Lặp lại</Badge>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
