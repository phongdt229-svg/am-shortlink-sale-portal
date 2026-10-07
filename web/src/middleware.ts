import { NextResponse, type NextRequest } from "next/server";
import { safeNext } from "@/lib/bff";
import { ensureFresh } from "@/lib/session/refresh";
import { cookieOptions, openSession, sealSession, SESSION_COOKIE } from "@/lib/session/session";

/**
 * Bắt buộc đăng nhập cho mọi trang (trừ /login):
 * - chưa có phiên → /login?next=<url đang xem>  (BFF → 401 problem+json)
 * - access token sắp hết hạn → refresh với portal-api, ghi lại cookie (cả request lẫn response
 *   để Server Component trong cùng request đọc được token mới)
 * - gia hạn phiên khi người dùng hoạt động (idle 8h).
 */
export async function middleware(req: NextRequest) {
  const secret = process.env.PORTAL_SESSION_SECRET ?? "";
  const apiUrl = process.env.PORTAL_API_URL ?? "http://localhost:8080";
  const idleHours = Number(process.env.SESSION_IDLE_HOURS ?? 8);
  const secure = process.env.COOKIE_SECURE ? process.env.COOKIE_SECURE === "true" : process.env.NODE_ENV === "production";

  const { pathname, search } = req.nextUrl;
  const isLogin = pathname === "/login";
  const isBff = pathname.startsWith("/api/bff/");

  const session = await openSession(req.cookies.get(SESSION_COOKIE)?.value, secret, idleHours);

  if (!session) {
    if (isLogin) return NextResponse.next();
    if (isBff) {
      return Response.json(
        { type: "about:blank", status: 401, code: "session_expired", title: "Phiên đăng nhập đã hết hạn" },
        { status: 401, headers: { "content-type": "application/problem+json" } },
      );
    }
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    url.search = "";
    if (pathname !== "/") url.searchParams.set("next", pathname + search);
    const res = NextResponse.redirect(url);
    if (req.cookies.has(SESSION_COOKIE)) res.cookies.delete(SESSION_COOKIE);
    return res;
  }

  if (isLogin) {
    const url = req.nextUrl.clone();
    url.pathname = safeNext(req.nextUrl.searchParams.get("next"));
    url.search = "";
    return NextResponse.redirect(url);
  }

  const fresh = await ensureFresh(apiUrl, session);
  if (fresh.kind === "expired") {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    url.search = `?next=${encodeURIComponent(pathname + search)}`;
    const res = isBff
      ? Response.json({ type: "about:blank", status: 401, code: "session_expired", title: "Phiên đăng nhập đã hết hạn" }, { status: 401 })
      : NextResponse.redirect(url);
    if (res instanceof NextResponse) res.cookies.delete(SESSION_COOKIE);
    return res;
  }
  if (!fresh.changed) return NextResponse.next();

  const sealed = await sealSession(fresh.session, secret);
  req.cookies.set(SESSION_COOKIE, sealed);
  const res = NextResponse.next({ request: { headers: req.headers } });
  res.cookies.set(SESSION_COOKIE, sealed, cookieOptions(secure, idleHours));
  return res;
}

export const config = {
  // Bỏ qua file tĩnh và route đăng nhập/đăng xuất (tự xử lý).
  matcher: ["/((?!_next/static|_next/image|favicon.ico|robots.txt|api/auth/).*)"],
};
