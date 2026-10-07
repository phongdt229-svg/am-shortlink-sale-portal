// Nhãn trạng thái link — module thường (không "use client") để dùng được cả ở Server Component.
export const STATUS_LABEL: Record<string, { label: string; tone: "success" | "warning" | "danger" }> = {
  active: { label: "Hoạt động", tone: "success" },
  disabled: { label: "Tắt", tone: "warning" },
  deleted: { label: "Đã xoá", tone: "danger" },
};
