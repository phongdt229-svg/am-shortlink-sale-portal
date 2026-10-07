"use client";
import {
  ChartNoAxesCombined,
  Download,
  LayoutDashboard,
  Link as LinkIcon,
  Megaphone,
  MousePointerClick,
  Search,
  Settings2,
  ShieldAlert,
  Tags,
  UserCheck,
  Users,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import type { Role } from "@/lib/session/session";
import { cn } from "@/lib/utils";
import { navFor, type NavItem } from "./nav";

const ICONS: Record<string, LucideIcon> = {
  "layout-dashboard": LayoutDashboard,
  users: Users,
  megaphone: Megaphone,
  "user-check": UserCheck,
  link: LinkIcon,
  "mouse-pointer-click": MousePointerClick,
  "chart-no-axes-combined": ChartNoAxesCombined,
  tags: Tags,
  "shield-alert": ShieldAlert,
  search: Search,
  download: Download,
  "settings-2": Settings2,
};

const SECTIONS: { key: NavItem["section"]; label: string }[] = [
  { key: "report", label: "Báo cáo" },
  { key: "tools", label: "Tiện ích" },
  { key: "admin", label: "Quản trị" },
];

/** Mục đang chọn = mục có href dài nhất khớp đường dẫn (vd /clicks/explore không làm sáng /clicks). */
function activeHref(items: NavItem[], pathname: string): string | undefined {
  return items.filter((i) => pathname === i.href || pathname.startsWith(i.href + "/")).sort((a, b) => b.href.length - a.href.length)[0]?.href;
}

const FILTER_KEYS = ["preset", "from", "to", "account", "campaign", "ctv", "prefix", "compare", "granularity"];

export function Sidebar({ role, onNavigate }: { role: Role; onNavigate?: () => void }) {
  const pathname = usePathname();
  const search = useSearchParams();
  const items = navFor(role);
  const active = activeHref(items, pathname);

  // Giữ bộ lọc chung khi chuyển màn hình báo cáo.
  const keep = new URLSearchParams();
  for (const k of FILTER_KEYS) {
    const v = search.get(k);
    if (v) keep.set(k, v);
  }
  const qs = keep.toString();

  return (
    <nav className="flex flex-col gap-4 p-3 text-sm" aria-label="Điều hướng chính">
      {SECTIONS.map((sec) => {
        const list = items.filter((i) => i.section === sec.key);
        if (!list.length) return null;
        return (
          <div key={sec.key} className="space-y-1">
            <p className="px-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">{sec.label}</p>
            {list.map((item) => {
              const Icon = ICONS[item.icon] ?? LayoutDashboard;
              const href = item.section === "report" && qs ? `${item.href}?${qs}` : item.href;
              return (
                <Link
                  key={item.href}
                  href={href}
                  onClick={onNavigate}
                  aria-current={active === item.href ? "page" : undefined}
                  className={cn(
                    "flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted",
                    active === item.href && "bg-accent font-medium text-accent-foreground hover:bg-accent",
                  )}
                >
                  <Icon className="size-4" />
                  {item.label}
                </Link>
              );
            })}
          </div>
        );
      })}
    </nav>
  );
}
