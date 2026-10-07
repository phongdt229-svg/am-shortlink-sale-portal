import { describe, expect, it, vi, afterEach } from "vitest";
import { ensureFresh } from "./refresh";
import { openSession, sealSession, type Session } from "./session";

const SECRET = "test-session-secret-0123456789abcdef";
const now = 1_800_000_000;

const s: Session = {
  at: "ACCESS-TOKEN-PLAINTEXT-MARKER",
  atExp: now + 600,
  rt: "rt_x",
  rtExp: now + 8 * 3600,
  user: { username: "partner_a", role: "user" },
  seen: now,
};

afterEach(() => vi.restoreAllMocks());

describe("session cookie (JWE)", () => {
  it("mã hoá / giải mã; không đọc được token từ cookie", async () => {
    const sealed = await sealSession(s, SECRET);
    expect(sealed).not.toContain("ACCESS-TOKEN-PLAINTEXT-MARKER");
    expect(sealed).not.toContain(btoa("ACCESS-TOKEN-PLAINTEXT").slice(0, 16));
    expect(await openSession(sealed, SECRET, 8, now)).toEqual(s);
  });
  it("sai secret / bị sửa → null", async () => {
    const sealed = await sealSession(s, SECRET);
    expect(await openSession(sealed, "another-secret-0123456789abcdef-xx", 8, now)).toBeNull();
    expect(await openSession(sealed.slice(0, -4) + "AAAA", SECRET, 8, now)).toBeNull();
  });
  it("nhàn rỗi quá 8h hoặc refresh hết hạn → null", async () => {
    const sealed = await sealSession(s, SECRET);
    expect(await openSession(sealed, SECRET, 8, now + 8 * 3600 + 1)).toBeNull();
    const old = await sealSession({ ...s, rtExp: now - 1 }, SECRET);
    expect(await openSession(old, SECRET, 8, now)).toBeNull();
  });
});

describe("ensureFresh", () => {
  it("token còn hạn → không gọi API; gia hạn seen khi cũ", async () => {
    const f = vi.spyOn(globalThis, "fetch");
    expect(await ensureFresh("http://api", s, now + 10)).toEqual({ kind: "ok", session: s, changed: false });
    const r = await ensureFresh("http://api", s, now + 400);
    expect(r).toMatchObject({ kind: "ok", changed: true, session: { seen: now + 400 } });
    expect(f).not.toHaveBeenCalled();
  });
  it("sắp hết hạn → refresh", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      Response.json({
        access_token: "jwt2",
        access_expires_at: new Date((now + 900) * 1000).toISOString(),
        refresh_token: "rt_y",
        refresh_expires_at: new Date((now + 8 * 3600) * 1000).toISOString(),
        user: { username: "partner_a", role: "user", all_accounts: false, accounts: ["partner_a"] },
      }),
    );
    const r = await ensureFresh("http://api", s, now + 560);
    expect(r).toMatchObject({ kind: "ok", changed: true, session: { at: "jwt2", rt: "rt_y" } });
  });
  it("refresh bị từ chối → expired", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null, { status: 401 }));
    expect(await ensureFresh("http://api", s, now + 560)).toEqual({ kind: "expired" });
  });
});
