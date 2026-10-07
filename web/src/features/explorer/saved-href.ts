import { encodeState, type ExplorerState } from "./state";

export type Globals = { from: string; to: string; account?: string[]; campaign?: string[]; ctv?: string[]; prefix?: string[]; compare?: boolean; granularity?: string };

/** Dựng URL Explorer từ trạng thái đã lưu (bộ lọc chung + `ex`). Kỳ lưu dạng tuỳ chọn để mở lại đúng khoảng ngày. */
export function savedHref(query: Record<string, unknown>): string {
  const st = query.state as ExplorerState | undefined;
  const g = (query.globals ?? {}) as Globals;
  const p = new URLSearchParams();
  if (g.from && g.to) {
    p.set("preset", "custom");
    p.set("from", g.from);
    p.set("to", g.to);
  }
  for (const k of ["account", "campaign", "ctv", "prefix"] as const) if (g[k]?.length) p.set(k, g[k]!.join(","));
  if (g.compare) p.set("compare", "true");
  if (st) p.set("ex", encodeState(st));
  return `/clicks/explore?${p.toString()}`;
}

