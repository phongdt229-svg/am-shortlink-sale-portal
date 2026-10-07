import { describe, expect, it } from "vitest";
import { filtersToSearch, resolveRange, toApiQuery, type Filters } from "./index";

const base: Filters = {
  preset: "30d",
  from: null,
  to: null,
  account: [],
  campaign: [],
  ctv: [],
  prefix: [],
  compare: false,
  granularity: "day",
};

describe("resolveRange", () => {
  const today = "2026-10-07";
  it("presets", () => {
    expect(resolveRange({ ...base, preset: "today" }, today)).toEqual({ from: today, to: today });
    expect(resolveRange({ ...base, preset: "7d" }, today)).toEqual({ from: "2026-10-01", to: today });
    expect(resolveRange({ ...base, preset: "30d" }, today)).toEqual({ from: "2026-09-08", to: today });
    expect(resolveRange({ ...base, preset: "this_month" }, today)).toEqual({ from: "2026-10-01", to: today });
    expect(resolveRange({ ...base, preset: "last_month" }, today)).toEqual({ from: "2026-09-01", to: "2026-09-30" });
  });

  it("custom: kẹp tương lai, đảo ngược, > 12 tháng", () => {
    expect(resolveRange({ preset: "custom", from: "2026-09-01", to: "2026-12-01" }, today)).toEqual({ from: "2026-09-01", to: today });
    expect(resolveRange({ preset: "custom", from: "2026-10-05", to: "2026-10-01" }, today).error).toBeDefined();
    const long = resolveRange({ preset: "custom", from: "2024-01-01", to: "2026-10-01" }, today);
    expect(long.error).toMatch(/12 tháng/);
    expect(long.from).toBe("2025-10-01");
  });

  it("custom: giá trị rác → mặc định 30 ngày", () => {
    expect(resolveRange({ preset: "custom", from: "abc", to: "x" }, today)).toEqual({ from: "2026-09-08", to: today });
  });
});

describe("toApiQuery / filtersToSearch", () => {
  it("bỏ tham số rỗng", () => {
    expect(toApiQuery(base, "2026-10-07")).toEqual({
      from: "2026-09-08",
      to: "2026-10-07",
      account: undefined,
      campaign: undefined,
      ctv: undefined,
      prefix: undefined,
      compare: undefined,
      granularity: "day",
    });
  });

  it("giữ bộ lọc khi drill-down", () => {
    expect(filtersToSearch({ ...base, account: ["a", "b"], compare: true }, { tab: "clicks" })).toBe(
      "?account=a%2Cb&compare=true&tab=clicks",
    );
    expect(filtersToSearch(base)).toBe("");
  });
});
