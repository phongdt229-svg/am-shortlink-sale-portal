import { expect, type Page } from "@playwright/test";

export const USERS = {
  admin: { username: "admin", password: "admin123" },
  viewer: { username: "viewer", password: "viewer123" },
  user: { username: "partner_a", password: "partner123" },
} as const;

export async function login(page: Page, who: keyof typeof USERS, next = "/overview") {
  await page.goto(`/login?next=${encodeURIComponent(next)}`);
  await page.getByLabel("Tên đăng nhập").fill(USERS[who].username);
  await page.getByLabel("Mật khẩu").fill(USERS[who].password);
  await page.getByRole("button", { name: "Đăng nhập" }).click();
  await expect(page).toHaveURL(new RegExp(next.replace(/[?]/g, "\\?")));
}

export function nav(page: Page) {
  return page.getByRole("navigation", { name: "Điều hướng chính" }).first();
}
