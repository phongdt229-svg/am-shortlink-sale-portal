import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// "server-only" ném lỗi khi import ngoài Server Component — trong test thì bỏ qua.
vi.mock("server-only", () => ({}));
