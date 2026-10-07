"use client";
import { useQuery } from "@tanstack/react-query";
import { Loader2, RotateCcw } from "lucide-react";
import { useQueryStates } from "nuqs";
import { useDeferredValue, useState, useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/primitives";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import {
  filterParsers,
  GRANULARITIES,
  GRANULARITY_LABELS,
  PRESET_LABELS,
  PRESETS,
  resolveRange,
  type Preset,
} from "@/lib/filters";
import { fmtDate, todayVN } from "@/lib/format";
import type { Role } from "@/lib/session/session";
import { cn } from "@/lib/utils";
import { MultiSelect } from "./multi-select";

/** Thanh bộ lọc chung — trạng thái nằm trên URL; đổi lọc → Server Component tải lại số liệu. */
export function FilterBar({ role }: { role: Role }) {
  const [pending, startTransition] = useTransition();
  const [f, setF] = useQueryStates(filterParsers, { shallow: false, startTransition, history: "push" });
  const range = resolveRange(f);
  const today = todayVN();

  const [accQ, setAccQ] = useState("");
  const [campQ, setCampQ] = useState("");
  const accQd = useDeferredValue(accQ);
  const campQd = useDeferredValue(campQ);

  const accounts = useQuery({
    queryKey: ["filters", "accounts", accQd],
    queryFn: async () => unwrap(await browserApi.GET("/v1/filters/accounts", { params: { query: { q: accQd || undefined, limit: 100 } } })),
    enabled: role !== "user",
  });
  const campaigns = useQuery({
    queryKey: ["filters", "campaigns", campQd, f.account],
    queryFn: async () =>
      unwrap(
        await browserApi.GET("/v1/filters/campaigns", {
          params: { query: { q: campQd || undefined, limit: 100, account: f.account.length ? f.account : undefined } },
        }),
      ),
  });
  const prefixes = useQuery({
    queryKey: ["filters", "prefixes"],
    queryFn: async () => unwrap(await browserApi.GET("/v1/filters/prefixes")),
    staleTime: 10 * 60_000,
  });

  const [ctvText, setCtvText] = useState(f.ctv.join(", "));
  const hasFilters = f.account.length + f.campaign.length + f.ctv.length + f.prefix.length > 0 || f.compare || f.preset !== "30d";

  return (
    <div className="sticky top-14 z-20 border-b bg-background/95 px-4 py-2 backdrop-blur">
      <div className="flex flex-wrap items-center gap-2">
        <Select
          aria-label="Khoảng ngày"
          value={f.preset}
          onChange={(e) => {
            const p = e.target.value as Preset;
            if (p === "custom") void setF({ preset: p, from: range.from, to: range.to });
            else void setF({ preset: p, from: null, to: null });
          }}
          className="h-8"
        >
          {PRESETS.map((p) => (
            <option key={p} value={p}>
              {PRESET_LABELS[p]}
            </option>
          ))}
        </Select>
        {f.preset === "custom" && (
          <div className="flex items-center gap-1">
            <Input type="date" aria-label="Từ ngày" className="h-8 w-36" value={range.from} max={today} onChange={(e) => void setF({ from: e.target.value })} />
            <span className="text-muted-foreground">–</span>
            <Input type="date" aria-label="Đến ngày" className="h-8 w-36" value={range.to} max={today} onChange={(e) => void setF({ to: e.target.value })} />
          </div>
        )}

        {role !== "user" && (
          <MultiSelect
            label="Tài khoản"
            value={f.account}
            onChange={(v) => void setF({ account: v.length ? v : null, campaign: null })}
            options={(accounts.data?.items ?? []).map((a) => ({ value: a.username, label: a.username }))}
            loading={accounts.isFetching}
            onSearch={setAccQ}
          />
        )}
        <MultiSelect
          label="Chiến dịch"
          value={f.campaign}
          onChange={(v) => void setF({ campaign: v.length ? v : null })}
          options={(campaigns.data?.items ?? []).map((c) => ({ value: c.code, label: c.code, hint: c.name !== c.code ? c.name : undefined }))}
          loading={campaigns.isFetching}
          onSearch={setCampQ}
        />
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const list = ctvText.split(/[\s,;]+/).map((s) => s.trim()).filter(Boolean).slice(0, 100);
            void setF({ ctv: list.length ? list : null });
          }}
        >
          <Input
            aria-label="CTV (SĐT / hash, cách nhau bởi dấu phẩy)"
            placeholder="CTV: SĐT / hash, Enter"
            className="h-8 w-48"
            value={ctvText}
            onChange={(e) => setCtvText(e.target.value)}
            onBlur={(e) => e.currentTarget.form?.requestSubmit()}
          />
        </form>
        <Select
          aria-label="Prefix"
          className="h-8"
          value={f.prefix[0] ?? ""}
          onChange={(e) => void setF({ prefix: e.target.value ? [e.target.value] : null })}
        >
          <option value="">Prefix: tất cả</option>
          {(prefixes.data?.items ?? []).map((p) => (
            <option key={p.id} value={p.id}>
              /{p.id}
            </option>
          ))}
        </Select>

        <label className="flex h-8 cursor-pointer items-center gap-2 rounded-md border bg-card px-2 text-sm">
          <input type="checkbox" checked={f.compare} onChange={(e) => void setF({ compare: e.target.checked || null })} />
          So kỳ trước
        </label>

        <div className="flex h-8 overflow-hidden rounded-md border text-sm" role="group" aria-label="Đơn vị thời gian">
          {GRANULARITIES.map((g) => (
            <button
              key={g}
              type="button"
              onClick={() => void setF({ granularity: g === "day" ? null : g })}
              className={cn("px-2.5", f.granularity === g ? "bg-primary text-primary-foreground" : "bg-card hover:bg-muted")}
              aria-pressed={f.granularity === g}
            >
              {GRANULARITY_LABELS[g]}
            </button>
          ))}
        </div>

        {hasFilters && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setCtvText("");
              void setF({ preset: null, from: null, to: null, account: null, campaign: null, ctv: null, prefix: null, compare: null, granularity: null });
            }}
          >
            <RotateCcw /> Đặt lại
          </Button>
        )}
        {pending && <Loader2 className="size-4 animate-spin text-muted-foreground" aria-label="Đang tải" />}
      </div>
      <p className="mt-1 text-xs text-muted-foreground">
        {fmtDate(range.from)} – {fmtDate(range.to)}
        {range.to === today && " · Số liệu hôm nay có thể trễ ≤ 1 phút"}
        {range.error && <span className="text-destructive"> · {range.error}</span>}
      </p>
    </div>
  );
}
