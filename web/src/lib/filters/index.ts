// Bộ lọc dùng chung ↔ URL (nuqs). Trạng thái lọc nằm trên URL → chia sẻ link báo cáo, back/forward đúng.
// Import từ "nuqs/server": dùng được ở cả Server lẫn Client Component (import "nuqs" ở RSC ra client reference).
import { parseAsArrayOf, parseAsBoolean, parseAsString, parseAsStringLiteral } from "nuqs/server";
import { addDays, differenceInCalendarDays, endOfMonth, format, parseISO, startOfMonth, subMonths } from "date-fns";
import { todayVN } from "@/lib/format";

export const PRESETS = ["today", "7d", "30d", "this_month", "last_month", "custom"] as const;
export type Preset = (typeof PRESETS)[number];

export const PRESET_LABELS: Record<Preset, string> = {
  today: "Hôm nay",
  "7d": "7 ngày",
  "30d": "30 ngày",
  this_month: "Tháng này",
  last_month: "Tháng trước",
  custom: "Tuỳ chọn",
};

export const GRANULARITIES = ["day", "week", "month"] as const;
export type Granularity = (typeof GRANULARITIES)[number];
export const GRANULARITY_LABELS: Record<Granularity, string> = { day: "Ngày", week: "Tuần", month: "Tháng" };

/** Giới hạn khoảng ngày: số liệu tổng hợp ≤ 12 tháng (366 ngày). */
export const MAX_RANGE_DAYS = 366;

const isoDate = parseAsString; // yyyy-MM-dd, kiểm ở resolveRange

export const filterParsers = {
  preset: parseAsStringLiteral(PRESETS).withDefault("30d"),
  from: isoDate,
  to: isoDate,
  account: parseAsArrayOf(parseAsString).withDefault([]),
  campaign: parseAsArrayOf(parseAsString).withDefault([]),
  ctv: parseAsArrayOf(parseAsString).withDefault([]),
  prefix: parseAsArrayOf(parseAsString).withDefault([]),
  compare: parseAsBoolean.withDefault(false),
  granularity: parseAsStringLiteral(GRANULARITIES).withDefault("day"),
};

export interface Filters {
  preset: Preset;
  from: string | null;
  to: string | null;
  account: string[];
  campaign: string[];
  ctv: string[];
  prefix: string[];
  compare: boolean;
  granularity: Granularity;
}

export interface DateRange {
  from: string; // yyyy-MM-dd (gồm cả ngày)
  to: string;
  error?: string;
}

const ISO = /^\d{4}-\d{2}-\d{2}$/;
const fmt = (d: Date) => format(d, "yyyy-MM-dd");

/** Đổi preset / from-to thành khoảng ngày cụ thể (theo ngày lịch Việt Nam). */
export function resolveRange(f: Pick<Filters, "preset" | "from" | "to">, today = todayVN()): DateRange {
  const t = parseISO(today);
  switch (f.preset) {
    case "today":
      return { from: today, to: today };
    case "7d":
      return { from: fmt(addDays(t, -6)), to: today };
    case "30d":
      return { from: fmt(addDays(t, -29)), to: today };
    case "this_month":
      return { from: fmt(startOfMonth(t)), to: today };
    case "last_month": {
      const lm = subMonths(t, 1);
      return { from: fmt(startOfMonth(lm)), to: fmt(endOfMonth(lm)) };
    }
    case "custom": {
      const from = f.from && ISO.test(f.from) ? f.from : fmt(addDays(t, -29));
      let to = f.to && ISO.test(f.to) ? f.to : today;
      if (to > today) to = today;
      if (from > to) return { from: to, to, error: "Ngày bắt đầu sau ngày kết thúc" };
      if (differenceInCalendarDays(parseISO(to), parseISO(from)) + 1 > MAX_RANGE_DAYS) {
        return { from: fmt(addDays(parseISO(to), -(MAX_RANGE_DAYS - 1))), to, error: "Khoảng ngày tối đa 12 tháng — đã tự thu hẹp" };
      }
      return { from, to };
    }
  }
}

/**
 * Tham số query gửi portal-api. Mảng → lặp tham số (account=a&account=b) theo OpenAPI (style form, explode).
 */
export function toApiQuery(f: Filters, today?: string) {
  const r = resolveRange(f, today);
  return {
    from: r.from,
    to: r.to,
    account: f.account.length ? f.account : undefined,
    campaign: f.campaign.length ? f.campaign : undefined,
    ctv: f.ctv.length ? f.ctv : undefined,
    prefix: f.prefix.length ? f.prefix : undefined,
    compare: f.compare || undefined,
    granularity: f.granularity,
  };
}

/** Chuỗi query của bộ lọc hiện tại (để giữ bộ lọc khi chuyển trang / drill-down). */
export function filtersToSearch(f: Partial<Filters>, extra?: Record<string, string>): string {
  const p = new URLSearchParams();
  if (f.preset && f.preset !== "30d") p.set("preset", f.preset);
  if (f.preset === "custom") {
    if (f.from) p.set("from", f.from);
    if (f.to) p.set("to", f.to);
  }
  for (const k of ["account", "campaign", "ctv", "prefix"] as const) {
    const v = f[k];
    if (v && v.length) p.set(k, v.join(","));
  }
  if (f.compare) p.set("compare", "true");
  if (f.granularity && f.granularity !== "day") p.set("granularity", f.granularity);
  for (const [k, v] of Object.entries(extra ?? {})) p.set(k, v);
  const s = p.toString();
  return s ? `?${s}` : "";
}
