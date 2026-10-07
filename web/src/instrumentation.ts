import { registerOTel } from "@vercel/otel";

// OpenTelemetry phía server: span cho request + fetch tới portal-api (tự gắn traceparent).
// Endpoint lấy từ env (OTEL_EXPORTER_OTLP_TRACES_ENDPOINT ← OTEL_ENDPOINT_TRACES), không hard-code.
export function register() {
  if (process.env.OTEL_ENABLED !== "true") return;
  if (process.env.OTEL_ENDPOINT_TRACES && !process.env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT) {
    process.env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT = process.env.OTEL_ENDPOINT_TRACES;
  }
  if (process.env.OTEL_SAMPLER === "ratio") {
    process.env.OTEL_TRACES_SAMPLER = "parentbased_traceidratio";
    process.env.OTEL_TRACES_SAMPLER_ARG = process.env.OTEL_SAMPLER_RATIO ?? "0.05";
  }
  registerOTel({
    serviceName: process.env.OTEL_SERVICE_NAME ?? "am-shortlink-portal-web",
    attributes: {
      "service.namespace": process.env.OTEL_SERVICE_NAMESPACE ?? "am-shortlink",
      "deployment.environment": process.env.APP_ENV ?? "local",
    },
    instrumentationConfig: {
      fetch: { propagateContextUrls: [/.*/] }, // portal-api là dịch vụ nội bộ
    },
  });
}
