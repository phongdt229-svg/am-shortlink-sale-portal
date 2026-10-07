import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { NuqsTestingAdapter } from "nuqs/adapters/testing";
import type { ReactElement } from "react";

/** Render component client với React Query + nuqs (URL giả lập). */
export function renderWithProviders(ui: ReactElement, { search = "" }: { search?: string } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <NuqsTestingAdapter searchParams={search} hasMemory>
        {ui}
      </NuqsTestingAdapter>
    </QueryClientProvider>,
  );
}
