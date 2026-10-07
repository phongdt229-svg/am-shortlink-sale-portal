package report

import (
	"context"
	"sort"
	"strings"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/store"
)

func paramSummary(e store.RegistryEntry) gen.ParamSummary {
	ps := gen.ParamSummary{Key: e.Key, Label: e.Label, Status: gen.ParamSummaryStatus(e.Status)}
	if ps.Label == "" {
		ps.Label = e.Key
	}
	if ps.Status == "" {
		ps.Status = gen.ParamSummaryStatus("active")
	}
	if e.Backfill != nil && !e.Backfill.From.IsZero() {
		ps.TrackedSince = &openapi_types.Date{Time: dateOnly(e.Backfill.From)}
	}
	return ps
}

// Params: tham số đang theo dõi (không PII) kèm số giá trị / số link / tổng click trong kỳ.
func (s *Service) Params(ctx context.Context, q Query) (gen.ParamSummaryList, error) {
	return cached(s, ctx, "params", q, nil, func(ctx context.Context) (gen.ParamSummaryList, error) {
		reg, err := s.st.Registry(ctx)
		if err != nil {
			return gen.ParamSummaryList{}, err
		}
		var keys []string
		entries := map[string]store.RegistryEntry{}
		for _, e := range reg {
			if e.Tracked && (e.PII == "" || e.PII == "none") {
				keys = append(keys, e.Key)
				entries[e.Key] = e
			}
		}
		out := gen.ParamSummaryList{Items: []gen.ParamSummary{}}
		if len(keys) == 0 {
			return out, nil
		}
		var (
			stats map[string]store.ParamKeyStat
			links map[string]int64
		)
		g, gctx := group(ctx)
		g.Go(func() (err error) { stats, err = s.st.ParamKeyStats(gctx, q.M, keys); return })
		g.Go(func() (err error) { links, err = s.st.ParamLinkCounts(gctx, q.M, keys); return })
		if err := g.Wait(); err != nil {
			return out, err
		}
		for _, k := range keys {
			ps := paramSummary(entries[k])
			ps.Clicks, ps.Values, ps.Links = stats[k].Clicks, stats[k].Values, links[k]
			out.Items = append(out.Items, ps)
		}
		sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].Clicks > out.Items[j].Clicks })
		return out, nil
	})
}

// Param: giá trị của 1 tham số + chỉ số, timeline top 5, cảnh báo dữ liệu.
func (s *Service) Param(ctx context.Context, q Query, key string, pg Paging) (gen.ParamReport, error) {
	e, err := s.st.RegistryEntry(ctx, key)
	if err != nil {
		return gen.ParamReport{}, err
	}
	if e == nil || !e.Tracked {
		return gen.ParamReport{}, domain.NotFound("param_not_tracked", "tham số chưa được theo dõi")
	}
	if e.PII != "" && e.PII != "none" {
		return gen.ParamReport{}, domain.Forbidden("pii_param", "tham số PII không có báo cáo theo giá trị")
	}
	return cached(s, ctx, "param", q, []any{key, pg}, func(ctx context.Context) (gen.ParamReport, error) {
		var (
			cur, prev []store.ParamValueStat
			active    map[string]int64
			linkCount map[string]int64
		)
		g, gctx := group(ctx)
		g.Go(func() (err error) { cur, err = s.st.ParamValueStats(gctx, q.M, key); return })
		g.Go(func() (err error) { active, err = s.st.ParamValueActiveLinks(gctx, q.M, key); return })
		g.Go(func() (err error) { linkCount, err = s.st.ParamLinkCounts(gctx, q.M, []string{key}); return })
		if q.Comp {
			g.Go(func() (err error) { prev, err = s.st.ParamValueStats(gctx, q.Prev, key); return })
		}
		if err := g.Wait(); err != nil {
			return gen.ParamReport{}, err
		}
		pm := map[string]int64{}
		for _, p := range prev {
			pm[p.Value] = p.Clicks
		}
		var total int64
		for _, c := range cur {
			total += c.Clicks
		}
		sum := paramSummary(*e)
		sum.Links = linkCount[key]
		rows := make([]gen.ParamValueRow, 0, len(cur))
		needle := strings.ToLower(strings.TrimSpace(pg.Q))
		for _, c := range cur {
			if c.Clicks > 0 {
				sum.Values++
				sum.Clicks += c.Clicks
			}
			if c.Clicks == 0 && c.Bot == 0 && c.NewLinks == 0 {
				continue
			}
			if needle != "" && !strings.Contains(strings.ToLower(c.Value), needle) {
				continue
			}
			a := active[c.Value]
			row := gen.ParamValueRow{
				Value: c.Value, Links: a, Accounts: c.Accounts, Campaigns: c.Campaigns, Share: ratio(c.Clicks, total),
				Metrics: toMetrics(store.Sums{Clicks: c.Clicks, Unique: c.Unique, Bot: c.Bot, Susp: c.Susp, NewLinks: c.NewLinks}, c.NewLinks, a, nil),
			}
			if q.Comp {
				row.ChangeClicks = change(float64(c.Clicks), float64(pm[c.Value]))
			}
			rows = append(rows, row)
		}
		by := pg.Sort
		if by == "" {
			by = "clicks"
		}
		sort.SliceStable(rows, func(i, j int) bool {
			switch by {
			case "value":
				return less(rows[i].Value < rows[j].Value, rows[i].Value > rows[j].Value, pg.Desc)
			case "links", "accounts", "campaigns":
				pick := map[string]func(gen.ParamValueRow) int64{
					"links": func(r gen.ParamValueRow) int64 { return r.Links }, "accounts": func(r gen.ParamValueRow) int64 { return r.Accounts },
					"campaigns": func(r gen.ParamValueRow) int64 { return r.Campaigns },
				}[by]
				a, b := pick(rows[i]), pick(rows[j])
				if a != b {
					return less(a < b, a > b, pg.Desc)
				}
			default:
				a, _ := metricValue(rows[i].Metrics, by, rows[i].ChangeClicks)
				b, _ := metricValue(rows[j].Metrics, by, rows[j].ChangeClicks)
				if a != b {
					return less(a < b, a > b, pg.Desc)
				}
			}
			return rows[i].Value < rows[j].Value
		})

		// Timeline top 5 theo clicks (không phụ thuộc cách sort bảng).
		byClicks := append([]store.ParamValueStat(nil), cur...)
		sort.SliceStable(byClicks, func(i, j int) bool { return byClicks[i].Clicks > byClicks[j].Clicks })
		var top []string
		for _, c := range byClicks {
			if len(top) < 5 && c.Clicks > 0 {
				top = append(top, c.Value)
			}
		}
		series, err := s.st.ParamValueSeries(ctx, q.M, key, top, q.Gran)
		if err != nil {
			return gen.ParamReport{}, err
		}
		buckets := Buckets(q.M.From, q.M.To, q.Gran)
		out := gen.ParamReport{Param: sum, Warnings: []gen.Alert{}}
		for _, v := range top {
			pts := make([]gen.SeriesPoint, len(buckets))
			for i, b := range buckets {
				sm := series[v][b.Format("2006-01-02")]
				pts[i] = gen.SeriesPoint{Period: openapi_types.Date{Time: b}, Clicks: sm.Clicks, UniqueClicks: sm.Unique, BotClicks: sm.Bot, NewLinks: sm.NewLinks}
			}
			out.TopSeries = append(out.TopSeries, struct {
				Points []gen.SeriesPoint `json:"points"`
				Value  string            `json:"value"`
			}{Points: pts, Value: v})
		}
		if out.TopSeries == nil {
			out.TopSeries = []struct {
				Points []gen.SeriesPoint `json:"points"`
				Value  string            `json:"value"`
			}{}
		}
		if e.Status == "high_cardinality" {
			out.Warnings = append(out.Warnings, gen.Alert{Level: gen.AlertLevel("warning"), Code: "high_cardinality",
				Message: "Tham số có quá nhiều giá trị — số liệu thống kê chỉ giữ top giá trị + \"other\"; lọc chính xác vẫn dùng được ở Click Explorer"})
		}
		if sum.TrackedSince != nil && sum.TrackedSince.After(q.M.From) {
			out.Warnings = append(out.Warnings, gen.Alert{Level: gen.AlertLevel("info"), Code: "tracked_since",
				Message: "Số liệu tham số này chỉ có từ " + sum.TrackedSince.Format("02/01/2006") + " (ngày bắt đầu theo dõi / backfill)"})
		}
		start, end := pg.bounds(len(rows))
		out.Rows, out.Total, out.Page, out.PageSize = rows[start:end], len(rows), max(pg.Page, 1), end-start
		return out, nil
	})
}
