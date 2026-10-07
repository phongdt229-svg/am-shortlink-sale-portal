// Làm mới access token khi sắp hết hạn (dùng chung cho middleware và BFF).
import type { components } from "@/lib/api/schema";
import { nowSec, sessionFromTokens, type Session } from "./session";

/** Access token còn ít hơn ngưỡng này (giây) thì refresh trước. */
export const REFRESH_SKEW = 60;
/** Ghi lại cookie để gia hạn phiên nếu lần hoạt động trước đã cũ hơn (giây). */
export const SEEN_UPDATE = 300;

export type FreshResult = { kind: "ok"; session: Session; changed: boolean } | { kind: "expired" };

export async function ensureFresh(apiUrl: string, s: Session, now = nowSec()): Promise<FreshResult> {
  if (s.atExp - now > REFRESH_SKEW) {
    if (now - s.seen > SEEN_UPDATE) return { kind: "ok", session: { ...s, seen: now }, changed: true };
    return { kind: "ok", session: s, changed: false };
  }
  try {
    const res = await fetch(`${apiUrl}/v1/auth/refresh`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ refresh_token: s.rt }),
      cache: "no-store",
    });
    if (!res.ok) return { kind: "expired" };
    const t = (await res.json()) as components["schemas"]["TokenPair"];
    return { kind: "ok", session: sessionFromTokens(t, now), changed: true };
  } catch {
    // portal-api tạm không gọi được: còn hạn thì dùng tiếp, hết hạn thì coi như hết phiên.
    return s.atExp > now ? { kind: "ok", session: s, changed: false } : { kind: "expired" };
  }
}
