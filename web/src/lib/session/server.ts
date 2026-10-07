import "server-only";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { serverEnv } from "@/lib/env";
import { openSession, SESSION_COOKIE, type Session } from "./session";

/** Phiên hiện tại trong RSC / route handler (middleware đã refresh nếu cần). */
export async function getSession(): Promise<Session | null> {
  const env = serverEnv();
  const jar = await cookies();
  return openSession(jar.get(SESSION_COOKIE)?.value, env.PORTAL_SESSION_SECRET, env.SESSION_IDLE_HOURS);
}

export async function requireSession(): Promise<Session> {
  const s = await getSession();
  if (!s) redirect("/login");
  return s;
}
