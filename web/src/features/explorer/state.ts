// Trạng thái Click Explorer trên URL (`ex` = base64url JSON) + chuyển thành request / drill-down.
import { addDays, endOfMonth, format, parseISO } from "date-fns";
import type { components } from "@/lib/api/schema";
import { b64urlDecode, b64urlEncode, compactFilters, encodeClickFilters, type ClickFilters } from "@/lib/filters/click-filters";

export type ExplorerQuery = components["schemas"]["ExplorerQuery"];
export type ExplorerRow = components["schemas"]["ExplorerRow"];
export type ChartKind = "auto" | "line" | "stacked" | "bar" | "heatmap" | "pie";
export type SortBy = NonNullable<ExplorerQuery["sort_by"]>;

export interface ExplorerState {
  g: string[]; // group_by
  f: ClickFilters;
  s?: SortBy;
  o?: "asc" | "desc";
  l?: number; // top N
  c?: ChartKind;
}

export const DEFAULT_STATE: ExplorerState = { g: ["day"], f: {}, s: "clicks", l: 100, c: "auto" };

export function encodeState(s: ExplorerState): string {
  return b64urlEncode(JSON.stringify({ ...s, f: compactFilters(s.f) }));
}

export function decodeState(raw: string | null | undefined): ExplorerState {
  if (!raw) return DEFAULT_STATE;
  try {
    const v = JSON.parse(b64urlDecode(raw)) as Partial<ExplorerState>;
    const g = Array.isArray(v.g) ? v.g.filter((x) => typeof x === "string").slice(0, 3) : DEFAULT_STATE.g;
    return { ...DEFAULT_STATE, ...v, g: g.length ? g : DEFAULT_STATE.g, f: v.f && typeof v.f === "object" ? v.f : {} };
  } catch {
    return DEFAULT_STATE;
  }
}

/** Phần bộ lọc chung khoá cứng (tab "Lượt click" trong chi tiết tài khoản / chiến dịch / CTV / link). */
export interface Lock {
  account?: string[];
  campaign?: string[];
  ctv?: string[];
  links?: string[];
}

export function buildQuery(
  st: ExplorerState,
  base: { from: string; to: string; account?: string[]; campaign?: string[]; ctv?: string[]; prefix?: string[]; compare?: boolean },
  lock?: Lock,
): ExplorerQuery {
  const f: ClickFilters = { ...st.f };
  if (lock?.links) f.links = { values: lock.links };
  return {
    from: base.from,
    to: base.to,
    account: lock?.account ?? base.account,
    campaign: lock?.campaign ?? base.campaign,
    ctv: lock?.ctv ?? base.ctv,
    prefix: base.prefix,
    compare: base.compare || undefined,
    filters: compactFilters(f),
    group_by: st.g,
    sort_by: st.s,
    order: st.o,
    limit: st.l,
  };
}

/** Bấm một ô / dòng → danh sách click thô (R6) với bộ lọc tương ứng (giữ bộ lọc hiện tại + khoá theo khoá của dòng). */
export function drillDownHref(q: ExplorerQuery, keys: { dim: string; value: string }[]): string {
  const p = new URLSearchParams();
  let from = q.from;
  let to = q.to;
  const account = [...(q.account ?? [])];
  let campaign = [...(q.campaign ?? [])];
  let ctv = [...(q.ctv ?? [])];
  let prefix = [...(q.prefix ?? [])];
  const f: ClickFilters = { ...(q.filters ?? {}) };
  const setCrit = (k: keyof ClickFilters, v: string) => {
    (f as Record<string, unknown>)[k] = { values: [v] };
  };
  for (const { dim, value } of keys) {
    switch (dim) {
      case "account":
        account.splice(0, account.length, value);
        break;
      case "campaign":
        campaign = [value];
        break;
      case "ctv":
        ctv = [value];
        break;
      case "link":
        setCrit("links", value);
        break;
      case "link_prefix":
        prefix = [value];
        break;
      case "day":
        from = to = value;
        break;
      case "week":
        from = value;
        to = format(addDays(parseISO(value), 6), "yyyy-MM-dd");
        break;
      case "month":
        from = value;
        to = format(endOfMonth(parseISO(value)), "yyyy-MM-dd");
        break;
      case "hour":
        f.hour_from = f.hour_to = Number(value);
        break;
      case "weekday":
        f.weekdays = [Number(value)];
        break;
      case "other":
        break;
      default:
        if (dim.startsWith("param.")) {
          const key = dim.slice(6);
          f.params = [...(f.params ?? []).filter((x) => x.key !== key), value ? { key, op: "eq", values: [value] } : { key, op: "not_exists" }];
        } else {
          setCrit(dim as keyof ClickFilters, value);
        }
    }
  }
  if (from < q.from) from = q.from;
  if (to > q.to) to = q.to;
  p.set("preset", "custom");
  p.set("from", from);
  p.set("to", to);
  if (account.length) p.set("account", account.join(","));
  if (campaign.length) p.set("campaign", campaign.join(","));
  if (ctv.length) p.set("ctv", ctv.join(","));
  if (prefix.length) p.set("prefix", prefix.join(","));
  const cf = encodeClickFilters(f);
  if (cf) p.set("cf", cf);
  return `/clicks?${p.toString()}`;
}
