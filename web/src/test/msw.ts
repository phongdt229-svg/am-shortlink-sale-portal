// MSW: mock portal-api qua BFF. Payload được kiểm kiểu bằng schema sinh từ OpenAPI → spec đổi là test lỗi biên dịch.
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { components } from "@/lib/api/schema";

type S = components["schemas"];

// Khớp mọi origin (jsdom mặc định http://localhost:3000).
export const bff = (path: string) => `*/api/bff${path}`;

export function json<T>(body: T, status = 200) {
  return HttpResponse.json(body as object, { status });
}

export function problem(status: number, code: string, detail: string, requestId = "req-test-1") {
  return HttpResponse.json({ type: "about:blank", status, title: "Lỗi", code, detail, request_id: requestId } satisfies S["Problem"], {
    status,
    headers: { "content-type": "application/problem+json" },
  });
}

export const server = setupServer();
export { http };
export type { S };
