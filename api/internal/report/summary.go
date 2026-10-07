package report

import (
	"context"
	"sort"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/store"
)

// metrics: chỉ số kỳ m theo định nghĩa §5.2.
func (s *Service) metrics(ctx context.Context, m store.Match) (gen.Metrics, error) {
	lvl := store.ChooseLevel(m, 0, false)
	var (
		sums           store.Sums
		active         int64
		total, created int64
	)
	g, gctx := group(ctx)
	g.Go(func() (err error) { sums, err = s.st.SumStats(gctx, lvl, m); return })
	g.Go(func() (err error) { active, err = s.st.ActiveLinks(gctx, m); return })
	g.Go(func() (err error) { total, created, err = s.st.LinkCounts(gctx, m); return })
	if err := g.Wait(); err != nil {
		return gen.Metrics{}, err
	}
	newLinks := created
	if lvl.HasNewLinks() {
		newLinks = sums.NewLinks
	}
	return toMetrics(sums, newLinks, active, &total), nil
}

func toMetrics(s store.Sums, newLinks, active int64, total *int64) gen.Metrics {
	return gen.Metrics{
		TotalLinks:       total,
		NewLinks:         newLinks,
		ActiveLinks:      active,
		Clicks:           s.Clicks,
		UniqueClicks:     s.Unique,
		BotClicks:        s.Bot,
		SuspiciousClicks: s.Susp,
		CtrPerLink:       ratio(s.Clicks, active),
	}
}

func ratio(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// change: (cur - prev) / prev; nil khi kỳ trước = 0.
func change(cur, prev float64) *float64 {
	if prev == 0 {
		return nil
	}
	v := (cur - prev) / prev
	return &v
}

func changes(c, p gen.Metrics) gen.MetricChanges {
	tl := func(m gen.Metrics) float64 {
		if m.TotalLinks == nil {
			return 0
		}
		return float64(*m.TotalLinks)
	}
	return gen.MetricChanges{
		TotalLinks:       change(tl(c), tl(p)),
		NewLinks:         change(float64(c.NewLinks), float64(p.NewLinks)),
		ActiveLinks:      change(float64(c.ActiveLinks), float64(p.ActiveLinks)),
		Clicks:           change(float64(c.Clicks), float64(p.Clicks)),
		UniqueClicks:     change(float64(c.UniqueClicks), float64(p.UniqueClicks)),
		BotClicks:        change(float64(c.BotClicks), float64(p.BotClicks)),
		SuspiciousClicks: change(float64(c.SuspiciousClicks), float64(p.SuspiciousClicks)),
		CtrPerLink:       change(c.CtrPerLink, p.CtrPerLink),
	}
}

func period(m store.Match) gen.Period {
	return gen.Period{From: openapi_types.Date{Time: m.From}, To: openapi_types.Date{Time: m.To}}
}

// Kpis: kỳ hiện tại (+ kỳ trước nếu Compare).
func (s *Service) Kpis(ctx context.Context, q Query) (gen.Kpis, error) {
	var cur, prev gen.Metrics
	g, gctx := group(ctx)
	g.Go(func() (err error) { cur, err = s.metrics(gctx, q.M); return })
	if q.Comp {
		g.Go(func() (err error) { prev, err = s.metrics(gctx, q.Prev); return })
	}
	if err := g.Wait(); err != nil {
		return gen.Kpis{}, err
	}
	k := gen.Kpis{Period: period(q.M), Current: cur}
	if q.Comp {
		pp, ch := period(q.Prev), changes(cur, prev)
		k.PreviousPeriod, k.Previous, k.Change = &pp, &prev, &ch
	}
	return k, nil
}

// Buckets liệt kê ngày đầu các bucket phủ [from, to].
func Buckets(from, to time.Time, g store.Granularity) []time.Time {
	start := from
	switch g {
	case store.Week:
		wd := (int(from.Weekday()) + 6) % 7 // thứ Hai = 0
		start = from.AddDate(0, 0, -wd)
	case store.Month:
		start = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	var out []time.Time
	for d := start; !d.After(to); {
		out = append(out, d)
		switch g {
		case store.Week:
			d = d.AddDate(0, 0, 7)
		case store.Month:
			d = d.AddDate(0, 1, 0)
		default:
			d = d.AddDate(0, 0, 1)
		}
	}
	return out
}

// series: chuỗi của kỳ m, gom vào bucket của kỳ hiển thị `view` (kỳ trước: view = kỳ hiện tại, dịch N ngày).
func (s *Service) series(ctx context.Context, m, view store.Match, gran store.Granularity) ([]gen.SeriesPoint, error) {
	shift := int(view.From.Sub(m.From).Hours() / 24)
	lvl := store.ChooseLevel(m, 0, false)
	var (
		sums   map[string]store.Sums
		active map[string]int64
		newL   map[string]int64
	)
	g, gctx := group(ctx)
	g.Go(func() (err error) { sums, err = s.st.SeriesStats(gctx, lvl, m, gran, shift); return })
	g.Go(func() (err error) { active, err = s.st.ActiveSeries(gctx, m, gran, shift); return })
	if !lvl.HasNewLinks() {
		g.Go(func() (err error) { newL, err = s.st.NewLinkSeries(gctx, m, gran, shift); return })
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	buckets := Buckets(view.From, view.To, gran)
	out := make([]gen.SeriesPoint, len(buckets))
	for i, b := range buckets {
		k := b.Format("2006-01-02")
		sm := sums[k]
		nl := sm.NewLinks
		if newL != nil {
			nl = newL[k]
		}
		out[i] = gen.SeriesPoint{
			Period: openapi_types.Date{Time: b}, Clicks: sm.Clicks, UniqueClicks: sm.Unique,
			BotClicks: sm.Bot, NewLinks: nl, ActiveLinks: active[k],
		}
	}
	return out, nil
}

func (s *Service) Series(ctx context.Context, q Query) (gen.Series, error) {
	var cur, prev []gen.SeriesPoint
	g, gctx := group(ctx)
	g.Go(func() (err error) { cur, err = s.series(gctx, q.M, q.M, q.Gran); return })
	if q.Comp {
		g.Go(func() (err error) { prev, err = s.series(gctx, q.Prev, q.M, q.Gran); return })
	}
	if err := g.Wait(); err != nil {
		return gen.Series{}, err
	}
	out := gen.Series{Granularity: gen.Granularity(q.Gran), Points: cur}
	if q.Comp {
		out.Previous = &prev
	}
	return out, nil
}

func (s *Service) Breakdowns(ctx context.Context, q Query) (gen.Breakdowns, error) {
	raw, err := s.st.Breakdowns(ctx, store.ChooseLevel(q.M, 0, true), q.M)
	if err != nil {
		return gen.Breakdowns{}, err
	}
	items := func(dim string) []gen.BreakdownItem {
		m := raw[dim]
		var total int64
		for _, v := range m {
			total += v
		}
		out := make([]gen.BreakdownItem, 0, len(m))
		for k, v := range m {
			if v == 0 {
				continue
			}
			if k == "" {
				k = "(không rõ)"
			}
			out = append(out, gen.BreakdownItem{Key: k, Clicks: v, Share: ratio(v, total)})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Clicks != out[j].Clicks {
				return out[i].Clicks > out[j].Clicks
			}
			return out[i].Key < out[j].Key
		})
		return out
	}
	return gen.Breakdowns{
		Device: items("device"), Os: items("os"), Browser: items("browser"), SourceGroup: items("source_group"),
		Referer: items("referer"), Country: items("country"), AccessPrefix: items("access_prefix"),
	}, nil
}

func (s *Service) Heatmap(ctx context.Context, q Query) ([]gen.HeatmapCell, error) {
	hm, err := s.st.Heatmap(ctx, store.ChooseLevel(q.M, 0, true), q.M)
	if err != nil {
		return nil, err
	}
	out := make([]gen.HeatmapCell, 0, 7*24)
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			out = append(out, gen.HeatmapCell{Weekday: d, Hour: h, Clicks: hm[d][h]})
		}
	}
	return out, nil
}

// Summary: KPI + timeline + phân rã + heatmap.
func (s *Service) Summary(ctx context.Context, q Query) (gen.ReportSummary, error) {
	var out gen.ReportSummary
	g, gctx := group(ctx)
	g.Go(func() (err error) { out.Kpis, err = s.Kpis(gctx, q); return })
	g.Go(func() (err error) { out.Series, err = s.Series(gctx, q); return })
	g.Go(func() (err error) { out.Breakdowns, err = s.Breakdowns(gctx, q); return })
	g.Go(func() (err error) { out.Heatmap, err = s.Heatmap(gctx, q); return })
	return out, g.Wait()
}
