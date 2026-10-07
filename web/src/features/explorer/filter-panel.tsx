"use client";
import { useQuery } from "@tanstack/react-query";
import { Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { MultiSelect } from "@/components/filters/multi-select";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/primitives";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import type { components } from "@/lib/api/schema";
import type { ClickFilters } from "@/lib/filters/click-filters";
import { fmtNumber } from "@/lib/format";
import { cn } from "@/lib/utils";
import { FACET_CRITERIA, WEEKDAYS } from "./dims";

type Criterion = components["schemas"]["Criterion"];
type ParamCondition = components["schemas"]["ParamCondition"];

function useFacets(field: string, q: string, accounts: string[], enabled = true) {
  return useQuery({
    queryKey: ["facets", field, q, accounts],
    queryFn: async () =>
      unwrap(
        await browserApi.GET("/v1/reports/clicks/facets", {
          params: { query: { field, q: q || undefined, account: accounts.length ? accounts : undefined, limit: 100 } },
        }),
      ),
    enabled,
    staleTime: 5 * 60_000,
  });
}

/** Một tiêu chí chọn nhiều + nút loại trừ (NOT). */
function FacetCriterion({
  label,
  field,
  value,
  onChange,
  accounts,
}: {
  label: string;
  field: string;
  value?: Criterion;
  onChange: (c?: Criterion) => void;
  accounts: string[];
}) {
  const [q, setQ] = useState("");
  const facets = useFacets(field, q, accounts);
  const vals = value?.values ?? [];
  const exclude = !!value?.exclude;
  return (
    <div className="flex items-center gap-1">
      <MultiSelect
        label={exclude ? `${label} ≠` : label}
        value={vals}
        onChange={(v) => onChange(v.length ? { values: v, exclude } : undefined)}
        options={(facets.data?.items ?? []).map((f) => ({ value: f.value, label: f.label, hint: fmtNumber(f.clicks) }))}
        loading={facets.isFetching}
        onSearch={setQ}
        className="max-w-56"
      />
      {vals.length > 0 && (
        <button
          type="button"
          onClick={() => onChange({ values: vals, exclude: !exclude })}
          className={cn("rounded border px-1.5 py-1 text-xs", exclude ? "border-destructive text-destructive" : "text-muted-foreground hover:bg-muted")}
          title="Loại trừ các giá trị đã chọn"
          aria-pressed={exclude}
        >
          NOT
        </button>
      )}
    </div>
  );
}

function ParamRow({
  cond,
  keys,
  accounts,
  onChange,
  onRemove,
}: {
  cond: ParamCondition;
  keys: { key: string; label: string }[];
  accounts: string[];
  onChange: (c: ParamCondition) => void;
  onRemove: () => void;
}) {
  const [q, setQ] = useState("");
  const needsValues = cond.op !== "exists" && cond.op !== "not_exists";
  const facets = useFacets(`param.${cond.key}`, q, accounts, !!cond.key && needsValues);
  return (
    <div className="flex flex-wrap items-center gap-1">
      <Select value={cond.key} onChange={(e) => onChange({ ...cond, key: e.target.value, values: [] })} className="h-8" aria-label="Tham số">
        {keys.map((k) => (
          <option key={k.key} value={k.key}>
            {k.label}
          </option>
        ))}
      </Select>
      <Select value={cond.op} onChange={(e) => onChange({ ...cond, op: e.target.value as ParamCondition["op"] })} className="h-8" aria-label="Điều kiện">
        <option value="in">thuộc</option>
        <option value="contains">chứa</option>
        <option value="exists">có tham số</option>
        <option value="not_exists">không có tham số</option>
      </Select>
      {cond.op === "in" && (
        <MultiSelect
          label="Giá trị"
          value={cond.values ?? []}
          onChange={(v) => onChange({ ...cond, values: v })}
          options={(facets.data?.items ?? []).map((f) => ({ value: f.value, label: f.label, hint: fmtNumber(f.clicks) }))}
          loading={facets.isFetching}
          onSearch={setQ}
        />
      )}
      {cond.op === "contains" && (
        <Input
          className="h-8 w-40"
          placeholder="chuỗi con"
          value={cond.values?.[0] ?? ""}
          onChange={(e) => onChange({ ...cond, values: e.target.value ? [e.target.value] : [] })}
        />
      )}
      <Button variant="ghost" size="icon" onClick={onRemove} aria-label="Bỏ điều kiện">
        <Trash2 />
      </Button>
    </div>
  );
}

const RADIO = {
  quality: [
    ["all", "Tất cả"],
    ["valid", "Hợp lệ (không bot)"],
    ["bot", "Chỉ bot"],
    ["suspicious", "Chỉ nghi vấn"],
  ],
  visit: [
    ["all", "Tất cả"],
    ["first", "Lần đầu trong ngày"],
    ["repeat", "Lặp lại"],
  ],
  campaign_presence: [
    ["any", "Tất cả"],
    ["with", "Có chiến dịch"],
    ["without", "Không chiến dịch"],
  ],
  ctv_presence: [
    ["any", "Tất cả"],
    ["identified", "Đã định danh"],
    ["unidentified", "Chưa định danh"],
  ],
} as const;

/** Bảng tiêu chí lọc chi tiết. AND giữa tiêu chí, OR trong cùng tiêu chí, NOT từng tiêu chí. */
export function FilterPanel({
  value,
  onChange,
  accounts,
  isAdmin,
  lockLinks,
}: {
  value: ClickFilters;
  onChange: (f: ClickFilters) => void;
  accounts: string[];
  isAdmin: boolean;
  lockLinks?: boolean;
}) {
  const set = (patch: Partial<ClickFilters>) => onChange({ ...value, ...patch });
  const params = useQuery({
    queryKey: ["params-registry-light"],
    queryFn: async () => {
      const today = new Date().toISOString().slice(0, 10);
      return unwrap(await browserApi.GET("/v1/reports/params", { params: { query: { from: today, to: today } } }));
    },
    staleTime: 10 * 60_000,
  });
  const paramKeys = (params.data?.items ?? []).map((p) => ({ key: p.key, label: p.label }));
  const conds = value.params ?? [];

  return (
    <div className="space-y-3 text-sm">
      <div className="flex flex-wrap gap-2">
        {FACET_CRITERIA.map((c) => (
          <FacetCriterion
            key={c.key}
            label={c.label}
            field={c.field}
            accounts={accounts}
            value={(value as Record<string, Criterion | undefined>)[c.key]}
            onChange={(v) => set({ [c.key]: v } as Partial<ClickFilters>)}
          />
        ))}
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <fieldset className="space-y-1">
          <Label>Khung giờ (giờ Việt Nam)</Label>
          <div className="flex items-center gap-1">
            <Select
              className="h-8"
              aria-label="Từ giờ"
              value={value.hour_from ?? ""}
              onChange={(e) => set({ hour_from: e.target.value === "" ? undefined : Number(e.target.value), hour_to: value.hour_to ?? 23 })}
            >
              <option value="">Cả ngày</option>
              {Array.from({ length: 24 }, (_, h) => (
                <option key={h} value={h}>
                  {h}:00
                </option>
              ))}
            </Select>
            {value.hour_from !== undefined && (
              <>
                <span>–</span>
                <Select className="h-8" aria-label="Đến giờ" value={value.hour_to ?? 23} onChange={(e) => set({ hour_to: Number(e.target.value) })}>
                  {Array.from({ length: 24 }, (_, h) => (
                    <option key={h} value={h}>
                      {h}:59
                    </option>
                  ))}
                </Select>
              </>
            )}
          </div>
          <div className="flex gap-2 text-xs">
            <button type="button" className="text-primary hover:underline" onClick={() => set({ hour_from: 8, hour_to: 17, weekdays: [1, 2, 3, 4, 5] })}>
              Giờ hành chính
            </button>
            <button type="button" className="text-primary hover:underline" onClick={() => set({ hour_from: 18, hour_to: 7 })}>
              Ngoài giờ
            </button>
          </div>
        </fieldset>
        <fieldset className="space-y-1">
          <Label>Thứ trong tuần</Label>
          <div className="flex flex-wrap gap-1">
            {[1, 2, 3, 4, 5, 6, 0].map((d) => {
              const on = value.weekdays?.includes(d);
              return (
                <button
                  key={d}
                  type="button"
                  aria-pressed={on}
                  onClick={() => {
                    const cur = value.weekdays ?? [];
                    const next = on ? cur.filter((x) => x !== d) : [...cur, d];
                    set({ weekdays: next.length ? next : undefined });
                  }}
                  className={cn("h-8 w-9 rounded border text-xs", on ? "border-primary bg-primary text-primary-foreground" : "hover:bg-muted")}
                >
                  {WEEKDAYS[d]}
                </button>
              );
            })}
          </div>
        </fieldset>
        {(["quality", "visit"] as const).map((k) => (
          <fieldset key={k} className="space-y-1">
            <Label>{k === "quality" ? "Chất lượng" : "Lượt truy cập"}</Label>
            <Select
              className="h-8 w-full"
              value={value[k] ?? "all"}
              onChange={(e) => set({ [k]: e.target.value === "all" ? undefined : e.target.value } as Partial<ClickFilters>)}
            >
              {RADIO[k].map(([v, l]) => (
                <option key={v} value={v}>
                  {l}
                </option>
              ))}
            </Select>
          </fieldset>
        ))}
        {(["campaign_presence", "ctv_presence"] as const).map((k) => (
          <fieldset key={k} className="space-y-1">
            <Label>{k === "campaign_presence" ? "Chiến dịch" : "CTV"}</Label>
            <Select
              className="h-8 w-full"
              value={value[k] ?? "any"}
              onChange={(e) => set({ [k]: e.target.value === "any" ? undefined : e.target.value } as Partial<ClickFilters>)}
            >
              {RADIO[k].map(([v, l]) => (
                <option key={v} value={v}>
                  {l}
                </option>
              ))}
            </Select>
          </fieldset>
        ))}
        <fieldset className="space-y-1">
          <Label>Link custom</Label>
          <Select
            className="h-8 w-full"
            value={value.is_custom === undefined ? "" : String(value.is_custom)}
            onChange={(e) => set({ is_custom: e.target.value === "" ? undefined : e.target.value === "true" })}
          >
            <option value="">Tất cả</option>
            <option value="true">Chỉ link custom</option>
            <option value="false">Chỉ link sinh tự động</option>
          </Select>
        </fieldset>
        <fieldset className="space-y-1">
          <Label>Ngày tạo link</Label>
          <div className="flex items-center gap-1">
            <Input
              type="date"
              className="h-8"
              aria-label="Link tạo từ ngày"
              value={value.link_created_from ?? ""}
              onChange={(e) => set({ link_created_from: e.target.value || undefined })}
            />
            <span>–</span>
            <Input
              type="date"
              className="h-8"
              aria-label="Link tạo đến ngày"
              value={value.link_created_to ?? ""}
              onChange={(e) => set({ link_created_to: e.target.value || undefined })}
            />
          </div>
        </fieldset>
      </div>

      <div className="grid gap-3 md:grid-cols-2">
        {!lockLinks && (
          <fieldset className="space-y-1">
            <Label htmlFor="ex-links">Mã link (dán danh sách)</Label>
            <Textarea
              id="ex-links"
              className="min-h-16 font-mono text-xs"
              placeholder="abc123, xyz789…"
              defaultValue={(value.links?.values ?? []).join("\n")}
              onBlur={(e) => {
                const v = e.target.value
                  .split(/[\s,;]+/)
                  .map((s) => s.replace(/^.*\//, "").trim())
                  .filter(Boolean)
                  .slice(0, 500);
                set({ links: v.length ? { values: v, exclude: value.links?.exclude } : undefined });
              }}
            />
          </fieldset>
        )}
        {isAdmin && (
          <fieldset className="space-y-1">
            <Label htmlFor="ex-ip">IP / dải CIDR (admin)</Label>
            <Textarea
              id="ex-ip"
              className="min-h-16 font-mono text-xs"
              placeholder="113.161.20.5, 27.72.0.0/16"
              defaultValue={(value.ip?.values ?? []).join("\n")}
              onBlur={(e) => {
                const v = e.target.value
                  .split(/[\s,;]+/)
                  .map((s) => s.trim())
                  .filter(Boolean)
                  .slice(0, 200);
                set({ ip: v.length ? { values: v } : undefined });
              }}
            />
          </fieldset>
        )}
      </div>

      <fieldset className="space-y-1">
        <Label>Tham số URL</Label>
        {conds.map((c, i) => (
          <ParamRow
            key={i}
            cond={c}
            keys={paramKeys}
            accounts={accounts}
            onChange={(n) => set({ params: conds.map((x, j) => (j === i ? n : x)) })}
            onRemove={() => {
              const next = conds.filter((_, j) => j !== i);
              set({ params: next.length ? next : undefined });
            }}
          />
        ))}
        {conds.length < 10 && paramKeys.length > 0 && (
          <Button variant="outline" size="sm" onClick={() => set({ params: [...conds, { key: paramKeys[0]!.key, op: "in", values: [] }] })}>
            <Plus /> Thêm điều kiện tham số
          </Button>
        )}
      </fieldset>
    </div>
  );
}
