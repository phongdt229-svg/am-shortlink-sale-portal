import "server-only";
import { createSearchParamsCache } from "nuqs/server";
import { filterParsers, type Filters } from "./index";

/** Đọc bộ lọc từ searchParams trong Server Component. */
export const filtersCache = createSearchParamsCache(filterParsers);

export async function readFilters(searchParams: Promise<Record<string, string | string[] | undefined>>): Promise<Filters> {
  return filtersCache.parse(await searchParams) as Filters;
}
