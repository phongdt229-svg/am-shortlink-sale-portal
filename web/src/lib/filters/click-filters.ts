// ClickFilters (§4.1a) trên URL: tham số `cf` = base64url(JSON) — để drill-down / chia sẻ link nhật ký click.
import type { components } from "@/lib/api/schema";

export type ClickFilters = components["schemas"]["ClickFilters"];

export function b64urlEncode(s: string): string {
  const bytes = new TextEncoder().encode(s);
  let bin = "";
  bytes.forEach((b) => (bin += String.fromCharCode(b)));
  return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export function b64urlDecode(s: string): string {
  const b64 = s.replace(/-/g, "+").replace(/_/g, "/") + "===".slice((s.length + 3) % 4);
  const bin = atob(b64);
  return new TextDecoder().decode(Uint8Array.from(bin, (c) => c.charCodeAt(0)));
}

/** Bỏ tiêu chí rỗng để URL gọn. */
export function compactFilters(f: ClickFilters): ClickFilters {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(f)) {
    if (v === undefined || v === null || v === "") continue;
    if (typeof v === "object" && !Array.isArray(v) && "values" in v && (v as { values: unknown[] }).values.length === 0) continue;
    if (Array.isArray(v) && v.length === 0) continue;
    if ((k === "quality" || k === "visit") && v === "all") continue;
    if ((k === "campaign_presence" || k === "ctv_presence") && v === "any") continue;
    out[k] = v;
  }
  return out as ClickFilters;
}

export function encodeClickFilters(f: ClickFilters): string {
  const c = compactFilters(f);
  return Object.keys(c).length ? b64urlEncode(JSON.stringify(c)) : "";
}

export function decodeClickFilters(s: string | null | undefined): ClickFilters {
  if (!s) return {};
  try {
    const v = JSON.parse(b64urlDecode(s));
    return v && typeof v === "object" ? (v as ClickFilters) : {};
  } catch {
    return {};
  }
}

/** Query string gọi GET /v1/reports/clicks: mảng lặp tham số, `filters` là JSON. */
export function clicksQuery(base: Record<string, string | string[] | boolean | undefined>, filters: ClickFilters, cursor?: string, pageSize = 50): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(base)) {
    if (v === undefined || v === false) continue;
    if (Array.isArray(v)) v.forEach((x) => p.append(k, x));
    else p.set(k, String(v));
  }
  const c = compactFilters(filters);
  if (Object.keys(c).length) p.set("filters", JSON.stringify(c));
  if (cursor) p.set("cursor", cursor);
  p.set("page_size", String(pageSize));
  return p.toString();
}
