import { AppShell } from "@/components/layout/app-shell";
import { requireSession } from "@/lib/session/server";

export default async function PortalLayout({ children }: { children: React.ReactNode }) {
  const s = await requireSession();
  return <AppShell user={s.user}>{children}</AppShell>;
}
