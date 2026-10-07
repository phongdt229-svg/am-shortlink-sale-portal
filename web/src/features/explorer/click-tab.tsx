"use client";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import { ClickLog } from "@/features/clicks/click-log";
import type { Role } from "@/lib/session/session";
import { Explorer } from "./explorer";
import type { Lock } from "./state";

/** Tab "Lượt click": Click Explorer khoá theo đối tượng đang xem + danh sách click thô cùng khoá. */
export function ClickTab({ role, lock }: { role: Role; lock: Lock }) {
  const base: Record<string, string[]> = {};
  if (lock.account) base.account = lock.account;
  if (lock.campaign) base.campaign = lock.campaign;
  if (lock.ctv) base.ctv = lock.ctv;
  return (
    <div className="space-y-4">
      <Explorer role={role} lock={lock} compact />
      <Card>
        <CardHeader>
          <CardTitle>Click thô</CardTitle>
        </CardHeader>
        <CardContent className="pt-2">
          <ClickLog locked={{ base, filters: lock.links ? { links: { values: lock.links } } : undefined }} />
        </CardContent>
      </Card>
    </div>
  );
}
