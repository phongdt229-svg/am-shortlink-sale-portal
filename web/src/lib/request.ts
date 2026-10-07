import type { NextRequest } from "next/server";

/** Origin công khai của Portal (sau reverse proxy dùng X-Forwarded-*). */
export function publicOrigin(req: NextRequest, configured?: string): string {
  if (configured) return configured.replace(/\/$/, "");
  const proto = req.headers.get("x-forwarded-proto") ?? req.nextUrl.protocol.replace(":", "");
  const host = req.headers.get("x-forwarded-host") ?? req.headers.get("host") ?? req.nextUrl.host;
  return `${proto}://${host}`;
}

/** IP trình duyệt để portal-api rate limit đăng nhập / ghi nhật ký. */
export function clientIp(req: NextRequest): string {
  const xff = req.headers.get("x-forwarded-for");
  if (xff) return xff.split(",")[0]!.trim();
  return req.headers.get("x-real-ip") ?? "";
}

export function problemJson(status: number, code: string, title: string, detail?: string) {
  return Response.json(
    { type: "about:blank", status, code, title, detail },
    { status, headers: { "content-type": "application/problem+json", "cache-control": "no-store" } },
  );
}
