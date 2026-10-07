"use client";
import { Check, ChevronDown, Loader2, X } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/primitives";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";

export interface Option {
  value: string;
  label: string;
  hint?: string;
}

interface Props {
  label: string;
  value: string[];
  onChange: (v: string[]) => void;
  options: Option[];
  loading?: boolean;
  /** Tìm phía server: gọi khi người dùng gõ (đã debounce ở caller nếu cần). */
  onSearch?: (q: string) => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
}

/** Chọn nhiều có tìm kiếm (OR trong cùng tiêu chí). */
export function MultiSelect({ label, value, onChange, options, loading, onSearch, placeholder, disabled, className }: Props) {
  const [q, setQ] = useState("");
  const filtered = onSearch ? options : options.filter((o) => (o.label + " " + o.value).toLowerCase().includes(q.toLowerCase()));
  const selectedLabels = value.map((v) => options.find((o) => o.value === v)?.label ?? v);

  function toggle(v: string) {
    onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v]);
  }

  return (
    <Popover>
      <PopoverTrigger asChild disabled={disabled}>
        <Button variant="outline" size="sm" className={cn("max-w-64 justify-between font-normal", className)}>
          <span className="truncate">
            <span className="text-muted-foreground">{label}: </span>
            {value.length === 0 ? "Tất cả" : value.length <= 2 ? selectedLabels.join(", ") : `${value.length} mục`}
          </span>
          <ChevronDown className="opacity-60" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-2">
        <Input
          autoFocus
          placeholder={placeholder ?? "Tìm…"}
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            onSearch?.(e.target.value);
          }}
          className="mb-2 h-8"
        />
        <div className="max-h-64 overflow-y-auto" role="listbox" aria-multiselectable="true">
          {loading && (
            <div className="flex items-center gap-2 p-2 text-xs text-muted-foreground">
              <Loader2 className="size-3 animate-spin" /> Đang tải…
            </div>
          )}
          {!loading && filtered.length === 0 && <p className="p-2 text-xs text-muted-foreground">Không có kết quả</p>}
          {filtered.map((o) => {
            const on = value.includes(o.value);
            return (
              <button
                key={o.value}
                type="button"
                role="option"
                aria-selected={on}
                onClick={() => toggle(o.value)}
                className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-muted"
              >
                <span className={cn("flex size-4 items-center justify-center rounded border", on && "border-primary bg-primary text-primary-foreground")}>
                  {on && <Check className="size-3" />}
                </span>
                <span className="truncate">{o.label}</span>
                {o.hint && <span className="ml-auto truncate text-xs text-muted-foreground">{o.hint}</span>}
              </button>
            );
          })}
        </div>
        {value.length > 0 && (
          <Button variant="ghost" size="sm" className="mt-2 w-full" onClick={() => onChange([])}>
            <X /> Bỏ chọn ({value.length})
          </Button>
        )}
      </PopoverContent>
    </Popover>
  );
}
