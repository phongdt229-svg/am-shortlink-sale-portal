// Định dạng hiển thị chuẩn Việt Nam. Frontend CHỈ định dạng — không tự tính lại chỉ số (PG5).
import { TZDate } from "@date-fns/tz";
import { format } from "date-fns";

export const TZ = "Asia/Ho_Chi_Minh";

const intFmt = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 0 });
const decFmt = new Intl.NumberFormat("vi-VN", { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const compactFmt = new Intl.NumberFormat("vi-VN", { notation: "compact", maximumFractionDigits: 1 });

/** 1234567 → "1.234.567" */
export function fmtNumber(n: number | null | undefined): string {
  if (n === null || n === undefined || Number.isNaN(n)) return "–";
  return intFmt.format(n);
}

/** 1234567 → "1,2 Tr" (trục biểu đồ) */
export function fmtCompact(n: number): string {
  return compactFmt.format(n);
}

/** Số thập phân 1 chữ số: 2.345 → "2,3" */
export function fmtDecimal(n: number | null | undefined): string {
  if (n === null || n === undefined || !Number.isFinite(n)) return "–";
  return decFmt.format(n);
}

/** Tỉ lệ 0.1234 → "12,3%" */
export function fmtPercent(ratio: number | null | undefined): string {
  if (ratio === null || ratio === undefined || !Number.isFinite(ratio)) return "–";
  return `${decFmt.format(ratio * 100)}%`;
}

export type Trend = "up" | "down" | "flat";

/** % thay đổi do API trả (vd 0.25 = +25%); null = kỳ trước bằng 0. */
export function fmtDelta(change: number | null | undefined): { text: string; trend: Trend } {
  if (change === null || change === undefined || !Number.isFinite(change)) return { text: "–", trend: "flat" };
  const trend: Trend = change > 0.0005 ? "up" : change < -0.0005 ? "down" : "flat";
  const sign = trend === "up" ? "+" : "";
  return { text: `${sign}${decFmt.format(change * 100)}%`, trend };
}

/** "2026-10-07" | Date → "07/10/2026" (ngày lịch, không đổi múi giờ) */
export function fmtDate(d: string | Date | null | undefined): string {
  if (!d) return "–";
  if (typeof d === "string" && /^\d{4}-\d{2}-\d{2}$/.test(d)) {
    const [y, m, day] = d.split("-");
    return `${day}/${m}/${y}`;
  }
  return format(new TZDate(new Date(d), TZ), "dd/MM/yyyy");
}

/** ISO timestamp → "07/10/2026 14:05" theo giờ Việt Nam */
export function fmtDateTime(iso: string | Date | null | undefined, withSeconds = false): string {
  if (!iso) return "–";
  return format(new TZDate(new Date(iso), TZ), withSeconds ? "dd/MM/yyyy HH:mm:ss" : "dd/MM/yyyy HH:mm");
}

/** Ngày hôm nay theo giờ Việt Nam dạng "yyyy-MM-dd". */
export function todayVN(now: Date = new Date()): string {
  return format(new TZDate(now, TZ), "yyyy-MM-dd");
}

export const WEEKDAYS_VN = ["CN", "T2", "T3", "T4", "T5", "T6", "T7"];
