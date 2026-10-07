import { defineConfig, devices } from "@playwright/test";

// E2E chạy trên stack đang chạy (docker compose hoặc local): portal-web + portal-api + MongoDB đã seed (cmd/seed).
//   E2E_BASE_URL=http://localhost:3000 pnpm test:e2e
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["junit", { outputFile: "e2e-results.xml" }], ["list"]] : "list",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    locale: "vi-VN",
    timezoneId: "Asia/Ho_Chi_Minh",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } }],
});
