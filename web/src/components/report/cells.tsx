import { ArrowDownRight, ArrowUpRight } from "lucide-react";
import { fmtDelta } from "@/lib/format";
import { cn } from "@/lib/utils";

/** Ô % so kỳ trước trong bảng. */
export function DeltaCell({ value }: { value?: number | null }) {
  if (value === undefined) return null;
  const d = fmtDelta(value);
  const Icon = d.trend === "up" ? ArrowUpRight : d.trend === "down" ? ArrowDownRight : null;
  return (
    <span className={cn("inline-flex items-center gap-0.5 text-xs", d.trend === "up" && "text-success", d.trend === "down" && "text-destructive")}>
      {Icon && <Icon className="size-3" />}
      {d.text}
    </span>
  );
}
