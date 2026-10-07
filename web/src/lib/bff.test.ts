import { describe, expect, it } from "vitest";
import { isAllowedPath, isSameOrigin, safeNext } from "./bff";

describe("BFF allow-list", () => {
  it("cho phép endpoint báo cáo", () => {
    expect(isAllowedPath("v1/me")).toBe(true);
    expect(isAllowedPath("v1/filters/accounts")).toBe(true);
    expect(isAllowedPath("v1/reports/links/abc123")).toBe(true);
    expect(isAllowedPath("v1/exports/66f0/download")).toBe(true);
  });
  it("chặn auth, đi ngược thư mục, endpoint lạ", () => {
    expect(isAllowedPath("v1/auth/login")).toBe(false);
    expect(isAllowedPath("v1/reports/../auth/refresh")).toBe(false);
    expect(isAllowedPath("v1//me")).toBe(false);
    expect(isAllowedPath("metrics")).toBe(false);
    expect(isAllowedPath("v2/me")).toBe(false);
  });
});

describe("CSRF Origin", () => {
  it("GET không cần Origin; POST phải đúng origin", () => {
    expect(isSameOrigin("GET", null, "https://portal.example")).toBe(true);
    expect(isSameOrigin("POST", "https://portal.example", "https://portal.example")).toBe(true);
    expect(isSameOrigin("POST", "https://evil.example", "https://portal.example")).toBe(false);
    expect(isSameOrigin("DELETE", null, "https://portal.example")).toBe(false);
  });
});

describe("safeNext", () => {
  it("chỉ nhận đường dẫn nội bộ", () => {
    expect(safeNext("/campaigns?x=1")).toBe("/campaigns?x=1");
    expect(safeNext("//evil.com")).toBe("/overview");
    expect(safeNext("https://evil.com")).toBe("/overview");
    expect(safeNext("/\\evil.com")).toBe("/overview");
    expect(safeNext("/login")).toBe("/overview");
    expect(safeNext(null)).toBe("/overview");
  });
});
