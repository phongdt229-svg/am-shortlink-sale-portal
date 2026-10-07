import "server-only";
import { createSearchParamsCache, parseAsInteger, parseAsString, parseAsStringLiteral } from "nuqs/server";
import { filtersToSearch, toApiQuery, type Filters } from "./index";
import { readFilters } from "./server";

export type SearchParams = Promise<Record<string, string | string[] | undefined>>;

const tableCache = createSearchParamsCache({
  page: parseAsInteger.withDefault(1),
  sort: parseAsString,
  order: parseAsStringLiteral(["asc", "desc"] as const).withDefault("desc"),
  q: parseAsString.withDefault(""),
});

/** Đọc bộ lọc chung + tham số bảng; trả query sẵn cho portal-api và chuỗi giữ bộ lọc. */
export async function pageContext(searchParams: SearchParams) {
  const raw = await searchParams;
  const f: Filters = await readFilters(Promise.resolve(raw));
  const t = tableCache.parse(raw);
  return {
    filters: f,
    api: toApiQuery(f),
    table: { page: t.page, sort: t.sort ?? undefined, order: t.order, q: t.q || undefined },
    search: filtersToSearch(f),
  };
}
