# portal-api — build từ thư mục gốc repo:  docker build -f api/Dockerfile .
FROM golang:1.26-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY api/go.mod api/go.sum ./api/
RUN cd api && go mod download
COPY openapi ./openapi
COPY api ./api
ARG VERSION=dev
RUN cd api && go build -ldflags "-s -w -X am-shortlink-portal/api/internal/platform/telemetry.Version=${VERSION}" -o /out/portal-api ./cmd/portal-api \
 && go build -ldflags "-s -w" -o /out/migrate ./cmd/migrate \
 && go build -ldflags "-s -w" -o /out/seed ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /zoneinfo.zip
ENV ZONEINFO=/zoneinfo.zip HTTP_ADDR=:8080
COPY --from=build /out/ /app/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/portal-api"]
