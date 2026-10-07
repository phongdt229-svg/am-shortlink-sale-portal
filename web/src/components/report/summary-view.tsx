import { BreakdownPanel } from "@/components/charts/breakdown-panel";
import { Heatmap } from "@/components/charts/heatmap";
import { SeriesChart } from "@/components/charts/series-chart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/primitives";
import type { components } from "@/lib/api/schema";
import { KpiCards } from "./kpi-cards";

type Summary = components["schemas"]["ReportSummary"];

/** Khối dùng chung cho Tổng quan / chi tiết tài khoản / chiến dịch / CTV / link. */
export function SummaryView({ summary, children }: { summary: Summary; children?: React.ReactNode }) {
  return (
    <>
      <KpiCards kpis={summary.kpis} />
      <Card>
        <CardHeader>
          <CardTitle>Diễn biến theo thời gian</CardTitle>
        </CardHeader>
        <CardContent>
          <SeriesChart series={summary.series} />
        </CardContent>
      </Card>
      {children}
      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Phân rã lượt click</CardTitle>
          </CardHeader>
          <CardContent>
            <BreakdownPanel data={summary.breakdowns} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Giờ × thứ trong tuần</CardTitle>
          </CardHeader>
          <CardContent>
            <Heatmap cells={summary.heatmap} />
          </CardContent>
        </Card>
      </div>
    </>
  );
}
