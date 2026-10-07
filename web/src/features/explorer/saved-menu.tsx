"use client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bookmark, Check, Link2, Save, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { ErrorState } from "@/components/states";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/primitives";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { browserApi } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/problem";
import { fmtDateTime } from "@/lib/format";
import { savedHref, type Globals } from "./saved-href";
import type { ExplorerState } from "./state";

export function SavedMenu({ state, globals }: { state: ExplorerState; globals: Globals }) {
  const qc = useQueryClient();
  const router = useRouter();
  const [name, setName] = useState("");
  const [copied, setCopied] = useState<string | null>(null);
  const list = useQuery({ queryKey: ["saved-reports"], queryFn: async () => unwrap(await browserApi.GET("/v1/saved-reports")) });
  const save = useMutation({
    mutationFn: async () =>
      unwrap(await browserApi.POST("/v1/saved-reports", { body: { name: name.trim(), kind: "explorer", query: { state, globals } } })),
    onSuccess: () => {
      setName("");
      void qc.invalidateQueries({ queryKey: ["saved-reports"] });
    },
  });
  const remove = useMutation({
    mutationFn: async (id: string) => {
      const r = await browserApi.DELETE("/v1/saved-reports/{id}", { params: { path: { id } } });
      if (!r.response.ok) unwrap(r);
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["saved-reports"] }),
  });
  const share = useMutation({
    mutationFn: async ({ id, enabled }: { id: string; enabled: boolean }) =>
      unwrap(await browserApi.POST("/v1/saved-reports/{id}/share", { params: { path: { id } }, body: { enabled } })),
    onSuccess: async (r) => {
      void qc.invalidateQueries({ queryKey: ["saved-reports"] });
      if (r.share_token) {
        const url = `${window.location.origin}/clicks/explore?shared=${r.share_token}`;
        await navigator.clipboard.writeText(url).catch(() => undefined);
        setCopied(r.id);
        setTimeout(() => setCopied(null), 2000);
      }
    },
  });

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm">
          <Bookmark /> Báo cáo của tôi
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-96 space-y-3" align="end">
        <form
          className="space-y-1.5"
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) save.mutate();
          }}
        >
          <Label htmlFor="save-name">Lưu bộ lọc hiện tại</Label>
          <div className="flex gap-2">
            <Input id="save-name" className="h-8" placeholder="Tên báo cáo" value={name} onChange={(e) => setName(e.target.value)} maxLength={120} />
            <Button size="sm" type="submit" disabled={!name.trim() || save.isPending}>
              <Save /> Lưu
            </Button>
          </div>
          {save.isError && <ErrorState error={save.error} />}
        </form>
        <div className="max-h-72 space-y-1 overflow-y-auto border-t pt-2">
          {(list.data?.items ?? []).length === 0 && <p className="text-xs text-muted-foreground">Chưa có báo cáo đã lưu.</p>}
          {(list.data?.items ?? []).map((r) => (
            <div key={r.id} className="flex items-center gap-1 rounded px-1 py-1 hover:bg-muted">
              <button type="button" className="min-w-0 flex-1 text-left" onClick={() => router.push(savedHref(r.query))}>
                <p className="truncate text-sm">{r.name}</p>
                <p className="text-xs text-muted-foreground">{fmtDateTime(r.updated_at)}</p>
              </button>
              <Button
                variant="ghost"
                size="icon"
                title={r.share_token ? "Đang chia sẻ — bấm để sao chép link (bấm giữ Shift để tắt chia sẻ)" : "Chia sẻ qua link"}
                onClick={(e) => share.mutate({ id: r.id, enabled: !(e.shiftKey && r.share_token) })}
                aria-label="Chia sẻ"
              >
                {copied === r.id ? <Check className="text-success" /> : <Link2 className={r.share_token ? "text-primary" : ""} />}
              </Button>
              <Button variant="ghost" size="icon" onClick={() => remove.mutate(r.id)} aria-label="Xoá">
                <Trash2 />
              </Button>
            </div>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">Người nhận link chỉ thấy phần dữ liệu thuộc quyền của họ.</p>
      </PopoverContent>
    </Popover>
  );
}
