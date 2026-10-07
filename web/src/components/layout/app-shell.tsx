"use client";
import { LogOut, Menu, X } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { Suspense, useState, type ReactNode } from "react";
import { FilterBar } from "@/components/filters/filter-bar";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/primitives";
import type { Role } from "@/lib/session/session";
import { NO_FILTER_PREFIXES } from "./nav";
import { Sidebar } from "./sidebar";

const ROLE_LABEL: Record<Role, string> = { admin: "Quản trị", user: "Đối tác", viewer: "Xem báo cáo" };

export function AppShell({ user, children }: { user: { username: string; role: Role }; children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);
  const router = useRouter();
  const pathname = usePathname();
  const showFilters = !NO_FILTER_PREFIXES.some((p) => pathname.startsWith(p));

  async function logout() {
    setLoggingOut(true);
    await fetch("/api/auth/logout", { method: "POST" }).catch(() => undefined);
    router.replace("/login");
    router.refresh();
  }

  return (
    <div className="flex min-h-dvh">
      <aside className="sticky top-0 hidden h-dvh w-60 shrink-0 overflow-y-auto border-r bg-card lg:block">
        <div className="flex h-14 items-center border-b px-4 font-semibold">Shortlink Portal</div>
        <Suspense>
          <Sidebar role={user.role} />
        </Suspense>
      </aside>

      {open && (
        <div className="fixed inset-0 z-40 lg:hidden" role="dialog" aria-modal="true">
          <div className="absolute inset-0 bg-black/40" onClick={() => setOpen(false)} />
          <aside className="absolute inset-y-0 left-0 w-64 overflow-y-auto bg-card shadow-lg">
            <div className="flex h-14 items-center justify-between border-b px-4 font-semibold">
              Shortlink Portal
              <Button variant="ghost" size="icon" onClick={() => setOpen(false)} aria-label="Đóng menu">
                <X />
              </Button>
            </div>
            <Suspense>
              <Sidebar role={user.role} onNavigate={() => setOpen(false)} />
            </Suspense>
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b bg-card/95 px-4 backdrop-blur">
          <Button variant="ghost" size="icon" className="lg:hidden" onClick={() => setOpen(true)} aria-label="Mở menu">
            <Menu />
          </Button>
          <div className="ml-auto flex items-center gap-3 text-sm">
            <span className="hidden sm:inline">{user.username}</span>
            <Badge tone="primary">{ROLE_LABEL[user.role]}</Badge>
            <Button variant="ghost" size="sm" onClick={logout} disabled={loggingOut}>
              <LogOut />
              <span className="hidden sm:inline">Đăng xuất</span>
            </Button>
          </div>
        </header>
        {showFilters && (
          <Suspense>
            <FilterBar role={user.role} />
          </Suspense>
        )}
        <main className="flex-1 space-y-4 p-4">{children}</main>
      </div>
    </div>
  );
}
