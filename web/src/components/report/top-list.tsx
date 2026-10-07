import Link from "next/link";
import { EmptyState } from "@/components/states";
import type { components } from "@/lib/api/schema";
import { fmtNumber } from "@/lib/format";

type TopItem = components["schemas"]["TopItem"];

/** Bảng top N gọn; mỗi dòng dẫn tới màn hình chi tiết (giữ bộ lọc qua `search`). */
export function TopList({ items, href, search = "", showOwner }: { items: TopItem[]; href?: (it: TopItem) => string; search?: string; showOwner?: boolean }) {
  if (!items.length) return <EmptyState className="py-6" />;
  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="text-xs text-muted-foreground">
          <th className="w-6 py-1 text-left font-normal">#</th>
          <th className="py-1 text-left font-normal">Tên</th>
          <th className="py-1 text-right font-normal">Click</th>
          <th className="hidden py-1 text-right font-normal sm:table-cell">Khách</th>
        </tr>
      </thead>
      <tbody>
        {items.map((it, i) => (
          <tr key={it.key + i} className="border-t">
            <td className="py-1.5 text-xs text-muted-foreground">{i + 1}</td>
            <td className="max-w-0 truncate py-1.5">
              {href ? (
                <Link href={href(it) + search} className="hover:text-primary hover:underline">
                  {it.label}
                </Link>
              ) : (
                it.label
              )}
              {showOwner && it.owner && <span className="ml-1 text-xs text-muted-foreground">· {it.owner}</span>}
            </td>
            <td className="py-1.5 text-right tabular-nums">{fmtNumber(it.clicks)}</td>
            <td className="hidden py-1.5 text-right tabular-nums text-muted-foreground sm:table-cell">{fmtNumber(it.unique_clicks)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
