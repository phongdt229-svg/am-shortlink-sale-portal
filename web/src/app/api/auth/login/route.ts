import { NextResponse, type NextRequest } from "next/server";
import type { components } from "@/lib/api/schema";
import { isSameOrigin } from "@/lib/bff";
import { serverEnv } from "@/lib/env";
import { logger } from "@/lib/logger";
import { clientIp, problemJson, publicOrigin } from "@/lib/request";
import { cookieOptions, sealSession, sessionFromTokens, SESSION_COOKIE } from "@/lib/session/session";

export const dynamic = "force-dynamic";

/** Đăng nhập: gọi portal-api, cất token vào cookie httpOnly đã mã hoá; trả về thông tin người dùng. */
export async function POST(req: NextRequest) {
  const env = serverEnv();
  if (!isSameOrigin("POST", req.headers.get("origin"), publicOrigin(req, env.PORTAL_PUBLIC_ORIGIN))) {
    return problemJson(403, "bad_origin", "Không có quyền", "request không đến từ Portal");
  }
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return problemJson(400, "invalid_request", "Yêu cầu không hợp lệ");
  }

  let res: Response;
  try {
    res = await fetch(`${env.PORTAL_API_URL}/v1/auth/login`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-forwarded-for": clientIp(req),
        "user-agent": req.headers.get("user-agent") ?? "",
      },
      body: JSON.stringify(body),
      cache: "no-store",
    });
  } catch (err) {
    logger.error({ err: String(err) }, "portal-api unreachable on login");
    return problemJson(503, "upstream_unavailable", "Hệ thống tạm thời không phản hồi", "vui lòng thử lại sau");
  }

  if (!res.ok) {
    // Chuyển nguyên problem+json của portal-api (đã thân thiện, có request_id).
    return new NextResponse(res.body, {
      status: res.status,
      headers: { "content-type": res.headers.get("content-type") ?? "application/problem+json" },
    });
  }

  const tokens = (await res.json()) as components["schemas"]["TokenPair"];
  const session = sessionFromTokens(tokens);
  const out = NextResponse.json({ user: tokens.user }, { headers: { "cache-control": "no-store" } });
  out.cookies.set(SESSION_COOKIE, await sealSession(session, env.PORTAL_SESSION_SECRET), cookieOptions(env.COOKIE_SECURE, env.SESSION_IDLE_HOURS));
  logger.info({ user: tokens.user.username, role: tokens.user.role }, "login");
  return out;
}
