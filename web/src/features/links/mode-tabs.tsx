"use client";
import { parseAsInteger, parseAsStringLiteral, useQueryStates } from "nuqs";
import { useTransition } from "react";
import { Select } from "@/components/ui/primitives";
import { cn } from "@/lib/utils";

const MODES = [
  { key: "clicks", label: "Top theo click" },
  { key: "growth", label: "Tăng trưởng so kỳ trước" },
  { key: "dead", label: "Link không có click" },
] as const;

export function ModeTabs({ mode, deadDays }: { mode: string; deadDays: number }) {
  const [pending, startTransition] = useTransition();
  const [, set] = useQueryStates(
    { mode: parseAsStringLiteral(["clicks", "growth", "dead"] as const), dead_days: parseAsInteger, page: parseAsInteger },
    { shallow: false, startTransition },
  );
  return (
    <div className={cn("flex flex-wrap items-center gap-3", pending && "opacity-70")}>
      <div className="flex gap-1 rounded-md border p-0.5" role="tablist">
        {MODES.map((m) => (
          <button
            key={m.key}
            type="button"
            role="tab"
            aria-selected={mode === m.key}
            onClick={() => void set({ mode: m.key === "clicks" ? null : m.key, page: null })}
            className={cn("rounded px-3 py-1 text-sm", mode === m.key ? "bg-primary text-primary-foreground" : "hover:bg-muted")}
          >
            {m.label}
          </button>
        ))}
      </div>
      {mode === "dead" && (
        <label className="flex items-center gap-2 text-sm">
          Không có click trong
          <Select value={String(deadDays)} onChange={(e) => void set({ dead_days: Number(e.target.value), page: null })} className="h-8">
            {[7, 14, 30, 60, 90].map((d) => (
              <option key={d} value={d}>
                {d} ngày
              </option>
            ))}
          </Select>
          gần nhất
        </label>
      )}
    </div>
  );
}
