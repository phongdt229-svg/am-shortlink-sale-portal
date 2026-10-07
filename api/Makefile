# portal-api — lệnh phát triển. Biến môi trường đọc từ .env (copy từ .env.example).
include .env
export

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X am-shortlink-portal/api/internal/platform/telemetry.Version=$(VERSION)

.PHONY: gen lint test test-integration run migrate seed build check-gen

gen: ## sinh code Go từ ../openapi/portal-api.yaml
	go tool oapi-codegen -config oapi-codegen.yaml ../openapi/portal-api.yaml

check-gen: gen ## CI: fail nếu code sinh lệch spec
	git diff --exit-code -- internal/httpapi/gen

lint:
	golangci-lint run ./...

test:
	go test -race ./...

# Cần MongoDB: MONGODB_TEST_URI (vd local) hoặc Docker (testcontainers).
test-integration:
	go test -race -tags=integration ./...

run:
	go run ./cmd/portal-api

migrate:
	go run ./cmd/migrate

seed:
	go run ./cmd/seed -reset

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/portal-api ./cmd/portal-api
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/migrate ./cmd/migrate
