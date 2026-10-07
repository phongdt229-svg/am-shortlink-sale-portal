import type { Metadata } from "next";
import { createSearchParamsCache, parseAsArrayOf, parseAsString } from "nuqs/server";
import { PageHeader } from "@/components/report/page-header";
import { CompareView } from "@/features/campaigns/compare-view";
import { data, serverApi } from "@/lib/api/server";
import { pageContext, type SearchParams } from "@/lib/filters/page";

export const metadata: Metadata = { title: "So sánh chiến dịch" };

const codesCache = createSearchParamsCache({ codes: parseAsArrayOf(parseAsString).withDefault([]) });

export default async function ComparePage({ searchParams }: { searchParams: SearchParams }) {
  const { api: query, filters, search } = await pageContext(searchParams);
  const { codes } = codesCache.parse(await searchParams);
  let result = null;
  if (codes.length >= 2) {
    const api = await serverApi();
    result = data(
      await api.GET("/v1/reports/campaigns/compare", {
        params: { query: { codes: codes.slice(0, 5), from: query.from, to: query.to, account: query.account, granularity: query.granularity } },
      }),
    );
  }
  return (
    <div className="space-y-4">
      <PageHeader title="So sánh chiến dịch" crumbs={[{ href: `/campaigns${search}`, label: "Chiến dịch" }]} />
      <CompareView data={result} accounts={filters.account} />
    </div>
  );
}
