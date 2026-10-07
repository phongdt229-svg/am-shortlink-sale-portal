# Portal — chia phase triển khai code

> Nguồn: [`PLAN_PORTAL_REPORT.md`](PLAN_PORTAL_REPORT.md) §3 (kiến trúc, cấu trúc repo) và §8 (lộ trình). P0 (thiết kế) và P5 (UAT / go-live) không phải việc code nên không nằm ở đây.
> Kiến trúc (P-D10): **`portal-web` (Next.js) ──BFF──► `portal-api` (Go) ──► MongoDB** (đọc trực tiếp dữ liệu Service). Portal **không** gọi API của Service, không còn mock Service v4.
> Trước khi Service ghi dữ liệu thật trên dev (mốc M-3), `portal-api` chạy trên **MongoDB dev do `cmd/seed` sinh** đúng hợp đồng schema Service (`migrations/` của Service). Đổi sang dữ liệu Service: chỉ đổi `MONGODB_URI`.

| Phase | Backend `api/` (Go) | Frontend `web/` (Next.js) | Màn hình / file chính | Trạng thái |
|-------|---------------------|---------------------------|-----------------------|-----------|
| **P1. Nền tảng** | `go.mod`, Makefile, `.golangci.yml`; `config` (envconfig + `_FILE`); `platform/mongodb` (readPref secondary, `maxTimeMS`), `logger` (slog + Kafka async), `telemetry` (OTel); `httpapi` (chi, middleware, problem+json, `/healthz` `/readyz`); `auth` (login bcrypt, JWT, refresh xoay vòng, khoá 5 lần sai); `scope`, `mask`; `cmd/migrate` (collection + index `portal_*`); `cmd/seed`; `GET /v1/me`, `GET /v1/filters/*` | Next.js 15 + TS strict + Tailwind + shadcn/ui; layout + sidebar; `/login`, đăng xuất; `lib/session` (cookie httpOnly JWE, 8h, gia hạn khi hoạt động); `middleware.ts` (chặn chưa đăng nhập, refresh JWT); BFF `/api/bff/*` (allow-list, kiểm Origin cho POST); client sinh từ OpenAPI; bộ lọc chung ↔ URL (nuqs); định dạng số/ngày VN; header bảo mật; OTel + pino | `openapi/portal-api.yaml`, `api/internal/{config,platform,httpapi,auth,scope,mask,store}`, `api/cmd/*`, `web/src/{middleware.ts,lib/**,app/api/**}`, `deploy/docker-compose.yml`, Dockerfile ×2, `.gitlab-ci.yml` | ✅ |
| **P2. Tổng quan + Tài khoản + Chiến dịch** | `report` R1–R3 đọc `stats_*_daily/monthly`, so kỳ trước, đơn vị ngày/tuần/tháng; test tích hợp số liệu | R1, R2 (danh sách, chi tiết), R3 (danh sách, chi tiết, so sánh); KPI card, biểu đồ, bảng phân trang server | `/overview`, `/accounts`, `/accounts/[username]`, `/campaigns`, `/campaigns/[code]`, `/campaigns/compare` | ✅ |
| **P3. CTV + Link + Nhật ký click** | R4 (xếp hạng, tra cứu CTV qua index `ctv_id`, chưa định danh), R5 (chi tiết, top / link chết), R6 (`clicks` phân trang con trỏ, IP che theo vai trò) | R4, R5 (+ QR), R6 | `/ctvs/**`, `/links/**`, `/clicks` | ✅ |
| **P3b. Click Explorer + Tham số URL** | `explorer`: `POST /v1/reports/clicks/query` dựng aggregation pipeline (≤ 3 chiều, loại trừ, `maxTimeMS`, huỷ theo context), facets từ `click_facets`; `params` đọc `link_params`, `stats_param_*`; `savedreport` | Bộ lọc đầy đủ + facets + loại trừ, group by 1–3 chiều, chỉ số, so kỳ trước, biểu đồ tự chọn, pivot, drill-down → click thô, tab "Lượt click" khoá bộ lọc trong chi tiết tài khoản / chiến dịch / CTV / link, lưu & chia sẻ bộ lọc; báo cáo theo tham số | `/clicks/explore`, `/params` | ✅ |
| **P4. Chất lượng traffic + Export + Quản lý tham số** | R7 (admin); `export` worker (CSV/XLSX, `portal_exports`, TTL 7 ngày); `param_registry` đọc/ghi (admin) + `audit` | R7, R8 (tạo job, danh sách, tải file, hết hạn), quản lý tham số (bật/tắt, tiến độ backfill) | `/traffic-quality`, `/exports`, `/admin/param-registry` | ✅ |
| **P4b. Kiểm thử** | `testing` + `testify`; `testcontainers-go` MongoDB + seed: số liệu từng endpoint, tổng nhóm Explorer = thẻ tổng = R2, user không đọc được tài khoản khác, user Mongo `portal_api` không ghi được `am_shortlink`; contract (`oapi-codegen` diff rỗng) | Vitest: định dạng, bộ lọc ↔ URL, BFF allow-list; MSW mock từ OpenAPI; Playwright: kịch bản theo vai trò trên `docker compose` | `api/**/*_test.go`, `api/testdata/`, `web/src/**/*.test.ts`, `web/e2e/` | ✅ |

Tài khoản seed (chỉ dev, do `cmd/seed` tạo trong `users`): `admin / admin123` (admin), `viewer / viewer123` (viewer, xem `partner_a`, `partner_b`), `partner_a / partner123` (user).

## Ghi chú triển khai (khác / bổ sung so với plan)

| Hạng mục | Thực tế | Lý do / việc cần làm |
|----------|---------|----------------------|
| OpenAPI | `openapi: 3.0.3` (plan ghi 3.1) | `oapi-codegen` v2 chưa hỗ trợ đủ 3.1 |
| Go | 1.26 (plan ghi 1.23+) | Thư viện mới nhất (mongo-driver, otel, excelize) yêu cầu |
| Cấu trúc `api/internal` | Explorer, tham số URL, R7, quản lý tham số nằm trong package `report` (`explorer.go`, `params.go`, `quality.go`); `savedreport`, `export`, `audit`, `cache` là package riêng | Dùng chung bộ dựng phạm vi / cache / che dữ liệu của `report`, tránh vòng import |
| Use-case trả DTO | `report` trả thẳng kiểu sinh từ OpenAPI | Bỏ một lớp mapping trùng lặp; handler vẫn mỏng |
| Explorer | Bước 1 gom (chiều + link) trong Mongo, bước 2 trong Go: link có click phân biệt, nhóm theo `param.<key>`, top N + "khác". Không có tiêu chí chi tiết và chỉ nhóm theo tài khoản / chiến dịch / CTV / link / thời gian → đọc `stats_link_daily` (≤ 12 tháng); ngược lại đọc `clicks` (≤ 3 tháng); giới hạn 300.000 nhóm trung gian | Đúng chiến lược §5.5 R6b; test đối chiếu stats = clicks |
| Export | Worker chạy trong `portal-api` (tắt bằng `EXPORT_WORKER=false` để tách deployment); file trên volume dùng chung (PVC RWX); job lưu ảnh chụp vai trò người tạo | Đủ cho tải hiện tại; tách worker khi cần |
| Che dữ liệu | Ngoài SĐT / IP, `portal-api` che cả `long_url`: `utm_extra_ctv` (trừ admin) và tham số PII (mọi vai trò) | Phát hiện khi QA: long URL chứa SĐT CTV / SĐT khách |
| Cache | Redis riêng của Portal (§3.2 `PLAN_PORTAL_REPORT.md`) | Theo yêu cầu bổ sung |
| Local | MongoDB / Redis / mongo-express chạy trong Docker (`deploy/docker-compose.yml`, cổng 27018 / 6380 / 8081) | Không dùng chung hạ tầng của dự án khác |

**Hợp đồng schema cần chốt với Service** (seed đang giả định — sai khác thì sửa seed + store):
1. Trường `date` của `stats_*_daily` = 00:00Z của **ngày lịch Việt Nam** (date-only); `month` = ngày 1.
2. Khoá map `by_*` (vd `by_referer`) thay `.` bằng `．` (U+FF0E); key rỗng = `(direct)`.
3. `by_*`, `clicks`, `unique_clicks`, `suspicious_clicks` chỉ tính click **không phải bot**; `active_links` = link có ≥ 1 click không bot.
4. Thêm index `stats_link_daily (prefix, date)` và `(date)` — lọc theo prefix / toàn hệ thống ở mức link (không có: ~0,9s).
5. `links.clicks` (tổng click toàn thời gian) dùng cho tra cứu CTV; `click_facets {owner, field, value, clicks, last_date}`; `param_values.clicks_total`.
6. `param_registry.backfill {status, progress, from, error}` — Portal ghi `status: pending, requested_at` khi bật theo dõi.
