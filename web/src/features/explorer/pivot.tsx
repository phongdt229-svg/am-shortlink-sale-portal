"use client";
import Link from "next/link";
import { fmtDate, fmtNumber } from "@/lib/format";
import { dimLabel, TIME_DIMS } from "./dims";
import { drillDownHref, type ExplorerQuery, type ExplorerRow } from "./state";

const MAX_COLS = 10;

/** Pivot: dòng = chiều 1, cột = chiều 2 (top 10 + Khác), ô = lượt click; tổng dòng / cột; bấm ô → click thô. */
export function Pivot({ rows, dims, query }: { rows: ExplorerRow[]; dims: string[]; query: ExplorerQuery }) {
  if (dims.length < 2) return null;
  const lbl = (dim: string, l: string) => (TIME_DIMS.has(dim) ? fmtDate(l) : l);
  const colTotals = new Map<string, { value: string; label: string; n: number }>();
  for (const r of rows) {
    const k = r.keys[1]!;
    const c = colTotals.get(k.value) ?? { value: k.value, label: k.label, n: 0 };
    c.n += r.metrics.clicks;
    colTotals.set(k.value, c);
  }
  const cols = [...colTotals.values()].sort((a, b) => b.n - a.n);
  const shown = cols.slice(0, MAX_COLS);
  const hasOther = cols.length > MAX_COLS;
  const shownSet = new Set(shown.map((c) => c.value));

  const rowMap = new Map<string, { value: string; label: string; cells: Map<string, number>; other: number; total: number }>();
  for (const r of rows) {
    const k0 = r.keys[0]!;
    const row = rowMap.get(k0.value) ?? { value: k0.value, label: k0.label, cells: new Map(), other: 0, total: 0 };
    const v = r.metrics.clicks;
    if (shownSet.has(r.keys[1]!.value)) row.cells.set(r.keys[1]!.value, (row.cells.get(r.keys[1]!.value) ?? 0) + v);
    else row.other += v;
    row.total += v;
    rowMap.set(k0.value, row);
  }
  const list = [...rowMap.values()];
  if (TIME_DIMS.has(dims[0]!)) list.sort((a, b) => a.value.localeCompare(b.value));
  else list.sort((a, b) => b.total - a.total);
  const grand = list.reduce((a, r) => a + r.total, 0);

  return (
    <div className="overflow-x-auto rounded-md border">
      <table className="w-full text-xs">
        <thead className="bg-muted/60 text-muted-foreground">
          <tr>
            <th className="sticky left-0 bg-muted/60 px-2 py-1.5 text-left font-medium">
              {dimLabel(dims[0]!)} \ {dimLabel(dims[1]!)}
            </th>
            {shown.map((c) => (
              <th key={c.value} className="max-w-28 truncate px-2 py-1.5 text-right font-medium" title={c.label}>
                {lbl(dims[1]!, c.label)}
              </th>
            ))}
            {hasOther && <th className="px-2 py-1.5 text-right font-medium">Khác</th>}
            <th className="px-2 py-1.5 text-right font-semibold">Tổng</th>
          </tr>
        </thead>
        <tbody>
          {list.map((r) => (
            <tr key={r.value} className="border-t">
              <th className="sticky left-0 max-w-48 truncate bg-card px-2 py-1 text-left font-normal" title={r.label}>
                {lbl(dims[0]!, r.label)}
              </th>
              {shown.map((c) => {
                const v = r.cells.get(c.value) ?? 0;
                return (
                  <td key={c.value} className="px-2 py-1 text-right tabular-nums">
                    {v ? (
                      <Link
                        className="hover:text-primary hover:underline"
                        href={drillDownHref(query, [
                          { dim: dims[0]!, value: r.value },
                          { dim: dims[1]!, value: c.value },
                        ])}
                      >
                        {fmtNumber(v)}
                      </Link>
                    ) : (
                      <span className="text-muted-foreground/50">·</span>
                    )}
                  </td>
                );
              })}
              {hasOther && <td className="px-2 py-1 text-right tabular-nums text-muted-foreground">{fmtNumber(r.other)}</td>}
              <td className="px-2 py-1 text-right font-medium tabular-nums">{fmtNumber(r.total)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot className="border-t-2 bg-muted/40 font-medium">
          <tr>
            <th className="sticky left-0 bg-muted/40 px-2 py-1 text-left">Tổng</th>
            {shown.map((c) => (
              <td key={c.value} className="px-2 py-1 text-right tabular-nums">
                {fmtNumber(c.n)}
              </td>
            ))}
            {hasOther && <td className="px-2 py-1 text-right tabular-nums">{fmtNumber(cols.slice(MAX_COLS).reduce((a, c) => a + c.n, 0))}</td>}
            <td className="px-2 py-1 text-right tabular-nums">{fmtNumber(grand)}</td>
          </tr>
        </tfoot>
      </table>
    </div>
  );
}
