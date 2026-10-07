import { describe, expect, it } from "vitest";
import { clicksQuery, compactFilters, decodeClickFilters, encodeClickFilters } from "./click-filters";

describe("ClickFilters ↔ URL", () => {
  it("mã hoá / giải mã khứ hồi, có tiếng Việt", () => {
    const f = { province: { values: ["Hồ Chí Minh"] }, device: { values: ["mobile"], exclude: true }, hour_from: 19, hour_to: 22 };
    const s = encodeClickFilters(f);
    expect(s).not.toMatch(/[+/=]/);
    expect(decodeClickFilters(s)).toEqual(f);
  });
  it("bỏ tiêu chí rỗng / mặc định", () => {
    expect(compactFilters({ device: { values: [] }, quality: "all", visit: "all", weekdays: [] })).toEqual({});
    expect(encodeClickFilters({ quality: "all" })).toBe("");
  });
  it("chuỗi rác → rỗng", () => {
    expect(decodeClickFilters("%%%")).toEqual({});
  });
  it("query: mảng lặp tham số, filters là JSON", () => {
    const q = new URLSearchParams(clicksQuery({ from: "2026-10-01", to: "2026-10-07", account: ["a", "b"] }, { quality: "valid" }, "c1", 100));
    expect(q.getAll("account")).toEqual(["a", "b"]);
    expect(JSON.parse(q.get("filters")!)).toEqual({ quality: "valid" });
    expect(q.get("cursor")).toBe("c1");
  });
});
