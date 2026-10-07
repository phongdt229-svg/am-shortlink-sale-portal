# am-shortlink-portal

Portal báo cáo shortlink: **`portal-web` (Next.js) ──BFF──► `portal-api` (Go) ──► MongoDB** (đọc trực tiếp dữ liệu do Service ghi) + Redis riêng cho cache.
Kế hoạch & tiến độ: [`docs/PLAN_PORTAL_REPORT.md`](docs/PLAN_PORTAL_REPORT.md) · [`docs/PORTAL_PHASES.md`](docs/PORTAL_PHASES.md).

```
api/       Go 1.26 — portal-api (chi, mongo-go-driver v2, go-redis v9, slog, OTel)
web/       Next.js 15 (App Router) — portal-web
openapi/   portal-api.yaml — hợp đồng duy nhất, sinh code Go + TypeScript
deploy/    docker-compose (local), Helm (portal-api, portal-web), OTel collector, role MongoDB
docs/      kế hoạch
```

## Chạy local

```bash
# 1. Hạ tầng trong Docker: MongoDB (27018), Redis (6380), mongo-express (http://localhost:8081)
docker compose -f deploy/docker-compose.yml up -d mongo redis mongo-express

# 2. portal-api — dữ liệu giả lập đúng schema Service + collection Portal
cd api && cp .env.example .env
make seed          # = go run ./cmd/seed -reset (từ chối xoá DB không do seed tạo)
make run           # http://localhost:8080

# 3. portal-web
cd web && cp .env.example .env.local && pnpm install
pnpm dev           # http://localhost:3000
```

Tài khoản dev (do seed tạo): `admin / admin123`, `viewer / viewer123` (xem `partner_a`, `partner_b`), `partner_a / partner123`.

Toàn bộ trong Docker: `docker compose -f deploy/docker-compose.yml up -d --build` rồi `docker compose -f deploy/docker-compose.yml run --rm seed`.

## Sửa API

1. Sửa `openapi/portal-api.yaml`
2. `make -C api gen` và `pnpm --dir web gen:api` (CI fail nếu code sinh lệch spec)
3. Implement handler (`api/internal/httpapi/handler`) → use-case (`internal/report`, …) → truy vấn (`internal/store` — nơi duy nhất dùng driver MongoDB)

## Kiểm thử

| Lệnh | Nội dung |
|------|----------|
| `go test ./...` (api) | Unit: auth, scope, mask, cache, config; guard tĩnh "không ghi `am_shortlink`" |
| `MONGODB_TEST_URI=mongodb://localhost:27018/?directConnection=true go test -tags=integration ./...` | Tích hợp trên DB seed: số liệu mọi báo cáo đối chiếu với đáp án tính từ click thô, tổng nhóm Explorer = thẻ tổng = R2, phạm vi theo vai trò, che SĐT/IP/PII, export, HTTP 401/403/400 |
| `pnpm test` (web) | Vitest: định dạng, bộ lọc ↔ URL, BFF allow-list / CSRF, phiên JWE, component với MSW |
| `pnpm test:e2e` (web) | Playwright theo vai trò trên stack đang chạy |
