// Quy tắc BFF dùng chung cho route handler và test.

/** Chỉ các nhóm endpoint này được proxy sang portal-api (auth/* đi qua /api/auth riêng). */
const ALLOW = /^v1\/(me|filters|reports|links|params|param-registry|saved-reports|exports)(\/[A-Za-z0-9._~%-]+)*$/;

export function isAllowedPath(path: string): boolean {
  if (path.includes("..") || path.includes("//")) return false;
  return ALLOW.test(path);
}

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

/**
 * CSRF: request ghi phải có Origin trùng origin của Portal.
 * (Cookie SameSite=Lax đã chặn phần lớn; kiểm Origin là lớp thứ hai.)
 */
export function isSameOrigin(method: string, origin: string | null, expected: string): boolean {
  if (SAFE_METHODS.has(method.toUpperCase())) return true;
  if (!origin) return false;
  return origin === expected;
}

/** Đường dẫn quay lại sau đăng nhập: chỉ chấp nhận đường dẫn nội bộ (chống open redirect). */
export function safeNext(next: string | null | undefined, fallback = "/overview"): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return fallback;
  if (next.startsWith("/login") || next.startsWith("/api/")) return fallback;
  return next;
}
