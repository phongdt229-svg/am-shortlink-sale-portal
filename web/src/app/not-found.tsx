import Link from "next/link";

export default function NotFound() {
  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-2 p-4 text-center">
      <h1 className="text-lg font-semibold">Không tìm thấy trang</h1>
      <p className="text-sm text-muted-foreground">Trang không tồn tại hoặc bạn không có quyền xem.</p>
      <Link href="/overview" className="text-sm text-primary underline">
        Về Tổng quan
      </Link>
    </main>
  );
}
