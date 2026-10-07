"use client";
import { X } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { Badge } from "@/components/ui/primitives";
import { decodeClickFilters, encodeClickFilters, type ClickFilters } from "@/lib/filters/click-filters";

const LABELS: Record<string, string> = {
  links: "Link",
  access_prefix: "Prefix truy cập",
  api_version: "API",
  dest_host: "Domain đích",
  device: "Thiết bị",
  os: "HĐH",
  browser: "Trình duyệt",
  source_group: "Nguồn",
  referer_host: "Referer",
  country: "Quốc gia",
  province: "Tỉnh/thành",
  ip: "IP",
  weekdays: "Thứ",
  hour_from: "Từ giờ",
  hour_to: "Đến giờ",
  quality: "Chất lượng",
  visit: "Lượt",
  is_custom: "Link custom",
  campaign_presence: "Chiến dịch",
  ctv_presence: "CTV",
  params: "Tham số",
  link_created_from: "Link tạo từ",
  link_created_to: "Link tạo đến",
};

function describe(v: unknown): string {
  if (v && typeof v === "object" && "values" in (v as object)) {
    const c = v as { values: string[]; exclude?: boolean };
    return (c.exclude ? "≠ " : "") + c.values.join(", ");
  }
  if (Array.isArray(v)) return v.map((x) => (typeof x === "object" ? `${x.key} ${x.op} ${(x.values ?? []).join("|")}` : String(x))).join("; ");
  return String(v);
}

/** Chip các tiêu chí chi tiết đang áp (từ drill-down Explorer) — bấm × để bỏ. */
export function ActiveClickFilters() {
  const sp = useSearchParams();
  const router = useRouter();
  const f = decodeClickFilters(sp.get("cf"));
  const keys = Object.keys(f) as (keyof ClickFilters)[];
  if (!keys.length) return null;
  const remove = (k: keyof ClickFilters) => {
    const next = { ...f };
    delete next[k];
    const p = new URLSearchParams(sp.toString());
    const enc = encodeClickFilters(next);
    if (enc) p.set("cf", enc);
    else p.delete("cf");
    router.replace(`?${p.toString()}`);
  };
  return (
    <div className="flex flex-wrap items-center gap-2 text-xs">
      <span className="text-muted-foreground">Tiêu chí chi tiết:</span>
      {keys.map((k) => (
        <Badge key={k} tone="primary" className="gap-1">
          {LABELS[k] ?? k}: {describe(f[k])}
          <button type="button" onClick={() => remove(k)} aria-label={`Bỏ ${LABELS[k] ?? k}`}>
            <X className="size-3" />
          </button>
        </Badge>
      ))}
    </div>
  );
}
