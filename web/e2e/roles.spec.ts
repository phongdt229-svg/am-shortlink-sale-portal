import { expect, test } from "@playwright/test";
import { login, nav } from "./helpers";

test.describe("Đăng nhập & phiên", () => {
  test("chưa đăng nhập → /login, đăng nhập xong quay lại đúng trang đang xem", async ({ page }) => {
    await page.goto("/campaigns?preset=7d");
    await expect(page).toHaveURL(/\/login\?next=%2Fcampaigns%3Fpreset%3D7d/);
    await page.getByLabel("Tên đăng nhập").fill("partner_a");
    await page.getByLabel("Mật khẩu").fill("partner123");
    await page.getByRole("button", { name: "Đăng nhập" }).click();
    await expect(page).toHaveURL(/\/campaigns\?preset=7d/);
    await expect(page.getByRole("heading", { name: "Báo cáo theo chiến dịch" })).toBeVisible();
  });

  test("sai mật khẩu báo lỗi kèm mã hỗ trợ; cookie phiên httpOnly, không lộ JWT", async ({ page, context }) => {
    await page.goto("/login");
    await page.getByLabel("Tên đăng nhập").fill("partner_a");
    await page.getByLabel("Mật khẩu").fill("sai-mat-khau");
    await page.getByRole("button", { name: "Đăng nhập" }).click();
    // (Next.js có sẵn 1 role=alert cho route announcer → lọc theo nội dung)
    const alert = page.getByRole("alert").filter({ hasText: "sai tên đăng nhập hoặc mật khẩu" });
    await expect(alert).toBeVisible();
    await expect(alert).toContainText("Mã hỗ trợ");

    await login(page, "user");
    const cookies = await context.cookies();
    const s = cookies.find((c) => c.name === "portal_session");
    expect(s?.httpOnly).toBe(true);
    expect(s?.sameSite).toBe("Lax");
    expect(await page.evaluate(() => document.cookie)).not.toContain("portal_session");
    expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toMatch(/eyJ/);
  });

  test("đăng xuất xoá phiên", async ({ page }) => {
    await login(page, "user");
    await page.getByRole("button", { name: "Đăng xuất" }).click();
    await expect(page).toHaveURL(/\/login/);
    await page.goto("/overview");
    await expect(page).toHaveURL(/\/login/);
  });
});

test.describe("Admin", () => {
  test("thấy toàn hệ thống + màn hình quản trị", async ({ page }) => {
    await login(page, "admin");
    await expect(page.getByText("Top tài khoản")).toBeVisible();
    await expect(nav(page).getByRole("link", { name: "Chất lượng traffic" })).toBeVisible();
    await expect(nav(page).getByRole("link", { name: "Quản lý tham số" })).toBeVisible();
    await nav(page).getByRole("link", { name: "Chất lượng traffic" }).click();
    await expect(page.getByText("Click bị gắn cờ gần nhất")).toBeVisible();
    await expect(page.locator("td.font-mono").first()).toHaveText(/^\d+\.\d+\.\d+\.\d+$/); // IP đầy đủ với admin
  });
});

test.describe("Đối tác (user)", () => {
  test("chỉ dữ liệu của mình; gọi tài khoản khác bị từ chối", async ({ page }) => {
    await login(page, "user");
    await expect(page.getByText("Tài khoản partner_a")).toBeVisible();
    await expect(nav(page).getByRole("link", { name: "Tài khoản", exact: true })).toHaveCount(0);
    await expect(nav(page).getByRole("link", { name: "Chất lượng traffic" })).toHaveCount(0);

    await page.goto("/accounts/partner_b");
    await expect(page.getByRole("alert").filter({ hasText: "Không tải được dữ liệu" })).toBeVisible();
    await expect(page.getByText("Tài khoản partner_b")).toHaveCount(0);
  });

  test("SĐT CTV và IP bị che; URL chi tiết CTV không chứa SĐT", async ({ page }) => {
    await login(page, "user", "/ctvs");
    const first = page.locator("tbody tr").first().getByRole("link");
    await expect(first).toHaveText(/\*{4}|^[0-9a-f]{12}$/);
    await first.click();
    await expect(page.getByRole("heading", { name: /^CTV / })).toBeVisible();
    expect(page.url()).not.toMatch(/\/ctvs\/0\d{9}/);

    await page.goto("/clicks?preset=7d");
    await expect(page.locator("tbody tr").first()).toBeVisible();
    for (const ip of await page.locator("td.font-mono").allTextContents()) expect(ip).toMatch(/x\.x$|^x/);
  });
});

test.describe("Viewer", () => {
  test("chỉ tài khoản được gán; không có màn hình admin; không xuất click thô", async ({ page }) => {
    await login(page, "viewer", "/accounts");
    const rows = page.locator("tbody tr");
    await expect(rows.first()).toBeVisible();
    for (const t of await rows.locator("td:first-child").allTextContents()) expect(["partner_a", "partner_b"]).toContain(t.trim());
    await expect(nav(page).getByRole("link", { name: "Chất lượng traffic" })).toHaveCount(0);

    // Trang có loading.tsx (stream) nên HTTP status đã là 200 trước khi notFound() — kiểm theo nội dung.
    await page.goto("/traffic-quality");
    await expect(page.getByText("Không tìm thấy trang")).toBeVisible();
    await expect(page.getByText("Click bị gắn cờ gần nhất")).toHaveCount(0);

    await page.goto("/clicks?preset=7d");
    await expect(page.getByRole("heading", { name: "Nhật ký click" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Xuất dữ liệu" })).toHaveCount(0);
  });
});

test.describe("Click Explorer", () => {
  test("nhóm 2 chiều có pivot; bấm ô → nhật ký click đúng tiêu chí", async ({ page }) => {
    const ex = Buffer.from(JSON.stringify({ g: ["device", "source_group"], f: { quality: "valid" } })).toString("base64url");
    await login(page, "viewer", `/clicks/explore?preset=30d&ex=${ex}`);
    await expect(page.getByText("Bảng pivot")).toBeVisible();
    await page.locator("table").first().locator("tbody a").first().click();
    await expect(page).toHaveURL(/\/clicks\?.*cf=/);
    await expect(page.getByText("Tiêu chí chi tiết:")).toBeVisible();
    await expect(page.locator("tbody tr").first()).toBeVisible();
  });
});
