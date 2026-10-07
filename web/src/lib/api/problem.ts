import type { components } from "./schema";

export type Problem = components["schemas"]["Problem"];

/** Lỗi API thống nhất cho UI: luôn có status + request_id (để người dùng báo hỗ trợ). */
export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly requestId?: string;
  readonly fields?: { field: string; message: string }[];

  constructor(p: Partial<Problem> & { status: number }) {
    super(p.detail || p.title || `HTTP ${p.status}`);
    this.name = "ApiError";
    this.status = p.status;
    this.code = p.code;
    this.requestId = p.request_id;
    this.fields = p.errors;
  }

  static from(error: unknown, response?: Response): ApiError {
    if (error instanceof ApiError) return error;
    const status = response?.status ?? 500;
    if (error && typeof error === "object" && "status" in error) {
      return new ApiError(error as Problem);
    }
    return new ApiError({
      status,
      title: "Lỗi kết nối",
      request_id: response?.headers.get("x-request-id") ?? undefined,
    });
  }
}

/** Ném ApiError nếu openapi-fetch trả lỗi; trả data nếu thành công. */
export function unwrap<T>(r: { data?: T; error?: unknown; response: Response }): T {
  if (r.error !== undefined || !r.response.ok) throw ApiError.from(r.error, r.response);
  return r.data as T;
}
