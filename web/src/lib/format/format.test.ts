import { describe, expect, it } from "vitest";
import { fmtDate, fmtDateTime, fmtDelta, fmtNumber, fmtPercent, todayVN } from "./index";

describe("format", () => {
  it("số kiểu Việt Nam", () => {
    expect(fmtNumber(1234567)).toBe("1.234.567");
    expect(fmtNumber(null)).toBe("–");
  });
  it("phần trăm 1 chữ số", () => {
    expect(fmtPercent(0.1234)).toBe("12,3%");
    expect(fmtPercent(Number.NaN)).toBe("–");
  });
  it("so kỳ trước", () => {
    expect(fmtDelta(0.25)).toEqual({ text: "+25,0%", trend: "up" });
    expect(fmtDelta(-0.1)).toEqual({ text: "-10,0%", trend: "down" });
    expect(fmtDelta(null)).toEqual({ text: "–", trend: "flat" });
  });
  it("ngày dd/MM/yyyy, giờ VN", () => {
    expect(fmtDate("2026-10-07")).toBe("07/10/2026");
    expect(fmtDateTime("2026-10-06T17:30:00Z")).toBe("07/10/2026 00:30");
    expect(todayVN(new Date("2026-10-06T18:00:00Z"))).toBe("2026-10-07");
  });
});
