import { ArrowDownRight, ArrowUpRight, Info, Minus } from "lucide-react";
import type { components } from "@/lib/api/schema";
import { fmtDecimal, fmtDelta, fmtNumber } from "@/lib/format";
import { cn } from "@/lib/utils";
import { METRIC_DEFS, type MetricKey } from "./metric-defs";

type Kpis = components["schemas"]["Kpis"];

const DEFAULT_KEYS: MetricKey[] = ["total_links", "new_links", "active_links", "clicks", "unique_clicks", "bot_clicks", "suspicious_clicks", "ctr_per_link"];

/** Thẻ KPI: giá trị + % so kỳ trước (mũi tên). Click bot / nghi vấn tăng là xấu → tô ngược màu. */
export function KpiCards({ kpis, keys = DEFAULT_KEYS }: { kpis: Kpis; keys?: MetricKey[] }) {
  const badWhenUp = new Set<MetricKey>(["bot_clicks", "suspicious_clicks"]);
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 2xl:grid-cols-8">
      {keys.map((k) => {
        const v = kpis.current[k];
        if (v === undefined) return null;
        const ch = kpis.change?.[k];
        const d = fmtDelta(ch);
        const good = badWhenUp.has(k) ? d.trend === "down" : d.trend === "up";
        const bad = badWhenUp.has(k) ? d.trend === "up" : d.trend === "down";
        const Icon = d.trend === "up" ? ArrowUpRight : d.trend === "down" ? ArrowDownRight : Minus;
        return (
          <div key={k} className="rounded-[var(--radius)] border bg-card p-3">
            <p className="flex items-center gap-1 text-xs text-muted-foreground" title={METRIC_DEFS[k].hint}>
              {METRIC_DEFS[k].label}
              <Info className="size-3 opacity-60" aria-label={METRIC_DEFS[k].hint} />
            </p>
            <p className="mt-1 text-xl font-semibold tabular-nums">{k === "ctr_per_link" ? fmtDecimal(v) : fmtNumber(v)}</p>
            {kpis.change && (
              <p
                className={cn("mt-0.5 flex items-center gap-0.5 text-xs tabular-nums text-muted-foreground", good && "text-success", bad && "text-destructive")}
                title={kpis.previous ? `Kỳ trước: ${fmtNumber(kpis.previous[k] as number)}` : undefined}
              >
                <Icon className="size-3" />
                {d.text}
                <span className="sr-only"> so với kỳ trước</span>
              </p>
            )}
          </div>
        );
      })}
    </div>
  );
}
