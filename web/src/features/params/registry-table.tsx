"use client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { useState } from "react";
import { EmptyState, ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Badge, Card, CardContent, Input, Select } from "@/components/ui/primitives";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import type { components } from "@/lib/api/schema";
import { fmtDate, fmtDateTime, fmtNumber, fmtPercent } from "@/lib/format";

type Item = components["schemas"]["ParamRegistryItem"];
type Update = components["schemas"]["ParamRegistryUpdate"];

function Row({ it }: { it: Item }) {
  const qc = useQueryClient();
  const [label, setLabel] = useState(it.label);
  const [pii, setPii] = useState(it.pii);
  const [maxValues, setMaxValues] = useState(it.max_values ?? 1000);
  const save = useMutation({
    mutationFn: async (body: Update) => unwrap(await browserApi.PATCH("/v1/param-registry/{key}", { params: { path: { key: it.key } }, body })),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["param-registry"] }),
  });
  const dirty = label !== it.label || pii !== it.pii || maxValues !== (it.max_values ?? 1000);
  const isPII = pii !== "none";
  const bf = it.backfill;
  return (
    <tr className="border-t align-top">
      <td className="px-2 py-2">
        <span className="font-mono font-medium">{it.key}</span>
        {it.in_registry === false && (
          <Badge tone="primary" className="ml-2">
            Mới
          </Badge>
        )}
      </td>
      <td className="px-2 py-2">
        <Input className="h-8 w-44" value={label} onChange={(e) => setLabel(e.target.value)} aria-label={`Nhãn ${it.key}`} maxLength={80} />
      </td>
      <td className="px-2 py-2 tabular-nums">{fmtNumber(it.links)}</td>
      <td className="px-2 py-2 tabular-nums">{it.values >= 10000 ? "≥ 10.000" : fmtNumber(it.values)}</td>
      <td className="max-w-56 px-2 py-2 text-xs text-muted-foreground">
        <span className="line-clamp-2 break-all">{(it.samples ?? []).join(", ")}</span>
      </td>
      <td className="px-2 py-2">
        <Select className="h-8" value={pii} onChange={(e) => setPii(e.target.value as Item["pii"])} aria-label={`PII ${it.key}`}>
          <option value="none">Không</option>
          <option value="hash">PII – băm</option>
          <option value="drop">PII – bỏ</option>
        </Select>
      </td>
      <td className="px-2 py-2">
        <Input
          type="number"
          className="h-8 w-24"
          min={10}
          value={maxValues}
          onChange={(e) => setMaxValues(Number(e.target.value))}
          aria-label={`Ngưỡng giá trị ${it.key}`}
        />
      </td>
      <td className="px-2 py-2">
        <label className="flex items-center gap-2" title={isPII ? "Tham số PII không bật theo dõi" : undefined}>
          <input
            type="checkbox"
            checked={it.tracked}
            disabled={isPII || save.isPending}
            onChange={(e) => save.mutate({ tracked: e.target.checked, label, pii, max_values: maxValues })}
          />
          {it.tracked ? "Đang theo dõi" : "Tắt"}
        </label>
        {it.status === "high_cardinality" && <Badge tone="warning">Nhiều giá trị</Badge>}
      </td>
      <td className="px-2 py-2 text-xs">
        {bf?.status ? (
          <div className="w-32 space-y-1">
            <Badge tone={bf.status === "done" ? "success" : bf.status === "failed" ? "danger" : "primary"}>
              {bf.status === "done" ? "Xong" : bf.status === "pending" ? "Chờ chạy" : bf.status}
            </Badge>
            {bf.status !== "done" && bf.progress !== undefined && (
              <div className="h-1.5 rounded bg-muted" role="progressbar" aria-valuenow={Math.round(bf.progress * 100)} aria-valuemin={0} aria-valuemax={100}>
                <div className="h-1.5 rounded bg-[var(--chart-1)]" style={{ width: `${Math.round(bf.progress * 100)}%` }} title={fmtPercent(bf.progress)} />
              </div>
            )}
            {bf.from && <p className="text-muted-foreground">Số liệu từ {fmtDate(bf.from)}</p>}
            {bf.error && <p className="text-destructive">{bf.error}</p>}
          </div>
        ) : (
          <span className="text-muted-foreground">–</span>
        )}
      </td>
      <td className="px-2 py-2 text-right">
        <Button
          size="sm"
          variant="outline"
          disabled={!dirty || save.isPending}
          onClick={() => save.mutate({ label, pii, max_values: maxValues, tracked: isPII ? false : it.tracked })}
        >
          {save.isPending && <Loader2 className="animate-spin" />} Lưu
        </Button>
        {it.updated_by && (
          <p className="mt-1 text-xs text-muted-foreground">
            {it.updated_by} · {fmtDateTime(it.updated_at)}
          </p>
        )}
        {save.isError && <ErrorState error={save.error} className="mt-1 text-left" />}
      </td>
    </tr>
  );
}

export function RegistryTable() {
  const q = useQuery({
    queryKey: ["param-registry"],
    queryFn: async () => unwrap(await browserApi.GET("/v1/param-registry")),
    refetchInterval: (query) =>
      query.state.data?.items.some((i) => i.backfill?.status && i.backfill.status !== "done" && i.backfill.status !== "failed") ? 5000 : false,
  });
  if (q.isError) return <ErrorState error={q.error} />;
  if (q.isPending) return <Loader2 className="size-4 animate-spin" />;
  if (!q.data.items.length) return <EmptyState />;
  return (
    <Card>
      <CardContent className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="text-xs text-muted-foreground">
            <tr>
              {["Tham số", "Nhãn hiển thị", "Số link", "Số giá trị", "Ví dụ", "PII", "Ngưỡng giá trị", "Theo dõi", "Backfill", ""].map((h) => (
                <th key={h} className="px-2 py-1.5 text-left font-normal">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {q.data.items.map((it) => (
              <Row key={`${it.key}-${it.updated_at ?? ""}`} it={it} />
            ))}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}
