"use client";
import { useState } from "react";
import { EmptyState } from "@/components/states";
import type { components } from "@/lib/api/schema";
import { fmtNumber, fmtPercent } from "@/lib/format";
import { cn } from "@/lib/utils";
import { BREAKDOWN_LABELS, SOURCE_LABELS } from "@/components/report/metric-defs";

type Breakdowns = components["schemas"]["Breakdowns"];
type Dim = keyof Breakdowns;

const ORDER: Dim[] = ["device", "source_group", "os", "browser", "country", "referer", "access_prefix"];

/** Phân rã theo 1 chiều: thanh ngang một màu (độ lớn), nhãn giá trị + % luôn hiển thị. Top 8 + "Khác". */
export function BreakdownPanel({ data, dims = ORDER }: { data: Breakdowns; dims?: Dim[] }) {
  const [dim, setDim] = useState<Dim>(dims[0]!);
  const items = data[dim];
  const top = items.slice(0, 8);
  const rest = items.slice(8);
  if (rest.length) {
    top.push({ key: "Khác", clicks: rest.reduce((a, b) => a + b.clicks, 0), share: rest.reduce((a, b) => a + b.share, 0) });
  }
  const max = Math.max(1, ...top.map((t) => t.clicks));
  const label = (k: string) => (dim === "source_group" ? (SOURCE_LABELS[k] ?? k) : dim === "access_prefix" ? `/${k}` : k);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-1" role="tablist">
        {dims.map((d) => (
          <button
            key={d}
            type="button"
            role="tab"
            aria-selected={dim === d}
            onClick={() => setDim(d)}
            className={cn("rounded-md px-2 py-1 text-xs", dim === d ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:bg-muted")}
          >
            {BREAKDOWN_LABELS[d]}
          </button>
        ))}
      </div>
      {top.length === 0 ? (
        <EmptyState />
      ) : (
        <ul className="space-y-1.5" role="tabpanel">
          {top.map((t) => (
            <li key={t.key} className="group grid grid-cols-[minmax(6rem,9rem)_1fr_auto] items-center gap-2 text-sm" title={`${label(t.key)}: ${fmtNumber(t.clicks)} click (${fmtPercent(t.share)})`}>
              <span className="truncate">{label(t.key)}</span>
              <span className="h-3 rounded-r bg-muted">
                <span className="block h-3 rounded-r bg-[var(--chart-1)] group-hover:opacity-80" style={{ width: `${(t.clicks / max) * 100}%` }} />
              </span>
              <span className="w-28 text-right text-xs tabular-nums">
                {fmtNumber(t.clicks)} <span className="text-muted-foreground">· {fmtPercent(t.share)}</span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
