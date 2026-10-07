import { type NextRequest } from "next/server";
import { isAllowedPath, isSameOrigin } from "@/lib/bff";
import { serverEnv } from "@/lib/env";
import { logger } from "@/lib/logger";
import { clientIp, problemJson, publicOrigin } from "@/lib/request";
import { openSession, SESSION_COOKIE } from "@/lib/session/session";

export const dynamic = "force-dynamic";

const FORWARD_RESPONSE_HEADERS = ["content-type", "content-disposition", "content-length", "x-request-id"];

/**
 * BFF: proxy có kiểm phiên tới portal-api.
 * - Chỉ đường dẫn trong allow-list; request ghi phải đúng Origin (CSRF).
 * - Gắn JWT từ cookie phiên (middleware đã refresh nếu sắp hết hạn).
 */
async function proxy(req: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  const env = serverEnv();
  const { path } = await ctx.params;
  const joined = path.map(encodeURIComponent).join("/");

  if (!isAllowedPath(joined)) return problemJson(404, "route_not_found", "Không tìm thấy");
  if (!isSameOrigin(req.method, req.headers.get("origin"), publicOrigin(req, env.PORTAL_PUBLIC_ORIGIN))) {
    return problemJson(403, "bad_origin", "Không có quyền", "request không đến từ Portal");
  }
  const s = await openSession(req.cookies.get(SESSION_COOKIE)?.value, env.PORTAL_SESSION_SECRET, env.SESSION_IDLE_HOURS);
  if (!s) return problemJson(401, "session_expired", "Phiên đăng nhập đã hết hạn");

  const headers: Record<string, string> = {
    authorization: `Bearer ${s.at}`,
    accept: req.headers.get("accept") ?? "application/json",
    "x-forwarded-for": clientIp(req),
  };
  const ct = req.headers.get("content-type");
  if (ct) headers["content-type"] = ct;

  const hasBody = !["GET", "HEAD"].includes(req.method);
  let upstream: Response;
  try {
    upstream = await fetch(`${env.PORTAL_API_URL}/${joined}${req.nextUrl.search}`, {
      method: req.method,
      headers,
      body: hasBody ? await req.arrayBuffer() : undefined,
      signal: req.signal, // người dùng huỷ → huỷ luôn truy vấn ở portal-api
      cache: "no-store",
    });
  } catch (err) {
    if (req.signal.aborted) return new Response(null, { status: 499 });
    logger.error({ err: String(err), path: joined }, "bff upstream error");
    return problemJson(502, "upstream_unavailable", "Hệ thống tạm thời không phản hồi", "vui lòng thử lại sau");
  }

  const out = new Headers({ "cache-control": "no-store" });
  for (const h of FORWARD_RESPONSE_HEADERS) {
    const v = upstream.headers.get(h);
    if (v) out.set(h, v);
  }
  return new Response(upstream.body, { status: upstream.status, headers: out });
}

export { proxy as GET, proxy as POST, proxy as PUT, proxy as PATCH, proxy as DELETE };
