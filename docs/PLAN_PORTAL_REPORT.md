# Project 2 — `am-shortlink-portal` (Portal báo cáo: frontend Next.js + backend Go đọc trực tiếp MongoDB)

> **Trạng thái:** Đề xuất (draft) · **Ngày lập:** 2026-10-07 · **Cập nhật:** 2026-10-07 — chốt kiến trúc: Portal có **backend Go riêng (`portal-api`) kết nối trực tiếp MongoDB**, frontend **Next.js**; không còn phụ thuộc API v4 của Service (P-D10)
> **Thuộc chương trình:** [`REWRITE_PLAN.md`](REWRITE_PLAN.md) (tổng quan 2 project) · Project 1: [`PLAN_SERVICE_API.md`](PLAN_SERVICE_API.md)
> **Phạm vi project này:** frontend Next.js (`portal-web`) + backend Go (`portal-api`, kết nối trực tiếp MongoDB) để xem báo cáo shortlink theo **tài khoản, chiến dịch, CTV, link**; export; phân quyền xem dữ liệu
> **Checklist nghiệm thu:** [`REWRITE_CHECKLIST.md`](REWRITE_CHECKLIST.md) (các mục gắn nhãn **Portal**)
> **Môi trường & observability:** [`ENV_OBSERVABILITY_PLAN.md`](ENV_OBSERVABILITY_PLAN.md)

---

## 1. Bối cảnh & mục tiêu

### Hiện trạng
- Báo cáo đang nằm rải rác: API `/api/v2/report*` (đối tác tự gọi), 2 trang HTML tĩnh `/report-chart`, `/report-users` (bảo vệ bằng mật khẩu chung `REPORT_PAGE_PASSWORD`), admin Blade cũ đã bị tắt phần lớn.
- Nghiệp vụ (PO, quản lý kinh doanh, người đối soát CTV) không có công cụ tự xem / lọc / xuất số liệu; mọi yêu cầu số liệu phải nhờ dev chạy query.
- Tra cứu CTV theo SĐT mất ~15s.

### Mục tiêu
| # | Mục tiêu | Đo bằng |
|---|----------|---------|
| PG1 | Người dùng tự xem báo cáo theo tài khoản / chiến dịch / CTV / link | Đủ màn hình §4, UAT đạt |
| PG2 | Nhanh | Trang báo cáo hiển thị < 2s (p95), tra cứu CTV < 1s |
| PG3 | Đúng quyền | Người dùng chỉ thấy dữ liệu trong phạm vi vai trò (§5); `portal-api` kiểm quyền cả khi gọi API trực tiếp (bỏ qua giao diện) |
| PG4 | Thay thế hoàn toàn `/report-chart`, `/report-users` | Tắt 2 trang cũ sau go-live |
| PG5 | Số liệu đúng định nghĩa, một nơi tính | Mọi con số do `portal-api` (Go) tính theo định nghĩa chỉ số `PLAN_SERVICE_API.md` §5.2 từ dữ liệu MongoDB; frontend Next.js không tự tính lại; khớp báo cáo cũ trên cùng bộ lọc |

### Ngoài phạm vi (giai đoạn 1)
- Quản trị link / user / API key / prefix / cache (đưa vào **giai đoạn 2**, §9).
- Tạo link trên Portal.
- BI tuỳ biến (kéo thả cột, tự tạo báo cáo).

---

## 2. Người dùng & vai trò

| Vai trò | Ai | Thấy gì |
|---------|----|---------|
| `admin` | Team vận hành / PO | Toàn hệ thống: tổng quan, mọi tài khoản, chiến dịch, CTV; nhật ký click IP đầy đủ |
| `user` | Đối tác / tài khoản tạo link | Chỉ dữ liệu `owner = chính mình` (chiến dịch, CTV, link của mình); IP bị che |
| `viewer` | Quản lý kinh doanh | Chỉ các tài khoản được gán; không export dữ liệu thô nếu không được cấp |

Quyền **thực thi ở backend Portal (`portal-api`, Go)**: mọi truy vấn MongoDB đều bị ép thêm điều kiện phạm vi tài khoản theo người dùng trước khi chạy; frontend Next.js chỉ ẩn/hiện giao diện cho phù hợp.

---

## 3. Kiến trúc

```
Trình duyệt ──HTTPS──► portal-web (Next.js, K8s) — FRONTEND
                         ├─ App Router: Server Components render báo cáo
                         ├─ BFF route handlers (/api/bff/*) ── gắn JWT người dùng ──┐
                         ├─ Session: cookie httpOnly, SameSite=Lax (không lưu token   │
                         │  ở localStorage)                                           │
                         └─ OTel (instrumentation.ts)                                 ▼
                                                      portal-api (Go, K8s) — BACKEND
                                                        ├─ chi router /v1/*: auth, me, filters, reports,
                                                        │  explorer, params, saved-reports, exports
                                                        ├─ phân quyền theo vai trò + che SĐT/IP + tính chỉ số
                                                        ├─ worker export (job nền)
                                                        ├─ go-redis v9 ──► Redis RIÊNG của Portal (cache báo cáo, rate limit)
                                                        └─ mongo-go-driver v2 ──► MongoDB (kết nối trực tiếp)
                                                              ├─ am_shortlink         (chỉ đọc)
                                                              └─ am_shortlink_report  (đọc; ghi collection của Portal)

am-shortlink-service (Project 1, Go) ──ghi──► MongoDB: links, users, clicks, stats_*, link_params, stats_param_* …

OTel (portal-web + portal-api) ──► otel-collector.fpt.net ; log JSON ──► Kafka [pfx]am-shortlink-portal-logs
```

Nguyên tắc:
1. **Backend Portal (Go) kết nối trực tiếp MongoDB** — không đi qua API của Service. Dữ liệu nghiệp vụ và thống kê do Service (Project 1) ghi; `portal-api` **chỉ đọc** chúng và chỉ **ghi** vào collection thuộc Portal (§3.1). Không kết nối Redis / Kafka **nghiệp vụ** của Service; Portal có **Redis riêng** chỉ để cache (§3.2).
2. **Frontend Next.js không kết nối DB**, không giữ thông tin kết nối DB; mọi dữ liệu lấy từ `portal-api` qua BFF.
3. **Một nơi tính số liệu**: `portal-api` tính mọi chỉ số theo định nghĩa `PLAN_SERVICE_API.md` §5.2 (đọc `stats_*` cho báo cáo tổng hợp, `clicks` cho click thô / Click Explorer); frontend chỉ hiển thị, định dạng (PG5).
4. **Bắt buộc đăng nhập, BFF giữ bí mật**: mọi trang cần đăng nhập; JWT + refresh token do `portal-api` cấp chỉ nằm phía server (BFF), trình duyệt chỉ có cookie phiên httpOnly. Portal **không** dùng API key `key` của đối tác.
5. **Trạng thái bộ lọc nằm trên URL** → chia sẻ được link báo cáo, back/forward đúng.
6. **Hợp đồng `portal-web` ↔ `portal-api` bằng OpenAPI 3.1** nằm trong repo Portal (`openapi/portal-api.yaml`); client TypeScript sinh bằng `openapi-typescript` → đổi API là lỗi compile ngay.
7. **Hợp đồng với Service bằng schema MongoDB** (tên collection, trường, index, định nghĩa chỉ số) — thay đổi schema phải review chung (§7).

### Stack

| Lớp | Lựa chọn |
|-----|----------|
| **Frontend** (`web/`) | Next.js 15 (App Router) + TypeScript strict, Node 22 |
| UI | shadcn/ui + Tailwind CSS, TanStack Table (bảng lớn, sort, phân trang server) |
| Dữ liệu (frontend) | TanStack Query (cache, refetch), `openapi-fetch` (client sinh từ spec `portal-api`) |
| Biểu đồ | Recharts (line, bar, pie, heatmap tự dựng) |
| Form / lọc | React Hook Form + Zod; `nuqs` đồng bộ bộ lọc ↔ URL |
| Ngày giờ | `date-fns` + `date-fns-tz` (Asia/Ho_Chi_Minh) |
| Phiên (frontend) | Cookie httpOnly mã hoá (JWE) do BFF quản lý; đăng nhập qua `portal-api` (P-D1) |
| **Backend** (`api/`) | Go 1.26, `chi`, **`mongo-go-driver v2`**, `go-redis v9`, `slog`, `golang-jwt/jwt v5`, `golang.org/x/crypto/bcrypt`, `envconfig`, `oapi-codegen` (strict server + validator kin-openapi từ OpenAPI), `excelize` (XLSX) |
| Cache | **Redis riêng của Portal** (`go-redis v9`): cache kết quả báo cáo + rate limit đăng nhập dùng chung giữa replica (§3.2) |
| DB | MongoDB (cùng cụm với Service) — đọc ưu tiên **secondary / node analytics** (`readPreference=secondaryPreferred`), `maxTimeMS` cho mọi truy vấn |
| File export | Object storage nội bộ (S3-compatible) hoặc GridFS trong `am_shortlink_report`, TTL 7 ngày |
| Test | Frontend: Vitest + Testing Library, Playwright (e2e), MSW mock từ OpenAPI · Backend: `testing` + `testify`, `testcontainers-go` (MongoDB) với dữ liệu seed |
| Observability | Frontend: `@vercel/otel`, log JSON (pino) · Backend: OpenTelemetry SDK Go (HTTP + Mongo), `slog` JSON → stdout + Kafka |
| CI/CD | GitLab CI template gitflow ISC, 2 image Harbor (`portal-web`, `portal-api`), K8s — giống Service |

### Cấu trúc repo

Monorepo `am-shortlink-portal` gồm 2 ứng dụng build / deploy độc lập (2 image) và **một** file OpenAPI dùng chung.

```
am-shortlink-portal/
├── api/                                        # BACKEND — Go 1.26 (portal-api)
│   ├── cmd/
│   │   ├── portal-api/main.go                  # HTTP server + worker export (cờ --worker để chạy tách)
│   │   ├── migrate/main.go                     # tạo collection + index của Portal (K8s Job trước deploy, idempotent)
│   │   └── seed/main.go                        # chỉ dev: sinh am_shortlink + am_shortlink_report giả lập đúng schema Service
│   ├── internal/
│   │   ├── config/                             # envconfig + <TÊN>_FILE (Vault), validate khi khởi động
│   │   ├── platform/
│   │   │   ├── mongodb/                        # client v2, readPref secondaryPreferred, maxTimeMS mặc định, otelmongo
│   │   │   ├── redis/                          # client go-redis v9 (+ redisotel), REDIS_ADDR rỗng = tắt
│   │   │   ├── logger/                         # slog JSON → stdout + Kafka async (che SĐT/IP/token)
│   │   │   └── telemetry/                      # OTel traces + metrics (OTLP/HTTP)
│   │   ├── httpapi/
│   │   │   ├── gen/                            # ⚙ oapi-codegen sinh từ openapi/ (không sửa tay)
│   │   │   ├── handler/                        # implement interface sinh ra; mỏng: parse → gọi use-case → map DTO
│   │   │   ├── middleware/                     # request-id, recover, otel, auth JWT, rate limit, access log
│   │   │   ├── problem/                        # application/problem+json + request_id
│   │   │   └── router.go                       # chi: /v1/*, /healthz, /readyz, /metrics
│   │   ├── cache/                              # cache báo cáo trên Redis: khoá theo phạm vi đã áp quyền, fail-open, singleflight
│   │   ├── domain/                             # kiểu nghiệp vụ thuần (User, Role, Filter, Period, Metric…), không phụ thuộc Mongo
│   │   ├── auth/                               # đăng nhập bcrypt, JWT (golang-jwt v5), refresh token xoay vòng, khoá sau 5 lần sai
│   │   ├── scope/                              # vai trò → điều kiện phạm vi (owner ∈ …) ép vào MỌI truy vấn; 403 khi vượt phạm vi
│   │   ├── mask/                               # che SĐT CTV, IP theo vai trò trước khi trả
│   │   ├── report/                             # use-case R1–R7: overview, accounts, campaigns, ctvs, links, clicks
│   │   ├── explorer/                           # Click Explorer R6b: dựng aggregation pipeline (≤ 3 chiều), facets, so kỳ trước
│   │   ├── params/                             # báo cáo tham số URL + param_registry (admin)
│   │   ├── savedreport/                        # báo cáo / bộ lọc đã lưu & chia sẻ
│   │   ├── export/                             # job R8: hàng đợi trong portal_exports, ghi CSV/XLSX, TTL 7 ngày
│   │   ├── audit/                              # portal_audit_log
│   │   └── store/                              # repository MongoDB — CHỈ nơi này import driver
│   │       ├── users.go  links.go  campaigns.go  clicks.go
│   │       ├── stats.go  ctvs.go  params.go
│   │       └── portal.go                       # portal_* (refresh token, login attempts, saved reports, exports)
│   ├── migrations/                             # 0001_portal_collections.json, 0002_indexes.json … (có version)
│   ├── testdata/                               # fixture seed cho test tích hợp + snapshot số liệu kỳ vọng
│   ├── .golangci.yml
│   ├── go.mod
│   ├── Makefile                                # gen, lint, test, test-integration, run, seed
│   └── Dockerfile                              # multi-stage → distroless, non-root
├── web/                                        # FRONTEND — Next.js 15 (App Router) + TypeScript strict, Node 22
│   ├── src/
│   │   ├── app/
│   │   │   ├── (auth)/login/page.tsx
│   │   │   ├── (portal)/                       # layout có sidebar + thanh bộ lọc chung; yêu cầu phiên
│   │   │   │   ├── overview/                            # R1
│   │   │   │   ├── accounts/  [username]/               # R2
│   │   │   │   ├── campaigns/ [code]/ compare/          # R3
│   │   │   │   ├── ctvs/ [ctvId]/ lookup/ unidentified/ # R4
│   │   │   │   ├── links/ [code]/ top/                  # R5
│   │   │   │   ├── clicks/ explore/                     # R6, R6b
│   │   │   │   ├── params/                              # §4.1 c2
│   │   │   │   ├── traffic-quality/                     # R7 (admin)
│   │   │   │   ├── exports/                             # R8
│   │   │   │   └── admin/param-registry/                # §4.1 c3 (admin)
│   │   │   ├── api/
│   │   │   │   ├── auth/{login,logout}/route.ts         # gọi portal-api, ghi / xoá cookie phiên
│   │   │   │   └── bff/[...path]/route.ts               # proxy allow-list → portal-api, gắn JWT, kiểm Origin cho POST
│   │   │   └── layout.tsx  error.tsx  not-found.tsx
│   │   ├── middleware.ts                       # chặn trang chưa đăng nhập → /login?next=…; refresh JWT gần hết hạn
│   │   ├── features/                           # theo màn hình: overview, accounts, campaigns, ctvs, links, clicks, explorer, params, exports
│   │   │   └── <feature>/ components/ queries.ts columns.tsx
│   │   ├── components/
│   │   │   ├── ui/                             # shadcn/ui
│   │   │   └── filters/ charts/ data-table/ kpi-card/ export-button/
│   │   ├── lib/
│   │   │   ├── api/                            # ⚙ schema.d.ts sinh bằng openapi-typescript + client openapi-fetch
│   │   │   ├── session/                        # cookie httpOnly mã hoá JWE (jose), hết hạn 8h, gia hạn khi hoạt động
│   │   │   ├── filters/                        # nuqs parsers: bộ lọc ↔ URL
│   │   │   ├── format/                         # số, %, ngày dd/MM/yyyy, Asia/Ho_Chi_Minh
│   │   │   └── logger.ts                       # pino JSON
│   │   └── instrumentation.ts                  # @vercel/otel, truyền traceparent sang portal-api
│   ├── e2e/                                    # Playwright theo vai trò admin / user / viewer
│   ├── mocks/                                  # MSW handler sinh từ OpenAPI (test frontend không cần backend)
│   ├── next.config.ts                          # output standalone, header bảo mật (CSP, HSTS…)
│   ├── package.json  tsconfig.json  eslint.config.mjs
│   └── Dockerfile                              # Next standalone, non-root
├── openapi/
│   └── portal-api.yaml                         # OpenAPI 3.0.3 — NGUỒN SỰ THẬT: sinh api/internal/httpapi/gen và web/src/lib/api
├── deploy/
│   ├── helm/portal-api/  helm/portal-web/      # values-{dev,staging,production}.yaml
│   └── docker-compose.yml                      # local: MongoDB (replica set 1 node), portal-api, portal-web, OTel Collector, Jaeger
├── .gitlab-ci.yml                              # job theo thư mục thay đổi: api/** → image portal-api, web/** → image portal-web
└── README.md
```

**Quy tắc phân lớp**

| Lớp | Được gọi | Không được |
|-----|----------|-----------|
| `web/` (Next.js) | `portal-api` qua BFF / server component (`lib/api`) | Kết nối MongoDB; tự tính chỉ số; để JWT ra trình duyệt |
| `httpapi/handler` | `auth`, `report`, `explorer`, `params`, `savedreport`, `export` | Import driver Mongo; dựng truy vấn |
| Use-case (`report`, `explorer`, …) | `scope`, `mask`, `store` | Trả dữ liệu chưa qua `scope` / `mask` |
| `store` | MongoDB (`am_shortlink` chỉ đọc; `am_shortlink_report` theo §3.1) | Ghi `am_shortlink`; chạy truy vấn thiếu điều kiện phạm vi |

**Luồng sinh code:** sửa `openapi/portal-api.yaml` → `make gen` (Go: `oapi-codegen` strict-server · TS: `openapi-typescript`) → CI fail nếu `git diff` sau khi generate khác rỗng hoặc `oasdiff breaking` báo phá hợp đồng.

**Chạy local:** `docker compose -f deploy/docker-compose.yml up -d` → `make -C api migrate seed` → `pnpm --dir web dev`. Không cần hạ tầng chung.

### 3.1 Truy cập MongoDB (`portal-api`)

| Database | Collection | Quyền `portal-api` | Dùng cho |
|----------|------------|--------------------|----------|
| `am_shortlink` | `users` | đọc | Đăng nhập (bcrypt `password_hash`), vai trò, `portal_access`, tài khoản được gán cho `viewer` |
| `am_shortlink` | `links`, `campaigns`, `prefixes` | đọc | Danh mục, chi tiết link, short URL đúng prefix |
| `am_shortlink` | `clicks` (time-series) | đọc | R6 click thô, R6b truy vấn tự do, R7 chất lượng traffic (≤ 3 tháng) |
| `am_shortlink_report` | `stats_*_daily`, `stats_*_monthly`, `click_facets`, `link_params`, `stats_param_*`, `param_values`, `ctvs` | đọc | R1–R5, facets, báo cáo tham số URL (≤ 12 tháng) |
| `am_shortlink_report` | `param_registry` | đọc + ghi (chỉ admin) | Bật/tắt theo dõi tham số — Service đọc thay đổi để chạy backfill (§7) |
| `am_shortlink_report` | `portal_saved_reports`, `portal_exports`, `portal_refresh_tokens`, `portal_login_attempts`, `portal_audit_log` | đọc + ghi | Báo cáo đã lưu, job export, phiên, khoá đăng nhập, nhật ký thao tác |

- User MongoDB riêng cho `portal-api` với **role tối thiểu** như bảng trên (không có quyền ghi `am_shortlink`).
- Mọi truy vấn: ép điều kiện phạm vi tài khoản **trước** (dùng index `(meta.owner, ts)`, `(owner, date)`), `maxTimeMS` 10s, huỷ được khi người dùng huỷ; giới hạn khoảng ngày (tổng hợp ≤ 12 tháng, click thô ≤ 3 tháng).
- Đọc từ secondary / node analytics để không ảnh hưởng redirect và API của Service.

### 3.2 Cache báo cáo (Redis)

| Hạng mục | Thiết kế |
|----------|----------|
| Redis | Instance **riêng của Portal** (không dùng Redis nghiệp vụ của Service), `REDIS_ADDR` / `REDIS_PASSWORD` (Vault) / `REDIS_DB`; trống `REDIS_ADDR` = tắt cache |
| Cache gì | Kết quả R1–R3 (và các báo cáo tổng hợp thêm ở phase sau) dưới dạng JSON |
| Khoá | `am-shortlink-portal:report:<báo cáo>:<sha256(phạm vi đã áp quyền + bộ lọc + cờ admin + tham số)>` — theo **phạm vi**, không theo username → viewer cùng phạm vi dùng chung; không bao giờ trả dữ liệu ngoài phạm vi |
| TTL | `REPORT_CACHE_TTL` = 60s khi kỳ có hôm nay (giữ hành vi `SETTING_REPORT_CACHE_TTL` cũ, số liệu trễ ≤ 1 phút); `REPORT_CACHE_TTL_PAST` = 15 phút khi kỳ đã kết thúc |
| An toàn | Fail-open: Redis lỗi / treo → bỏ qua sau ≤ 150ms, tính trực tiếp từ MongoDB; không retry. `SET key val EX ttl` một lệnh. Singleflight chống dồn truy vấn trùng trong pod |
| Không cache | Dữ liệu cá nhân hoá theo phiên (`/me`, refresh token), click thô có phân trang con trỏ (R6), job export |
| Rate limit đăng nhập | Đếm theo IP / phút trên Redis (đúng khi nhiều replica); Redis lỗi → giới hạn trong bộ nhớ pod |
| Metrics | `portal_report_cache_lookups_total{report,result=hit|miss}`, `portal_report_cache_errors_total{op}` |

---

## 4. Màn hình báo cáo

Bộ lọc dùng chung (thanh trên cùng, lưu trên URL): **khoảng ngày** (hôm nay / 7 ngày / 30 ngày / tháng này / tháng trước / tuỳ chọn ≤ 12 tháng), **tài khoản**, **chiến dịch**, **CTV**, **prefix** (`sale` / `lm`), **so với kỳ trước** (bật/tắt), **đơn vị thời gian** (ngày / tuần / tháng).

Cột **API** là endpoint của `portal-api` (Go), dưới tiền tố `/v1`. Cột **Nguồn MongoDB** là collection mà `portal-api` đọc trực tiếp (§3.1).

| # | Màn hình | Nội dung | API `portal-api` | Nguồn MongoDB | Vai trò |
|---|----------|----------|------------------|---------------|---------|
| R1 | **Tổng quan** | Thẻ KPI (link, link mới, click, unique, bot, % so kỳ trước); line chart click & link mới; top 10 tài khoản / chiến dịch / CTV / link; phân rã thiết bị, nguồn, quốc gia; heatmap giờ × thứ | `GET /reports/overview` | `stats_system_daily`, `stats_owner_daily`, `stats_link_daily` | admin (user: tổng quan của mình) |
| R2 | **Theo tài khoản** — danh sách | Bảng: username, total/new/active links, clicks, unique, bot, % so kỳ trước; sort, tìm, phân trang; click dòng → chi tiết | `GET /reports/accounts` | `stats_owner_daily` | admin, viewer |
| R2 | **Theo tài khoản** — chi tiết | KPI, timeline, bảng chiến dịch của tài khoản, top CTV, top link, phân rã; **tab "Lượt click"** (Click Explorer khoá theo tài khoản, §4.1) | `GET /reports/accounts/{username}` | `stats_owner_daily`, `stats_campaign_daily`, `stats_ctv_daily`, `stats_link_daily` | admin, viewer, chính chủ |
| R3 | **Theo chiến dịch** — danh sách | Bảng: code, tên, new/active links, clicks, unique, số CTV, ngày bắt đầu / click cuối | `GET /reports/campaigns` | `stats_campaign_daily`, `campaigns` | tất cả (theo phạm vi) |
| R3 | **Theo chiến dịch** — chi tiết | KPI, timeline, **bảng xếp hạng CTV trong chiến dịch**, top link, phân rã thiết bị / nguồn / quốc gia / giờ | `GET /reports/campaigns/{code}` | `stats_campaign_daily`, `stats_ctv_daily`, `stats_link_daily` | tất cả |
| R3 | **So sánh chiến dịch** | Chọn 2–5 chiến dịch; timeline chồng + bảng chỉ số cạnh nhau | `GET /reports/campaigns/compare` | `stats_campaign_daily` | tất cả |
| R4 | **Theo CTV** — bảng xếp hạng | ctv_id (SĐT che), tên (nếu có danh mục), new/active links, clicks, unique, **click nghi vấn**, hạng; nút export đối soát | `GET /reports/ctvs` | `stats_ctv_daily`, `ctvs` | tất cả |
| R4 | **Theo CTV** — chi tiết | KPI, timeline, danh sách link của CTV + click, chiến dịch tham gia, cảnh báo bất thường | `GET /reports/ctvs/{ctv_id}` | `stats_ctv_daily`, `stats_link_daily`, `links` | tất cả |
| R4 | **Tra cứu CTV** | Ô nhập SĐT / hash → kết quả tức thì (thay `report-total-detail` ~15s) | `GET /reports/ctvs/lookup` | `links` (index `ctv_id`), `stats_link_daily` | tất cả |
| R4 | **CTV chưa định danh** | Số link / click không có `ctv_identifier`, danh sách link để nghiệp vụ xử lý | `GET /reports/ctvs/unidentified` | `links`, `stats_link_daily` | admin, chính chủ |
| R5 | **Chi tiết link** | Short URL (đúng prefix), long URL, QR, KPI, timeline, phân rã, 100 click gần nhất | `GET /reports/links/{code}`, `GET /links/{code}/qrcode` | `links`, `stats_link_daily`, `clicks` | theo quyền |
| R5 | **Top link / link "chết"** | Top theo click, tăng trưởng; link không có click N ngày | `GET /reports/links/top` | `stats_link_daily`, `links` | tất cả |
| R6 | **Nhật ký click** | Bảng click thô (thời gian, link, CTV, thiết bị / OS / trình duyệt, nguồn, quốc gia / tỉnh, IP che/đủ theo vai trò); lọc đủ tiêu chí §4.1a; phân trang con trỏ; ≤ 3 tháng | `GET /reports/clicks` | `clicks` | tất cả (IP đủ: admin) |
| R6b | **Phân tích lượt click (Click Explorer)** | Lọc nhiều tiêu chí + nhóm theo 1–3 chiều + biểu đồ + pivot + drill-down; là tab "Lượt click" trong chi tiết tài khoản / chiến dịch / CTV / link — chi tiết §4.1 | `POST /reports/clicks/query`, `GET /reports/clicks/facets`, `/saved-reports` | `stats_*` (chiều tổng hợp sẵn), `clicks` (kết hợp tự do), `click_facets`, `portal_saved_reports` | tất cả (theo phạm vi) |
| R7 | **Chất lượng traffic** | Top IP, tỉ lệ click lặp theo CTV / chiến dịch, danh sách click nghi vấn kèm lý do, tỉ lệ bot theo nguồn | `GET /reports/traffic-quality` | `clicks`, `stats_ctv_daily`, `stats_campaign_daily` | admin |
| R8 | **Xuất dữ liệu** | Nút "Xuất CSV/XLSX" trên R2–R6 → tạo job; trang danh sách job (trạng thái, tải file, hết hạn 7 ngày) | `POST /exports`, `GET /exports`, `GET /exports/{id}`, `GET /exports/{id}/download` | như màn hình nguồn; job lưu `portal_exports` | theo quyền |

### 4.1 Báo cáo lượt click chi tiết (Click Explorer) — trọng tâm theo tài khoản

Màn hình mới `/(reports)/clicks/explore`, đồng thời là **tab "Lượt click"** trong *Chi tiết tài khoản* (R2), *Chi tiết chiến dịch* (R3), *Chi tiết CTV* (R4), *Chi tiết link* (R5) — mở từ tab nào thì bộ lọc tương ứng được điền sẵn và khoá (vd đang ở tài khoản `partner_a` → `account = partner_a`).

#### a. Tiêu chí lọc

| Nhóm | Tiêu chí | Kiểu chọn | Ghi chú |
|------|----------|-----------|---------|
| **Tài khoản** | Tài khoản (1 hoặc nhiều), vai trò tài khoản, `users.prefix` | multi-select có tìm kiếm | `user` chỉ thấy chính mình; `viewer` chỉ các tài khoản được gán |
| **Thời gian click** | Khoảng ngày; **khung giờ** (vd 08:00–12:00); **thứ trong tuần**; preset "giờ hành chính / ngoài giờ" | date range + time range + checkbox | Múi giờ Asia/Ho_Chi_Minh |
| **Link** | Mã ngắn (1 hoặc nhiều, dán danh sách); prefix truy cập (`/sale`, `/lm`); prefix cấp khi tạo; link custom / sinh tự động; tạo qua API v1 / v2 / v3; trạng thái link (active / disabled / deleted); **ngày tạo link** | multi-select, textarea | Phân biệt "click trong kỳ" với "link tạo trong kỳ" |
| **Đích đến (long URL)** | Domain đích (`fpt.vn`, `shop.fpt.vn`...); path chứa | multi-select (facets) + text | |
| **Tham số URL** | **Bất kỳ tham số nào đang được theo dõi**: `utm_source`, `utm_medium`, `utm_campaign`, `utm_content`, `utm_term`, `utm_extra_ctv`, và tham số admin bật thêm (vd `ref`, `promo`, `locationid`); điều kiện: bằng / thuộc danh sách / chứa / có tham số / không có tham số | Thêm dòng điều kiện động: chọn key → chọn value (facets) | Dữ liệu từ `link_params` / `stats_param_*` (Service §5.4b); tham số PII không hiển thị |
| **Chiến dịch** | Mã chiến dịch (nhiều), tên chiến dịch, có / không có chiến dịch | multi-select | |
| **CTV** | SĐT / hash (nhiều, dán danh sách), có / chưa định danh | textarea | SĐT nhập dạng nào cũng chuẩn hoá về `0xxxxxxxxx` |
| **Thiết bị** | Loại (`mobile`, `tablet`, `desktop`, `bot`, `unknown`), **hệ điều hành** (Android, iOS, Windows, macOS...), **trình duyệt** (Chrome, Safari, Zalo in-app, Facebook in-app, Cốc Cốc...) | multi-select | Tách từ user-agent lúc ghi click |
| **Nguồn truy cập** | Nhóm nguồn (Zalo, Facebook, Google, TikTok, trực tiếp, khác), `referer_host` cụ thể | multi-select | Nhóm nguồn theo bảng quy đổi cấu hình được |
| **Vị trí** | Quốc gia; **tỉnh/thành** (nếu GeoIP City bật) | multi-select | |
| **Chất lượng** | Chỉ click hợp lệ / chỉ bot / chỉ nghi vấn / tất cả; **lần đầu hay lặp lại** (unique trong ngày) | radio | Mặc định: click hợp lệ |
| **IP** *(admin)* | IP / dải CIDR | text | User thường không lọc / không xem IP đầy đủ |

Bộ lọc kết hợp **AND** giữa các tiêu chí, **OR** trong cùng một tiêu chí; có nút "loại trừ" (NOT) cho từng tiêu chí (vd loại `bot`, loại domain test).

#### b. Nhóm theo (group by) & chỉ số

- Chọn **1–3 chiều** nhóm: tài khoản, chiến dịch, CTV, link, prefix, domain đích, **bất kỳ tham số URL đang theo dõi** (`utm_source`, `utm_medium`, `utm_campaign`, `ref`...), thiết bị, hệ điều hành, trình duyệt, nhóm nguồn, referer host, quốc gia, tỉnh/thành, ngày / tuần / tháng, giờ trong ngày, thứ.
- Chỉ số chọn hiển thị: `clicks`, `unique_clicks`, `bot_clicks`, `suspicious_clicks`, `active_links`, `clicks / link`, **% trên tổng**, **% so kỳ trước**, click đầu tiên / cuối cùng.
- Ví dụ câu hỏi trả lời được:
  - *Tài khoản `partner_a` tháng 9: click theo ngày × thiết bị.*
  - *Chiến dịch X: CTV nào có nhiều click từ Zalo in-app nhất, khung 19h–22h.*
  - *Tất cả tài khoản: click theo `utm_source` × tuần, chỉ click hợp lệ.*
  - *Link tạo qua API v3 (prefix `/lm`) nhưng được mở qua `/sale` bao nhiêu lượt.*

#### c. Hiển thị

| Khối | Nội dung |
|------|----------|
| Thẻ tổng | Tổng click, unique, bot, nghi vấn, số link có click, số tài khoản / CTV liên quan, % so kỳ trước |
| Biểu đồ | Tự chọn theo nhóm: line (theo thời gian), stacked bar (thời gian × chiều phụ), bar ngang (top N), heatmap (giờ × thứ), pie (tỉ trọng ≤ 6 phần) |
| Bảng pivot | Dòng = chiều 1, cột = chiều 2 (nếu có), ô = chỉ số; tổng dòng / cột; sort theo cột; top N + "khác" |
| Drill-down | Bấm một ô / cột → mở danh sách click thô (R6) với bộ lọc tương ứng |
| Danh sách click thô | Thời gian, mã link, long URL (rút gọn), tài khoản, chiến dịch, CTV (che), thiết bị / OS / trình duyệt, nguồn, quốc gia / tỉnh, IP (che theo vai trò), cờ bot / nghi vấn / lặp lại |

#### c2. Báo cáo theo tham số URL (màn hình `/(reports)/params`)

| Khối | Nội dung |
|------|----------|
| Chọn tham số | Danh sách tham số đang theo dõi (`utm_source`, `utm_medium`, `utm_campaign`...) kèm số giá trị, số link, tổng click |
| Bảng giá trị | Mỗi giá trị của tham số: clicks, unique, số link, số tài khoản / chiến dịch dùng, % tổng, % so kỳ trước; sort, tìm |
| Biểu đồ | Timeline top 5 giá trị; stacked bar giá trị × thiết bị / nhóm nguồn |
| Kết hợp | Ma trận 2 tham số (vd `utm_source` × `utm_medium`), hoặc tham số × tài khoản / chiến dịch / CTV |
| Drill-down | Giá trị → danh sách link mang giá trị đó → click thô |
| Cảnh báo dữ liệu | Hiện nhãn khi tham số `high_cardinality` (chỉ top N) hoặc mới bật theo dõi (số liệu chỉ có từ ngày backfill) |

#### c3. Quản lý tham số theo dõi *(admin)*

Màn hình `/(admin)/param-registry`: danh sách tham số **đã xuất hiện** trên link (kèm số link, số giá trị, ví dụ giá trị), bật / tắt theo dõi, đặt nhãn hiển thị, đánh dấu PII (`hash` / `drop`), ngưỡng số giá trị; khi bật mới → `portal-api` ghi `param_registry` (kèm nhật ký `portal_audit_log`), job backfill của Service đọc thay đổi, tính `stats_param_daily` và cập nhật tiến độ vào `param_registry.backfill` → Portal hiển thị tiến độ. Đây là màn hình quản trị duy nhất có ở giai đoạn 1 vì báo cáo phụ thuộc vào nó.

#### d. Tiện ích

- **Lưu bộ lọc** thành "báo cáo của tôi" (tên, mô tả), chia sẻ cho người khác qua link (người nhận chỉ thấy phần dữ liệu thuộc quyền mình).
- **So sánh 2 kỳ** hoặc **2 tập lọc** (vd tài khoản A vs B) trên cùng biểu đồ.
- **Export** đúng bảng đang xem (tổng hợp) hoặc click thô (job nền R8).
- Giới hạn: tổng hợp ≤ 12 tháng; click thô / lọc theo IP, khung giờ chi tiết ≤ 3 tháng; khi vượt → gợi ý thu hẹp.
- Thời gian phản hồi mục tiêu: tổng hợp < 2s, có chỉ báo "đang tính" và huỷ được truy vấn dài.

Tiêu chuẩn giao diện chung:
- Số định dạng Việt Nam (`1.234.567`), % 1 chữ số thập phân, mũi tên tăng/giảm so kỳ trước.
- Trạng thái **đang tải** (skeleton), **không có dữ liệu**, **lỗi** (kèm `request_id` để báo hỗ trợ) trên mọi khối.
- Ghi chú "Số liệu hôm nay có thể trễ ≤ 1 phút" ở các màn hình có ngày hiện tại.
- Bảng > 1.000 dòng: phân trang server, không tải hết về trình duyệt.
- Responsive: dùng được trên laptop 1366px và tablet; điện thoại chỉ cần xem KPI / tra cứu CTV.
- Tooltip giải thích từng chỉ số theo định nghĩa `PLAN_SERVICE_API.md` §5.2 (`portal-api` tính đúng định nghĩa này).

---

## 5. Xác thực & phân quyền

| Hạng mục | Thiết kế |
|----------|----------|
| Đăng nhập | ✅ **Bắt buộc** — mọi trang (trừ `/login`) chặn bởi middleware Next.js, chưa đăng nhập → chuyển `/login` (giữ lại URL đang xem). Cách đăng nhập (P-D1): tài khoản `users` (username + mật khẩu) qua `POST /v1/auth/login` của `portal-api` — `portal-api` đọc `users` trong MongoDB, so bcrypt `password_hash`; hoặc SSO OIDC nội bộ nếu ISC có. **Không** đăng nhập bằng API key |
| Tài khoản | Người dùng Portal là bản ghi `users` có `portal_access = true` (admin cấp); đối tác chỉ vào được khi P-D5 cho phép. `viewer` có danh sách tài khoản được xem (`viewer_accounts`) |
| Khoá đăng nhập | Sai 5 lần → khoá 15 phút; đếm trong `portal_login_attempts` (Portal không ghi vào `users` của Service) |
| Phiên (trình duyệt ↔ `portal-web`) | Cookie httpOnly + Secure + SameSite=Lax (nội dung mã hoá), hết hạn 8 giờ, gia hạn khi hoạt động; đăng xuất xoá phiên |
| BFF → `portal-api` | JWT người dùng ngắn hạn (15 phút, `portal-api` ký) + refresh token (lưu băm trong `portal_refresh_tokens`, thu hồi được); BFF giữ phía server, tự refresh. Trình duyệt không bao giờ thấy JWT. API của đối tác vẫn dùng `key` như cũ ở Service — Portal không dùng `key` |
| Vai trò | `GET /v1/me` → `admin` / `user` / `viewer` + danh sách tài khoản được xem; `portal-api` đọc lại `users` mỗi lần cấp / refresh token để khoá / đổi quyền có hiệu lực nhanh |
| Phạm vi dữ liệu | Package `scope` của `portal-api` ép điều kiện vào **mọi** truy vấn Mongo: admin = toàn bộ; user = `owner = chính mình`; viewer = `owner ∈ viewer_accounts`. Tham số `owner` / `account` ngoài phạm vi → 403 |
| Bảo vệ | CSRF cho request ghi qua BFF (kiểm `Origin`), rate limit đăng nhập, header bảo mật (CSP, HSTS, X-Frame-Options); `portal-api` chỉ nhận request từ mạng nội bộ cụm (NetworkPolicy), không public ra Internet |
| Dữ liệu nhạy cảm | SĐT CTV che `090****052`, IP che theo vai trò (`113.161.x.x`, đủ với admin) — **`portal-api` che trước khi trả**, frontend không nhận dữ liệu đủ rồi tự che. Tham số PII (`pii = hash/drop`) không bao giờ trả bản rõ |

---

## 6. Môi trường & vận hành

| | dev | staging | production |
|---|-----|---------|------------|
| Domain | `portal-dev…` | `portal-stag…` | `portal…` (P-D3) |
| `portal-api` → MongoDB | Mongo dev — DB tạo bằng `cmd/seed` (dữ liệu giả lập đúng schema) hoặc dữ liệu Service dev | Mongo staging (bản sao prod đã ẩn danh), đọc secondary | Mongo production, đọc secondary / node analytics |
| Tài khoản Mongo | User `portal_api` quyền theo §3.1 | như dev | như dev |
| Redis cache (§3.2) | `am-shortlink-portal-redis-dev` (local: docker compose) | `am-shortlink-portal-redis-staging` | `am-shortlink-portal-redis` (xin cấp) |
| OTel | `OTEL_SERVICE_NAMESPACE=am-shortlink`, `OTEL_SERVICE_NAME=am-shortlink-portal-web` / `am-shortlink-portal-api`, collector `http://otel-collector.fpt.net`, sampler `always` | như dev | sampler `ratio` 0.05 |
| Log Kafka | `dev-am-shortlink-portal-logs` | `stag-am-shortlink-portal-logs` | `am-shortlink-portal-logs` (P-D4: hay dùng chung topic `am-shortlink-logs`) |
| Secret (Vault) | `portal-web`: `PORTAL_SESSION_SECRET` (mã hoá cookie phiên), OIDC client secret (nếu SSO) · `portal-api`: `MONGODB_URI` (user `portal_api`), `PORTAL_JWT_SIGNING_KEY`, `PII_HASH_SALT` (tra cứu tham số `pii = hash`), `REDIS_PASSWORD` — nhánh `isc-project/<PROJECT_FULLNAME>/<env>/am-shortlink-portal` |

- `portal-web` chỉ có `PORTAL_API_URL` (địa chỉ nội bộ `portal-api`) và secret phiên; **không** có thông tin kết nối MongoDB.
- Chỉ biến `NEXT_PUBLIC_*` không bí mật ra trình duyệt (vd tên môi trường, URL Portal).
- Trace nối liền trình duyệt → `portal-web` → `portal-api` → MongoDB (header `traceparent`, instrument driver Mongo), xem `ENV_OBSERVABILITY_PLAN.md` §4.
- Metrics: thời gian render trang, lỗi BFF, thời gian truy vấn Mongo theo báo cáo, số truy vấn vượt `maxTimeMS`, tỉ lệ lỗi API theo màn hình, Web Vitals (LCP, INP).

---

## 7. Phụ thuộc vào Service (Project 1)

Portal không gọi API của Service; phụ thuộc là **dữ liệu trong MongoDB** do Service ghi.

| Portal cần | Service cung cấp | Mốc Service | Portal làm gì trong lúc chờ |
|------------|------------------|-------------|------------------------------|
| **Hợp đồng schema MongoDB** (collection, trường, index, kiểu) cho `users`, `links`, `campaigns`, `clicks`, `stats_*`, `link_params`, `stats_param_*`, `click_facets`, `ctvs`, `param_registry` | `PLAN_SERVICE_API.md` §4, §5.4, §5.4b + file `migrations/` trong repo Service | **Cuối Phase 1** | — (điều kiện bắt đầu backend P1 của Portal) |
| Định nghĩa chỉ số | §5.2 (D17, D18 chốt) | Phase 0 | Dùng bản đề xuất, đánh dấu tooltip "tạm" |
| Trường cho Portal trong `users` | `portal_access`, `role` (`admin`/`user`/`viewer`), `viewer_accounts` (D22) | Phase 1–2 | Seed dev có sẵn các trường này |
| `stats_*` được ghi đầy đủ (consumer + backfill) trên dev | Phase 4a | Phase 4 | `portal-api` chạy trên DB seed (`cmd/seed`) |
| `clicks` có đủ trường làm giàu (`os`, `browser`, `source_group`, `access_prefix`, `hour`, `weekday`, `is_*`) | Phase 2–4 (§5.5 R6b) | Phase 4 | Seed dev |
| Job backfill tham số đọc `param_registry` (Portal ghi) và cập nhật tiến độ | §5.4b | Phase 4 | Hiển thị trạng thái "chờ backfill" |
| Dữ liệu thật trên staging | Migrate + backfill `stats_*` | Phase 6 | UAT trên dữ liệu seed lớn |

Quy ước: schema MongoDB có **version**; Service đổi schema (đổi tên / xoá trường, đổi kiểu, đổi index Portal dùng) phải báo Portal trong MR và giữ tương thích ít nhất 1 release. `portal-api` có test tích hợp chạy trên file `migrations/` của Service để phát hiện lệch schema sớm.

---

## 8. Lộ trình Portal

Giả định: 1–2 dev frontend, 1 dev backend Go, 1 designer bán thời gian, QA dùng chung với Service.

| Phase | Nội dung | Thời lượng | Tiêu chí xong |
|-------|----------|-----------|----------------|
| **P0. Thiết kế** | Phỏng vấn người dùng (PO, kinh doanh, đối soát CTV); wireframe R1–R8; chốt P-D1…P-D10; review hợp đồng schema MongoDB cùng Service; viết OpenAPI `portal-api` | 1–2 tuần (song song Service Phase 0–1) | Wireframe được duyệt, schema + OpenAPI `portal-api` thống nhất |
| **P1. Nền tảng** | Repo (`web/` + `api/`), CI/CD 3 môi trường cho 2 image · **Backend**: `portal-api` (chi, kết nối Mongo, cấu hình, đăng nhập / JWT / refresh, `scope`, `mask`, problem+json, OTel), `cmd/seed` tạo DB dev · **Frontend**: layout, đăng nhập, phiên, BFF, client sinh từ OpenAPI, bộ lọc chung ↔ URL, OTel + log | 2–3 tuần | Đăng nhập được trên dev, `portal-api` đọc DB seed, trace Portal → Mongo thấy trên Collector |
| **P2. Tổng quan + Tài khoản + Chiến dịch** | Backend: truy vấn `stats_*` cho R1–R3 · Frontend: R1, R2 (danh sách, chi tiết), R3 (danh sách, chi tiết, so sánh) | 3 tuần | Màn hình chạy trên DB dev, số khớp định nghĩa §5.2 |
| **P3. CTV + Link + Nhật ký click** | Backend: R4–R6 (tra cứu CTV qua index `ctv_id`, click thô phân trang con trỏ) · Frontend: R4 (xếp hạng, chi tiết, tra cứu, chưa định danh), R5, R6 | 3 tuần | Tra cứu CTV < 1s trên dữ liệu staging |
| **P3b. Click Explorer + Tham số URL** | Backend: `POST /reports/clicks/query` (aggregation pipeline, ≤ 3 chiều, `maxTimeMS`, huỷ), facets, báo cáo tham số · Frontend: §4.1 bộ lọc đầy đủ, group by, biểu đồ, pivot, drill-down, tab trong chi tiết, lưu & chia sẻ bộ lọc | 3 tuần | Trả lời được 4 câu hỏi mẫu §4.1b trên staging trong < 2s |
| **P4. Chất lượng traffic + Export + Quản lý tham số** | Backend: R7, worker export (CSV/XLSX, TTL 7 ngày), `param_registry` + nhật ký · Frontend: R7, R8, §4.1 c3 | 2 tuần | Export khớp số liệu màn hình |
| **P5. UAT & go-live** | UAT trên staging dữ liệu thật; sửa lỗi; hiệu năng truy vấn Mongo (index, explain); bảo mật (pentest nhẹ, kiểm quyền user Mongo); tài liệu hướng dẫn | 2 tuần | UAT ký duyệt; tắt `/report-chart`, `/report-users` |

**Tổng:** ~16–18 tuần, bắt đầu song song Service; go-live khi Service đã ghi `stats_*` đầy đủ (Phase 4) và có dữ liệu thật trên staging (Service Phase 6).

---

## 9. Giai đoạn 2 (sau go-live báo cáo)

| Tính năng | Ghi chú |
|-----------|---------|
| Quản trị link | Tìm, sửa long URL, bật/tắt, xoá/khôi phục (cập nhật cache redirect) |
| Quản trị tài khoản & API key | Tạo/khoá user, vai trò, quota, sinh lại key (chỉ hiện 4 ký tự cuối), `prefix`, độ dài mã, hạn link |
| Quản lý prefix, domain, template | Thêm prefix mới không cần deploy |
| Công cụ cache | Kiểm / xoá cache theo mã (admin) |
| Danh mục CTV | Nhập / sửa tên, SĐT, tài khoản quản lý (D19) |
| Báo cáo định kỳ (R9) | Đăng ký nhận email / webhook tóm tắt ngày / tuần |
| Tạo link | Tạo đơn + hàng loạt (CSV ≤ 1000 dòng, báo lỗi từng dòng) |

Các tính năng **ghi dữ liệu nghiệp vụ** (link, user, API key, prefix, cache redirect) phải đi qua Service (Service sở hữu quy tắc nghiệp vụ, cache, sự kiện Kafka) — `portal-api` gọi API quản trị của Service hoặc Service bổ sung endpoint khi bắt đầu giai đoạn 2; `portal-api` **không** ghi thẳng vào `am_shortlink`.

---

## 10. Kiểm thử

| Loại | Cách làm |
|------|----------|
| Unit (frontend) | Định dạng số / ngày, chuyển đổi bộ lọc ↔ URL, component bảng / biểu đồ |
| Unit (backend) | Tính chỉ số §5.2, `scope` (điều kiện phạm vi theo vai trò), `mask` (SĐT / IP), chuẩn hoá SĐT, kiểm tham số đầu vào |
| Tích hợp (backend) | `testcontainers-go` MongoDB + dữ liệu seed: mỗi endpoint trả đúng số; tổng nhóm Explorer = thẻ tổng = R2; tổng CTV + "chưa định danh" = tổng click chiến dịch; user không đọc được tài khoản khác |
| Contract | OpenAPI `portal-api`: server sinh bằng `oapi-codegen`, client frontend sinh bằng `openapi-typescript`; frontend test trên mock MSW sinh từ cùng spec · Schema Mongo: test chạy trên `migrations/` của Service |
| E2E (Playwright) | Kịch bản theo vai trò: admin xem tổng quan → drill-down tài khoản → chiến dịch → CTV → link → click; user không truy cập được tài khoản khác (kể cả sửa URL / gọi thẳng API); export và tải file |
| Đối chiếu số liệu | Mỗi màn hình so con số `portal-api` với truy vấn đối chiếu trực tiếp trên Mongo và với báo cáo cũ (PHP) trên cùng bộ lọc |
| Hiệu năng | Lighthouse / Web Vitals; `explain()` mọi truy vấn chính (phải dùng index); trang báo cáo p95 < 2s trên staging dữ liệu thật |
| Bảo mật | Kiểm phân quyền ở `portal-api`, CSRF, cookie, header bảo mật; user Mongo `portal_api` không ghi được `am_shortlink`; không lộ thông tin DB / JWT ra trình duyệt |
| Khả dụng | Tiếng Việt đầy đủ, bàn phím, độ tương phản; thử với người dùng thật ở UAT |

---

## 11. Quyết định cần chốt (Portal)

| # | Câu hỏi | Đề xuất |
|---|---------|---------|
| P-D1 | Portal **bắt buộc đăng nhập** (đã chốt). Cách đăng nhập: SSO OIDC nội bộ hay tài khoản `users` | Tài khoản `users` (username + mật khẩu, đổi mật khẩu lần đầu, khoá sau 5 lần sai); thêm SSO nếu ISC có |
| P-D2 | BFF xác thực với backend Portal thế nào | ✅ JWT người dùng ngắn hạn (15 phút) + refresh token, do `portal-api` cấp khi đăng nhập; BFF giữ phía server |
| P-D3 | Domain Portal mỗi môi trường | Hỏi Ops (vd `sl-portal-stag.fpt.vn`, `sl-portal.fpt.vn`) |
| P-D4 | Topic log Portal riêng hay chung `am-shortlink-logs` | Chung topic, phân biệt bằng trường `service` |
| P-D5 | Đối tác (`user`) có được vào Portal không, hay chỉ nội bộ | Giai đoạn 1 chỉ nội bộ (admin, viewer); mở cho đối tác sau UAT |
| P-D6 | Có cần đa ngôn ngữ | Chỉ tiếng Việt giai đoạn 1 |
| P-D7 | Bảng quy đổi "nhóm nguồn" (Zalo, Facebook, Google, TikTok...) và danh sách trình duyệt in-app cần nhận diện | Nghiệp vụ duyệt danh sách; cấu hình ở Service (áp lúc ghi click) |
| P-D8 | Cho phép chia sẻ "báo cáo đã lưu" ra ngoài người tạo không | Có, trong nội bộ; người nhận vẫn bị giới hạn theo quyền của họ |
| P-D9 | Ai được bật / tắt theo dõi tham số URL | Chỉ `admin`; mọi thay đổi ghi `portal_audit_log` (ai, khi nào, giá trị cũ / mới) |
| P-D10 | Portal lấy dữ liệu qua API Service hay đọc trực tiếp MongoDB | ✅ **Đã chốt: backend Portal bằng Go (`portal-api`) kết nối trực tiếp MongoDB** (chỉ đọc dữ liệu Service, ghi collection của Portal); frontend Next.js chỉ gọi `portal-api` qua BFF |
| P-D11 | Một repo (`web/` + `api/`) hay hai repo cho Portal | Một repo `am-shortlink-portal`, 2 image / 2 deployment độc lập |
| P-D12 | Lưu file export ở đâu | Object storage nội bộ nếu Ops có; nếu không, GridFS trong `am_shortlink_report` (TTL 7 ngày) |

---

## 12. Rủi ro

| Rủi ro | Mức | Giảm thiểu |
|--------|-----|-----------|
| Service đổi schema MongoDB làm hỏng Portal | Cao | Hợp đồng schema có version (§7), review chung, test tích hợp `portal-api` trên `migrations/` của Service |
| Truy vấn báo cáo nặng ảnh hưởng redirect / API | Cao | Đọc secondary / node analytics, `maxTimeMS`, giới hạn khoảng ngày, ưu tiên `stats_*`, `explain()` trong review; theo dõi slow query |
| `stats_*` chưa đủ / trễ (consumer Service) | Trung bình | Làm trên DB seed; hiển thị "số liệu đến <thời điểm cập nhật cuối>"; đối chiếu đêm |
| Số liệu Portal lệch báo cáo cũ | Cao | Đối chiếu từng màn hình; định nghĩa chỉ số thống nhất §5.2 Service; tooltip giải thích |
| Rò dữ liệu giữa tài khoản | Cao | Package `scope` bắt buộc trên mọi truy vấn (review + test tích hợp truy cập chéo); e2e thử sửa URL / gọi thẳng API |
| Lộ quyền DB | Trung bình | User Mongo `portal_api` quyền tối thiểu; chỉ `portal-api` có `MONGODB_URI`; `portal-api` không public |
| Bảng / biểu đồ chậm với dữ liệu lớn | Trung bình | Phân trang server, giới hạn khoảng ngày, cache kết quả 60s ở `portal-api`, cache TanStack Query |

---

## 13. Vai trò

| Vai trò | Trách nhiệm |
|---------|-------------|
| Frontend (1–2) | `portal-web` Next.js, BFF, test e2e |
| Backend Go (1) | `portal-api`: kết nối MongoDB, truy vấn báo cáo, phân quyền, che dữ liệu, export, OpenAPI, test tích hợp |
| Designer (bán thời gian) | Wireframe, UI kit, duyệt giao diện |
| Backend Service | Ghi dữ liệu đúng hợp đồng schema MongoDB (`clicks`, `stats_*`, `link_params`…), backfill tham số (Project 1) |
| QA | E2E, đối chiếu số liệu, UAT |
| PO / nghiệp vụ | Chốt màn hình, chỉ số, P-D1/P-D5; nghiệm thu |

---

## 14. Việc tiếp theo

- [ ] Chốt P-D1 (SSO hay tài khoản `users`) và P-D5 (đối tượng dùng Portal)
- [ ] Phỏng vấn 3–5 người dùng báo cáo (PO, kinh doanh, đối soát CTV); thu danh sách câu hỏi số liệu hay gặp
- [ ] Wireframe R1–R8
- [ ] Review hợp đồng schema MongoDB với team Service; viết OpenAPI `portal-api`
- [ ] Xin user MongoDB `portal_api` (quyền §3.1) cho 3 môi trường
- [ ] Tạo repo `am-shortlink-portal` (`web/` + `api/`), xin domain + secret trong Vault cho 3 môi trường
