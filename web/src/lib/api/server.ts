import "server-only";
import createClient from "openapi-fetch";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { serverEnv } from "@/lib/env";
import { requireSession } from "@/lib/session/server";
import { unwrap } from "./problem";
import type { paths } from "./schema";

/**
 * Client portal-api cho Server Components: gắn JWT của phiên, chuyển IP trình duyệt.
 * traceparent được @vercel/otel tự gắn vào fetch.
 */
export async function serverApi() {
  const s = await requireSession();
  const h = await headers();
  return createClient<paths>({
    baseUrl: serverEnv().PORTAL_API_URL,
    headers: {
      authorization: `Bearer ${s.at}`,
      "x-forwarded-for": h.get("x-forwarded-for") ?? "",
    },
    cache: "no-store",
  });
}

/** Lấy data hoặc ném ApiError; 401 (token bị thu hồi / tài khoản bị khoá) → về /login. */
export function data<T>(r: { data?: T; error?: unknown; response: Response }): T {
  if (r.response.status === 401) redirect("/login");
  return unwrap(r);
}
