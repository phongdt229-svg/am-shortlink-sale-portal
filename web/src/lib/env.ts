import { z } from "zod";

// Biến môi trường phía server. KHÔNG có thông tin kết nối MongoDB — portal-web chỉ gọi portal-api.
// Chỉ NEXT_PUBLIC_* (không bí mật) được lộ ra trình duyệt.
const schema = z.object({
  PORTAL_API_URL: z.url().default("http://localhost:8080"),
  PORTAL_SESSION_SECRET: z.string().min(32, "PORTAL_SESSION_SECRET phải dài ≥ 32 ký tự"),
  // Origin công khai của Portal (kiểm CSRF cho request ghi). Rỗng = lấy theo Host của request.
  PORTAL_PUBLIC_ORIGIN: z.string().optional(),
  SESSION_IDLE_HOURS: z.coerce.number().positive().default(8),
  COOKIE_SECURE: z
    .enum(["true", "false"])
    .optional()
    .transform((v) => (v === undefined ? process.env.NODE_ENV === "production" : v === "true")),
});

export type ServerEnv = z.infer<typeof schema>;

let cached: ServerEnv | undefined;

export function serverEnv(): ServerEnv {
  if (cached) return cached;
  const parsed = schema.safeParse(process.env);
  if (!parsed.success) {
    // Chỉ báo TÊN biến, không in giá trị.
    const names = parsed.error.issues.map((i) => i.path.join(".")).join(", ");
    throw new Error(`Cấu hình portal-web không hợp lệ: ${names}`);
  }
  cached = parsed.data;
  return cached;
}
