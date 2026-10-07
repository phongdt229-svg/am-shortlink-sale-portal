import { NextResponse, type NextRequest } from "next/server";
import { isSameOrigin } from "@/lib/bff";
import { serverEnv } from "@/lib/env";
import { problemJson, publicOrigin } from "@/lib/request";
import { openSession, SESSION_COOKIE } from "@/lib/session/session";

export const dynamic = "force-dynamic";

/** Đăng xuất: thu hồi refresh token ở portal-api (cả chuỗi) và xoá cookie phiên. */
export async function POST(req: NextRequest) {
  const env = serverEnv();
  if (!isSameOrigin("POST", req.headers.get("origin"), publicOrigin(req, env.PORTAL_PUBLIC_ORIGIN))) {
    return problemJson(403, "bad_origin", "Không có quyền");
  }
  const s = await openSession(req.cookies.get(SESSION_COOKIE)?.value, env.PORTAL_SESSION_SECRET, env.SESSION_IDLE_HOURS);
  if (s) {
    await fetch(`${env.PORTAL_API_URL}/v1/auth/logout`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ refresh_token: s.rt }),
      cache: "no-store",
    }).catch(() => undefined); // portal-api lỗi vẫn xoá phiên phía trình duyệt
  }
  const out = new NextResponse(null, { status: 204 });
  out.cookies.delete(SESSION_COOKIE);
  return out;
}
