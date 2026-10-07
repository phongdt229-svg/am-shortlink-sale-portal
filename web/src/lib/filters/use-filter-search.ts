"use client";
import { useSearchParams } from "next/navigation";

const KEYS = ["preset", "from", "to", "account", "campaign", "ctv", "prefix", "compare", "granularity"];

/** Chuỗi ?query chỉ gồm bộ lọc chung — để drill-down giữ nguyên bộ lọc. */
export function useFilterSearch(drop: string[] = []): string {
  const sp = useSearchParams();
  const p = new URLSearchParams();
  for (const k of KEYS) {
    if (drop.includes(k)) continue;
    const v = sp.get(k);
    if (v) p.set(k, v);
  }
  const s = p.toString();
  return s ? `?${s}` : "";
}
