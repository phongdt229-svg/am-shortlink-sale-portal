"use client";
import { useQuery } from "@tanstack/react-query";
import { Loader2, Search } from "lucide-react";
import Link from "next/link";
import { parseAsString, useQueryState } from "nuqs";
import { useState } from "react";
import { EmptyState, ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, Input } from "@/components/ui/primitives";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import { fmtDate, fmtNumber } from "@/lib/format";

export function CTVLookup() {
  const [q, setQ] = useQueryState("q", parseAsString.withDefault(""));
  const [text, setText] = useState(q);
  const res = useQuery({
    queryKey: ["ctv-lookup", q],
    queryFn: async () => unwrap(await browserApi.GET("/v1/reports/ctvs/lookup", { params: { query: { q } } })),
    enabled: q.trim().length >= 3,
  });

  return (
    <div className="space-y-4">
      <form
        className="flex max-w-lg gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void setQ(text.trim() || null);
        }}
      >
        <Input value={text} onChange={(e) => setText(e.target.value)} placeholder="0901 234 567 / +84901234567 / hash" aria-label="SĐT hoặc hash CTV" autoFocus inputMode="search" />
        <Button type="submit" disabled={text.trim().length < 3}>
          <Search /> Tra cứu
        </Button>
      </form>
      {res.isFetching && (
        <p className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" /> Đang tra cứu…
        </p>
      )}
      {res.isError && <ErrorState error={res.error} />}
      {res.data && (
        <>
          <p className="text-sm text-muted-foreground">
            Giá trị chuẩn hoá: <span className="font-mono text-foreground">{res.data.normalized}</span>
          </p>
          {res.data.items.length === 0 ? (
            <EmptyState title="Không tìm thấy CTV" hint="Kiểm tra lại SĐT / hash hoặc CTV không thuộc phạm vi tài khoản của bạn" />
          ) : (
            res.data.items.map((it) => (
              <Card key={it.owner}>
                <CardHeader>
                  <CardTitle>
                    <Link href={`/ctvs/${encodeURIComponent(it.ctv_ref)}?account=${encodeURIComponent(it.owner)}`} className="text-primary hover:underline">
                      {it.ctv_display}
                    </Link>
                    {it.name && <span className="ml-2 font-normal text-muted-foreground">{it.name}</span>}
                  </CardTitle>
                  <span className="text-xs text-muted-foreground">Tài khoản {it.owner}</span>
                </CardHeader>
                <CardContent className="space-y-3 text-sm">
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                    <Stat label="Số link" value={fmtNumber(it.links)} />
                    <Stat label="Tổng click (toàn thời gian)" value={fmtNumber(it.clicks_total)} />
                    <Stat label="Link đầu tiên" value={fmtDate(it.first_link_at)} />
                    <Stat label="Link gần nhất" value={fmtDate(it.last_link_at)} />
                  </div>
                  {it.sample_links?.length ? (
                    <ul className="space-y-1">
                      {it.sample_links.map((l) => (
                        <li key={l.code} className="truncate">
                          <Link href={`/links/${encodeURIComponent(l.code)}`} className="font-medium text-primary hover:underline">
                            /{l.prefix}/{l.code}
                          </Link>{" "}
                          <span className="text-xs text-muted-foreground">{l.campaign_code || "—"} · {l.long_url}</span>
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </CardContent>
              </Card>
            ))
          )}
        </>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="font-semibold tabular-nums">{value}</p>
    </div>
  );
}
