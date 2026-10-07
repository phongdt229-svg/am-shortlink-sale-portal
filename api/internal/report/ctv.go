package report

import (
	"context"
	"fmt"
	"sort"
	"strings"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/scope"
	"am-shortlink-portal/api/internal/store"
)

var errCTVNotFound = domain.NotFound("ctv_not_found", "không tìm thấy CTV trong phạm vi được xem")

// CTVs: R4 bảng xếp hạng (bỏ dòng chưa định danh — có màn hình riêng).
func (s *Service) CTVs(ctx context.Context, q Query, pg Paging) (gen.CTVRankPage, error) {
	return cached(s, ctx, "ctvs", q, pg, func(ctx context.Context) (gen.CTVRankPage, error) {
		rows, err := s.dimRows(ctx, q, store.DimCTV)
		if err != nil {
			return gen.CTVRankPage{}, err
		}
		list := make([]*row, 0, len(rows))
		for _, r := range rows {
			if r.key != "" && (r.sums.Clicks > 0 || r.sums.NewLinks > 0 || r.sums.Bot > 0) {
				list = append(list, r)
			}
		}
		// Hạng luôn theo lượt click (giảm dần), độc lập cách người dùng sắp xếp bảng.
		sort.Slice(list, func(i, j int) bool {
			if list[i].sums.Clicks != list[j].sums.Clicks {
				return list[i].sums.Clicks > list[j].sums.Clicks
			}
			return list[i].key < list[j].key
		})
		ids := make([]string, len(list))
		for i, r := range list {
			ids[i] = r.key
		}
		names, err := s.st.CTVNames(ctx, ids)
		if err != nil {
			return gen.CTVRankPage{}, err
		}
		items := make([]gen.CTVRankRow, 0, len(list))
		needle := strings.ToLower(strings.TrimSpace(pg.Q))
		norm := mask.NormalizePhone(pg.Q)
		var kept []*row
		for i, r := range list {
			if needle != "" && !strings.Contains(r.key, norm) && !strings.Contains(strings.ToLower(names[r.key]), needle) {
				continue
			}
			kept = append(kept, r)
			it := gen.CTVRankRow{CtvRef: s.ctvRef(r.key), CtvDisplay: q.CTVDisplay(r.key), Rank: i + 1, Metrics: r.metrics()}
			if nm := names[r.key]; nm != "" {
				it.Name = &nm
			}
			items = append(items, it)
		}
		if pg.Sort != "" && pg.Sort != "rank" {
			sort.SliceStable(items, func(i, j int) bool {
				a, _ := metricValue(items[i].Metrics, pg.Sort, nil)
				b, _ := metricValue(items[j].Metrics, pg.Sort, nil)
				if a == b {
					return items[i].Rank < items[j].Rank
				}
				return less(a < b, a > b, pg.Desc)
			})
		}
		start, end := pg.bounds(len(items))
		return gen.CTVRankPage{Items: items[start:end], Total: len(items), Page: max(pg.Page, 1), PageSize: end - start, Totals: sumRows(kept)}, nil
	})
}

// CTV: R4 chi tiết theo ctv_ref (không đưa SĐT lên URL).
func (s *Service) CTV(ctx context.Context, q Query, ref string) (gen.CTVReport, error) {
	id, err := s.masker.ParseCTVRef(ref)
	if err != nil {
		return gen.CTVReport{}, errCTVNotFound
	}
	groups, err := s.st.LinksByCTV(ctx, q.M, id)
	if err != nil {
		return gen.CTVReport{}, err
	}
	if len(groups) == 0 {
		return gen.CTVReport{}, errCTVNotFound
	}
	q.M.CTVs = []string{id}
	if q.Comp {
		q.Prev.CTVs = []string{id}
	}
	return cached(s, ctx, "ctv", q, id, func(ctx context.Context) (gen.CTVReport, error) {
		out := gen.CTVReport{CtvRef: ref, CtvDisplay: q.CTVDisplay(id)}
		owners := make([]string, len(groups))
		for i, g := range groups {
			owners[i] = g.Owner
		}
		sort.Strings(owners)
		out.Owners = &owners
		if names, err := s.st.CTVNames(ctx, []string{id}); err == nil && names[id] != "" {
			nm := names[id]
			out.Name = &nm
		}
		g, gctx := group(ctx)
		g.Go(func() (err error) { out.Summary, err = s.Summary(gctx, q); return })
		g.Go(func() (err error) {
			out.Campaigns, _, err = s.campaignRows(gctx, q)
			sortCampaigns(out.Campaigns, "clicks", true)
			return
		})
		g.Go(func() error {
			page, err := s.topLinks(gctx, q, Paging{Page: 1, PageSize: 100})
			out.Links = page.Items
			return err
		})
		if err := g.Wait(); err != nil {
			return out, err
		}
		out.Alerts = ctvAlerts(out.Summary)
		return out, nil
	})
}

// ctvAlerts: cảnh báo bất thường để soát gian lận CTV.
func ctvAlerts(sum gen.ReportSummary) []gen.Alert {
	m := sum.Kpis.Current
	alerts := []gen.Alert{}
	add := func(level gen.AlertLevel, code, msg string) {
		alerts = append(alerts, gen.Alert{Level: level, Code: code, Message: msg})
	}
	if m.Clicks >= 20 {
		if r := ratio(m.SuspiciousClicks, m.Clicks); r >= 0.15 {
			add(gen.AlertLevel("critical"), "suspicious_high", fmt.Sprintf("%.1f%% lượt click là nghi vấn (IP vượt ngưỡng)", r*100))
		} else if r >= 0.05 {
			add(gen.AlertLevel("warning"), "suspicious_elevated", fmt.Sprintf("%.1f%% lượt click là nghi vấn", r*100))
		}
		if r := 1 - ratio(m.UniqueClicks, m.Clicks); r >= 0.4 {
			add(gen.AlertLevel("warning"), "repeat_high", fmt.Sprintf("%.0f%% lượt click là khách lặp lại trong ngày", r*100))
		}
	}
	if total := m.Clicks + m.BotClicks; total >= 20 {
		if r := ratio(m.BotClicks, total); r >= 0.1 {
			add(gen.AlertLevel("warning"), "bot_high", fmt.Sprintf("%.1f%% lượt truy cập là bot", r*100))
		}
	}
	// Đột biến: ngày cao nhất > 5 × trung vị các ngày có click.
	var vals []int64
	var peak gen.SeriesPoint
	for _, p := range sum.Series.Points {
		if p.Clicks > 0 {
			vals = append(vals, p.Clicks)
		}
		if p.Clicks > peak.Clicks {
			peak = p
		}
	}
	if len(vals) >= 5 && sum.Series.Granularity == gen.Granularity("day") {
		sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
		if med := vals[len(vals)/2]; med > 0 && peak.Clicks > 5*med && peak.Clicks >= 30 {
			add(gen.AlertLevel("warning"), "spike", fmt.Sprintf("Đột biến ngày %s: %d click (trung vị %d/ngày)", peak.Period.Format("02/01/2006"), peak.Clicks, med))
		}
	}
	return alerts
}

// Lookup: tra cứu CTV theo SĐT / hash / ctv_ref — khớp chính xác qua index ctv_id.
func (s *Service) Lookup(ctx context.Context, p domain.Principal, input string) (gen.CTVLookup, error) {
	q := Query{P: p, admin: p.IsAdmin()}
	s.pii(ctx)
	owners, all, _ := scope.For(p).Resolve(nil)
	m := store.Match{Owners: owners, AllOwners: all}
	id := mask.NormalizePhone(input)
	if ref, err := s.masker.ParseCTVRef(strings.TrimSpace(input)); err == nil {
		id = ref
	}
	out := gen.CTVLookup{Query: input, Normalized: q.CTVDisplay(id), Items: []gen.CTVLookupItem{}}
	groups, err := s.st.LinksByCTV(ctx, m, id)
	if err != nil {
		return out, err
	}
	var name *string
	if names, err := s.st.CTVNames(ctx, []string{id}); err == nil && names[id] != "" {
		nm := names[id]
		name = &nm
	}
	for _, g := range groups {
		samples := make([]gen.LinkInfo, len(g.Sample))
		for i, l := range g.Sample {
			samples[i] = s.linkInfo(q, l)
		}
		out.Items = append(out.Items, gen.CTVLookupItem{
			CtvRef: s.ctvRef(id), CtvDisplay: q.CTVDisplay(id), Name: name, Owner: g.Owner,
			Links: g.Links, ClicksTotal: g.ClicksTotal, FirstLinkAt: g.First, LastLinkAt: g.Last, SampleLinks: &samples,
		})
	}
	return out, nil
}

// Unidentified: link / click chưa định danh CTV (admin, chính chủ).
func (s *Service) Unidentified(ctx context.Context, q Query, pg Paging) (gen.UnidentifiedReport, error) {
	if q.P.Role == domain.RoleViewer {
		return gen.UnidentifiedReport{}, domain.Forbidden("forbidden_role", "chỉ admin hoặc chủ tài khoản xem được danh sách này")
	}
	return cached(s, ctx, "unidentified", q, pg, func(ctx context.Context) (gen.UnidentifiedReport, error) {
		um := q.M
		um.CTVs = []string{""}
		var (
			cur, all gen.Metrics
			links    []store.LinkInfo
			total    int64
		)
		size := pg.PageSize
		if size <= 0 {
			size = 50
		}
		skip := int64((max(pg.Page, 1) - 1) * size)
		g, gctx := group(ctx)
		g.Go(func() (err error) { cur, err = s.metrics(gctx, um); return })
		g.Go(func() (err error) { all, err = s.metrics(gctx, q.M); return })
		g.Go(func() (err error) {
			links, total, err = s.st.FindLinks(gctx, q.M, store.UnidentifiedFilter(), skip, int64(size))
			return
		})
		if err := g.Wait(); err != nil {
			return gen.UnidentifiedReport{}, err
		}
		ids := make([]int64, len(links))
		for i, l := range links {
			ids[i] = l.ID
		}
		lm := um
		lm.LinkIDs = ids
		var stats []store.GroupRow
		if len(ids) > 0 {
			var err error
			if stats, err = s.st.GroupBy(ctx, store.LevelLink, lm, store.DimLink, store.GroupOpts{}); err != nil {
				return gen.UnidentifiedReport{}, err
			}
		}
		byID := map[int64]store.GroupRow{}
		for _, r := range stats {
			byID[r.LinkID] = r
		}
		items := make([]gen.LinkRow, len(links))
		for i, l := range links {
			r := byID[l.ID]
			items[i] = gen.LinkRow{Link: s.linkInfo(q, l), Clicks: r.Sums.Clicks, UniqueClicks: r.Sums.Unique, SuspiciousClicks: ptr(r.Sums.Susp)}
			if !r.LastClick.IsZero() {
				items[i].LastClickDate = &openapi_types.Date{Time: r.LastClick}
			}
		}
		share := ratio(cur.Clicks, all.Clicks)
		return gen.UnidentifiedReport{
			Metrics: cur, ShareOfClicks: &share,
			Links: gen.LinkRowPage{Items: items, Total: int(total), Page: max(pg.Page, 1), PageSize: len(items)},
		}, nil
	})
}
