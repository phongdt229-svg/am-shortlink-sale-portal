"use client";
import { parseAsStringLiteral, useQueryState } from "nuqs";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

const TABS = ["report", "clicks"] as const;

/**
 * Tab trong màn hình chi tiết (tài khoản / chiến dịch / CTV / link): "Báo cáo" và "Lượt click".
 * Tab "Lượt click" khoá sẵn bộ lọc theo đối tượng đang xem (§4.1).
 */
export function DetailTabs({ children, clicks }: { children: ReactNode; clicks: ReactNode; explorerLock?: Record<string, string[]> }) {
  const [tab, setTab] = useQueryState("tab", parseAsStringLiteral(TABS).withDefault("report"));
  return (
    <div className="space-y-4">
      <div className="flex gap-1 border-b" role="tablist">
        {TABS.map((t) => (
          <button
            key={t}
            type="button"
            role="tab"
            aria-selected={tab === t}
            onClick={() => void setTab(t === "report" ? null : t)}
            className={cn("-mb-px border-b-2 px-3 py-2 text-sm", tab === t ? "border-primary font-medium text-foreground" : "border-transparent text-muted-foreground hover:text-foreground")}
          >
            {t === "report" ? "Báo cáo" : "Lượt click"}
          </button>
        ))}
      </div>
      <div role="tabpanel">{tab === "report" ? children : clicks}</div>
    </div>
  );
}
