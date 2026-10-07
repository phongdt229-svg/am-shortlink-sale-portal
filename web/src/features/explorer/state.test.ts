import { describe, expect, it } from "vitest";
import { decodeClickFilters } from "@/lib/filters/click-filters";
import { buildQuery, decodeState, drillDownHref, encodeState } from "./state";

describe("Explorer state", () => {
  it("URL khứ hồi; rác → mặc định", () => {
    const s = { g: ["day", "device"], f: { device: { values: ["mobile"] } }, s: "clicks" as const, l: 50 };
    expect(decodeState(encodeState(s))).toMatchObject(s);
    expect(decodeState("%%").g).toEqual(["day"]);
    expect(decodeState(encodeState({ g: ["a", "b", "c", "d"], f: {} })).g).toHaveLength(3);
  });

  it("khoá của tab chi tiết đè bộ lọc chung", () => {
    const q = buildQuery({ g: ["day"], f: {} }, { from: "2026-10-01", to: "2026-10-07", account: ["x"] }, { account: ["partner_a"], links: ["abc"] });
    expect(q.account).toEqual(["partner_a"]);
    expect(q.filters?.links?.values).toEqual(["abc"]);
  });

  it("drill-down: ngày + thiết bị + tham số → nhật ký click đúng bộ lọc", () => {
    const q = buildQuery({ g: ["day", "device", "param.utm_source"], f: { quality: "valid" } }, { from: "2026-09-01", to: "2026-10-07" });
    const href = drillDownHref(q, [
      { dim: "day", value: "2026-09-15" },
      { dim: "device", value: "mobile" },
      { dim: "param.utm_source", value: "zalo" },
    ]);
    const u = new URL(href, "http://x");
    expect(u.pathname).toBe("/clicks");
    expect(u.searchParams.get("from")).toBe("2026-09-15");
    expect(u.searchParams.get("to")).toBe("2026-09-15");
    const f = decodeClickFilters(u.searchParams.get("cf"));
    expect(f.device?.values).toEqual(["mobile"]);
    expect(f.quality).toBe("valid");
    expect(f.params).toEqual([{ key: "utm_source", op: "eq", values: ["zalo"] }]);
  });

  it("drill-down tuần bị kẹp trong kỳ", () => {
    const q = buildQuery({ g: ["week"], f: {} }, { from: "2026-09-03", to: "2026-10-07" });
    const u = new URL(drillDownHref(q, [{ dim: "week", value: "2026-08-31" }]), "http://x");
    expect(u.searchParams.get("from")).toBe("2026-09-03");
    expect(u.searchParams.get("to")).toBe("2026-09-06");
  });
});
