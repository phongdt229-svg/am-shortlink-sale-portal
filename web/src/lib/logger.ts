import pino from "pino";

// Log JSON (stdout → K8s thu thập → Kafka/Kibana). Không ghi token, cookie, mật khẩu.
export const logger = pino({
  level: process.env.LOG_LEVEL ?? "info",
  base: { service: process.env.OTEL_SERVICE_NAME ?? "am-shortlink-portal-web", env: process.env.APP_ENV ?? "local" },
  timestamp: pino.stdTimeFunctions.isoTime,
  redact: {
    paths: ["password", "token", "access_token", "refresh_token", "authorization", "cookie", "*.password", "*.authorization", "*.cookie"],
    censor: "[REDACTED]",
  },
  formatters: { level: (label) => ({ level: label }) },
});
