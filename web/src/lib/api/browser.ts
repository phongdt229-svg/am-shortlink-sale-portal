"use client";
import createClient from "openapi-fetch";
import type { paths } from "./schema";

/**
 * Client cho Client Components: gọi qua BFF cùng origin (/api/bff/v1/...).
 * Cookie phiên đi kèm tự động; trình duyệt không bao giờ thấy JWT.
 */
export const browserApi = createClient<paths>({ baseUrl: "/api/bff" });

browserApi.use({
  onResponse({ response }) {
    if (response.status === 401 && typeof window !== "undefined") {
      const next = window.location.pathname + window.location.search;
      window.location.assign(`/login?next=${encodeURIComponent(next)}`);
    }
    return response;
  },
});
