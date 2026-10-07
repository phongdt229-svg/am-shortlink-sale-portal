import { ChevronRight } from "lucide-react";
import Link from "next/link";

export function PageHeader({
  title,
  crumbs = [],
  actions,
  subtitle,
}: {
  title: string;
  crumbs?: { href: string; label: string }[];
  actions?: React.ReactNode;
  subtitle?: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-2">
      <div className="min-w-0 space-y-0.5">
        {crumbs.length > 0 && (
          <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
            {crumbs.map((c) => (
              <span key={c.href} className="flex items-center gap-1">
                <Link href={c.href} className="hover:text-foreground hover:underline">
                  {c.label}
                </Link>
                <ChevronRight className="size-3" />
              </span>
            ))}
          </nav>
        )}
        <h1 className="truncate text-lg font-semibold">{title}</h1>
        {subtitle && <p className="text-sm text-muted-foreground">{subtitle}</p>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </div>
  );
}
