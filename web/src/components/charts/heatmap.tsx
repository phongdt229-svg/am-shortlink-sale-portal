"use client";
import { useState } from "react";
import type { components } from "@/lib/api/schema";
import { fmtNumber, WEEKDAYS_VN } from "@/lib/format";

type Cell = components["schemas"]["HeatmapCell"];

// Ramp tuần tự một sắc xanh (nhạt = ít). Bậc 0 dùng màu nền muted.
const RAMP = ["#cde2fb", "#9ec5f4", "#6da7ec", "#3987e5", "#256abf", "#184f95", "#0d366b"];
const ROWS = [1, 2, 3, 4, 5, 6, 0]; // T2 → CN

/** Heatmap giờ × thứ (giờ Việt Nam). Hover hiện số; có chú giải thang màu. */
export function Heatmap({ cells }: { cells: Cell[] }) {
  const [hover, setHover] = useState<Cell | null>(null);
  const grid = new Map(cells.map((c) => [`${c.weekday}-${c.hour}`, c]));
  const max = Math.max(1, ...cells.map((c) => c.clicks));
  const color = (n: number) => (n === 0 ? "var(--muted)" : RAMP[Math.min(RAMP.length - 1, Math.floor((n / max) * RAMP.length))]);

  return (
    <div className="space-y-2">
      <div className="overflow-x-auto">
        <table className="w-full border-separate border-spacing-[2px] text-[10px]" aria-label="Lượt click theo giờ và thứ">
          <thead>
            <tr>
              <th />
              {Array.from({ length: 24 }, (_, h) => (
                <th key={h} className="font-normal text-muted-foreground">
                  {h % 3 === 0 ? h : ""}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {ROWS.map((d) => (
              <tr key={d}>
                <th className="pr-1 text-right font-normal text-muted-foreground">{WEEKDAYS_VN[d]}</th>
                {Array.from({ length: 24 }, (_, h) => {
                  const c = grid.get(`${d}-${h}`) ?? { weekday: d, hour: h, clicks: 0 };
                  return (
                    <td
                      key={h}
                      className="h-5 min-w-4 rounded-sm"
                      style={{ background: color(c.clicks) }}
                      onMouseEnter={() => setHover(c)}
                      onMouseLeave={() => setHover(null)}
                      title={`${WEEKDAYS_VN[d]} ${h}h: ${fmtNumber(c.clicks)} click`}
                    />
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span aria-live="polite">{hover ? `${WEEKDAYS_VN[hover.weekday]} ${hover.hour}:00–${hover.hour}:59 · ${fmtNumber(hover.clicks)} click` : "Rê chuột lên ô để xem số"}</span>
        <span className="flex items-center gap-1">
          Ít
          {RAMP.map((c) => (
            <span key={c} className="inline-block size-3 rounded-sm" style={{ background: c }} />
          ))}
          Nhiều ({fmtNumber(max)})
        </span>
      </div>
    </div>
  );
}
