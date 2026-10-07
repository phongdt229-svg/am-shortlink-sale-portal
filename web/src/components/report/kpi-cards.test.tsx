import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { components } from "@/lib/api/schema";
import { KpiCards } from "./kpi-cards";

const kpis: components["schemas"]["Kpis"] = {
  period: { from: "2026-09-08", to: "2026-10-07" },
  current: { total_links: 2900, new_links: 685, active_links: 1954, clicks: 39108, unique_clicks: 35498, bot_clicks: 1709, suspicious_clicks: 76, ctr_per_link: 20.01 },
  previous: { total_links: 2200, new_links: 700, active_links: 1600, clicks: 39000, unique_clicks: 35000, bot_clicks: 1500, suspicious_clicks: 40, ctr_per_link: 24.4 },
  change: { clicks: 0.0028, new_links: -0.021, bot_clicks: 0.139, suspicious_clicks: 0.9, total_links: null },
};

describe("KpiCards", () => {
  it("định dạng số VN, % so kỳ trước; bot / nghi vấn tăng tô đỏ", () => {
    render(<KpiCards kpis={kpis} />);
    expect(screen.getByText("39.108")).toBeInTheDocument();
    expect(screen.getByText("20,0")).toBeInTheDocument();
    expect(screen.getByText("+0,3%")).toHaveClass("text-success", { exact: false });
    const susp = screen.getByText("+90,0%");
    expect(susp.className).toContain("text-destructive");
    expect(screen.getByText("-2,1%").className).toContain("text-destructive");
  });

  it("có tooltip định nghĩa chỉ số §5.2", () => {
    render(<KpiCards kpis={kpis} keys={["clicks"]} />);
    expect(screen.getAllByTitle(/không tính bot/).length).toBeGreaterThan(0);
  });
});
