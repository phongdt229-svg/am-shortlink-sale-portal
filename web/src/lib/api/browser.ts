"use client";
import createClient from "openapi-fetch";
import type { paths } from "./schema";

/**
 * Client cho Client Components: gọi qua BFF cùng origin (/api/bff/v1/...).
 * Cookie phiên đi kèm tự động; trình duyệt không bao giờ thấy JWT.
 */
export const browserApi = createClient<paths>({
  // Origin tuyệt đối: Request của Node (test jsdom) không nhận URL tương đối; trên trình duyệt kết quả như nhau.
  baseUrl: typeof window !== "undefined" ? `${window.location.origin}/api/bff` : "/api/bff",
  // Gọi globalThis.fetch lúc chạy (không giữ tham chiếu lúc import) → instrumentation / mock vá fetch sau vẫn áp dụng.
  fetch: (input) => globalThis.fetch(input),
});

browserApi.use({
  onResponse({ response }) {
    if (response.status === 401 && typeof window !== "undefined") {
      const next = window.location.pathname + window.location.search;
      window.location.assign(`/login?next=${encodeURIComponent(next)}`);
    }
    return response;
  },
});
