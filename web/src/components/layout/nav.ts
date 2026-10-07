import type { Role } from "@/lib/session/session";

export interface NavItem {
  href: string;
  label: string;
  icon: string; // tên icon lucide (map ở sidebar)
  roles?: Role[]; // không khai báo = mọi vai trò
  section: "report" | "tools" | "admin";
}

// Phân quyền hiển thị chỉ là UX — portal-api mới là nơi thực thi quyền.
export const NAV: NavItem[] = [
  { href: "/overview", label: "Tổng quan", icon: "layout-dashboard", section: "report" },
  { href: "/accounts", label: "Tài khoản", icon: "users", roles: ["admin", "viewer"], section: "report" },
  { href: "/campaigns", label: "Chiến dịch", icon: "megaphone", section: "report" },
  { href: "/ctvs", label: "CTV", icon: "user-check", section: "report" },
  { href: "/links/top", label: "Link", icon: "link", section: "report" },
  { href: "/clicks", label: "Nhật ký click", icon: "mouse-pointer-click", section: "report" },
  { href: "/clicks/explore", label: "Phân tích click", icon: "chart-no-axes-combined", section: "report" },
  { href: "/params", label: "Tham số URL", icon: "tags", section: "report" },
  { href: "/traffic-quality", label: "Chất lượng traffic", icon: "shield-alert", roles: ["admin"], section: "report" },
  { href: "/ctvs/lookup", label: "Tra cứu CTV", icon: "search", section: "tools" },
  { href: "/exports", label: "Xuất dữ liệu", icon: "download", section: "tools" },
  { href: "/admin/param-registry", label: "Quản lý tham số", icon: "settings-2", roles: ["admin"], section: "admin" },
];

export function navFor(role: Role): NavItem[] {
  return NAV.filter((n) => !n.roles || n.roles.includes(role));
}

/** Trang không cần thanh bộ lọc chung. */
export const NO_FILTER_PREFIXES = ["/exports", "/admin", "/ctvs/lookup"];
