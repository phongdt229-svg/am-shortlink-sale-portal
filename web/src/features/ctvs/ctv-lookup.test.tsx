import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "@/test/render";
import { bff, http, json, problem, server, type S } from "@/test/msw";
import { CTVLookup } from "./ctv-lookup";

vi.mock("next/link", () => ({ default: ({ href, children, ...p }: { href: string; children: React.ReactNode }) => <a href={href} {...p}>{children}</a> }));

beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

const result: S["CTVLookup"] = {
  query: "+84 901 234 052",
  normalized: "090****052",
  items: [
    {
      ctv_ref: "ref-abc",
      ctv_display: "090****052",
      name: "Nguyễn Văn An",
      owner: "partner_a",
      links: 12,
      clicks_total: 3456,
      first_link_at: "2026-08-01T03:00:00Z",
      last_link_at: "2026-10-01T03:00:00Z",
      sample_links: [],
    },
  ],
};

describe("Tra cứu CTV", () => {
  it("gửi SĐT người dùng nhập, hiển thị kết quả đã che từ portal-api", async () => {
    let asked = "";
    server.use(
      http.get(bff("/v1/reports/ctvs/lookup"), ({ request }) => {
        asked = new URL(request.url).searchParams.get("q") ?? "";
        return json(result);
      }),
    );
    renderWithProviders(<CTVLookup />);
    await userEvent.type(screen.getByLabelText("SĐT hoặc hash CTV"), "+84 901 234 052");
    await userEvent.click(screen.getByRole("button", { name: /Tra cứu/ }));

    expect(await screen.findByText("Nguyễn Văn An")).toBeInTheDocument();
    expect(asked).toBe("+84 901 234 052");
    expect(screen.getByText("3.456")).toBeInTheDocument();
    expect(screen.getAllByText("090****052").length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: "090****052" })).toHaveAttribute("href", "/ctvs/ref-abc?account=partner_a");
  });

  it("không tìm thấy → thông báo rỗng", async () => {
    server.use(http.get(bff("/v1/reports/ctvs/lookup"), () => json({ ...result, items: [] })));
    renderWithProviders(<CTVLookup />, { search: "?q=0999999999" });
    expect(await screen.findByText("Không tìm thấy CTV")).toBeInTheDocument();
  });

  it("lỗi API → hiển thị mã hỗ trợ request_id", async () => {
    server.use(http.get(bff("/v1/reports/ctvs/lookup"), () => problem(504, "query_timeout", "quá lâu", "req-xyz")));
    renderWithProviders(<CTVLookup />, { search: "?q=0901234052" });
    await waitFor(() => expect(screen.getByRole("alert")).toBeInTheDocument());
    expect(screen.getByText(/req-xyz/)).toBeInTheDocument();
    expect(screen.getByText(/thu hẹp khoảng ngày/)).toBeInTheDocument();
  });
});
