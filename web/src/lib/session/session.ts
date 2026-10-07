// Phiên trình duyệt ↔ portal-web: cookie httpOnly chứa JWE (dir + A256GCM).
// JWT / refresh token của portal-api chỉ nằm TRONG cookie đã mã hoá — trình duyệt không đọc được.
// Chạy được cả ở Edge (middleware) lẫn Node (route handler, RSC).
import { EncryptJWT, jwtDecrypt } from "jose";
import type { components } from "@/lib/api/schema";

export const SESSION_COOKIE = "portal_session";

export type Role = components["schemas"]["Role"];

export interface Session {
  at: string; // access token (JWT portal-api)
  atExp: number; // epoch giây
  rt: string; // refresh token
  rtExp: number;
  user: { username: string; role: Role };
  seen: number; // lần hoạt động gần nhất (epoch giây) — gia hạn khi hoạt động
}

let keyCache: { secret: string; key: Uint8Array } | undefined;

async function keyFor(secret: string): Promise<Uint8Array> {
  if (keyCache?.secret === secret) return keyCache.key;
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(secret));
  keyCache = { secret, key: new Uint8Array(digest) };
  return keyCache.key;
}

export async function sealSession(s: Session, secret: string): Promise<string> {
  return new EncryptJWT({ s })
    .setProtectedHeader({ alg: "dir", enc: "A256GCM" })
    .setIssuedAt()
    .encrypt(await keyFor(secret));
}

/** Giải mã cookie; sai / bị sửa / hết hạn nhàn rỗi / refresh hết hạn → null. */
export async function openSession(
  token: string | undefined,
  secret: string,
  idleHours: number,
  now = nowSec(),
): Promise<Session | null> {
  if (!token) return null;
  try {
    const { payload } = await jwtDecrypt(token, await keyFor(secret));
    const s = payload.s as Session | undefined;
    if (!s?.at || !s.rt) return null;
    if (now - s.seen > idleHours * 3600) return null;
    if (s.rtExp <= now) return null;
    return s;
  } catch {
    return null;
  }
}

export function nowSec(): number {
  return Math.floor(Date.now() / 1000);
}

export function cookieOptions(secure: boolean, idleHours: number) {
  return {
    httpOnly: true,
    secure,
    sameSite: "lax" as const,
    path: "/",
    maxAge: Math.round(idleHours * 3600),
  };
}

type TokenPair = components["schemas"]["TokenPair"];

export function sessionFromTokens(t: TokenPair, now = nowSec()): Session {
  return {
    at: t.access_token,
    atExp: Math.floor(Date.parse(t.access_expires_at) / 1000),
    rt: t.refresh_token,
    rtExp: Math.floor(Date.parse(t.refresh_expires_at) / 1000),
    user: { username: t.user.username, role: t.user.role },
    seen: now,
  };
}
