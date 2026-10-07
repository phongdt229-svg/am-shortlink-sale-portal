package report

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/store"
)

// MaxStage1Rows: giới hạn nhóm trung gian (chiều × link) — vượt → yêu cầu thu hẹp.
const MaxStage1Rows = 300_000

var knownDims = map[string]bool{
	"account": true, "campaign": true, "ctv": true, "link": true, "link_prefix": true, "access_prefix": true,
	"api_version": true, "dest_host": true, "device": true, "os": true, "browser": true, "source_group": true,
	"referer_host": true, "country": true, "province": true, "day": true, "week": true, "month": true,
	"hour": true, "weekday": true,
}

var timeDims = map[string]bool{"day": true, "week": true, "month": true}

// filtersEmpty: không có tiêu chí chi tiết → đọc được từ stats (≤ 12 tháng).
func filtersEmpty(f *gen.ClickFilters) bool {
	if f == nil {
		return true
	}
	return compactCriteria(*f) == 0
}

func compactCriteria(f gen.ClickFilters) int {
	n := 0
	for _, c := range []*gen.Criterion{f.Links, f.AccessPrefix, f.ApiVersion, f.DestHost, f.Device, f.Os, f.Browser, f.SourceGroup, f.RefererHost, f.Country, f.Province, f.Ip} {
		if c != nil && len(c.Values) > 0 {
			n++
		}
	}
	if f.Weekdays != nil && len(*f.Weekdays) > 0 {
		n++
	}
	if f.HourFrom != nil || f.HourTo != nil || f.IsCustom != nil || f.LinkCreatedFrom != nil || f.LinkCreatedTo != nil {
		n++
	}
	if f.Params != nil && len(*f.Params) > 0 {
		n++
	}
	if f.Quality != nil && *f.Quality != "all" {
		n++
	}
	if f.Visit != nil && *f.Visit != "all" {
		n++
	}
	if f.CampaignPresence != nil && *f.CampaignPresence != "any" {
		n++
	}
	if f.CtvPresence != nil && *f.CtvPresence != "any" {
		n++
	}
	return n
}

type explorerPlan struct {
	dims      []string
	paramKeys []string
	useStats  bool
}

func (s *Service) planExplorer(ctx context.Context, q Query, body gen.ExplorerQuery) (explorerPlan, error) {
	p := explorerPlan{}
	seen := map[string]bool{}
	for _, d := range body.GroupBy {
		d = strings.TrimSpace(d)
		if seen[d] {
			return p, domain.BadRequest("invalid_group_by", "chiều nhóm bị lặp: "+d)
		}
		seen[d] = true
		if key, ok := strings.CutPrefix(d, "param."); ok {
			if key == "" || s.pii(ctx)[key] != "" {
				return p, domain.BadRequest("invalid_group_by", "không nhóm được theo tham số PII / rỗng: "+d)
			}
			p.paramKeys = append(p.paramKeys, key)
		} else if !knownDims[d] {
			return p, domain.BadRequest("invalid_group_by", "chiều không hỗ trợ: "+d)
		}
		p.dims = append(p.dims, d)
	}
	if len(p.dims) == 0 || len(p.dims) > 3 {
		return p, domain.BadRequest("invalid_group_by", "chọn 1–3 chiều nhóm")
	}
	p.useStats = filtersEmpty(body.Filters)
	for _, d := range p.dims {
		if !store.StatsDims[d] && !strings.HasPrefix(d, "param.") {
			p.useStats = false
		}
	}
	if !p.useStats && days(q.M.From, q.M.To) > MaxRawDays {
		return p, domain.BadRequest("range_too_long", "lọc / nhóm theo tiêu chí chi tiết chỉ trong 3 tháng — hãy thu hẹp khoảng ngày hoặc chỉ nhóm theo tài khoản / chiến dịch / CTV / link / thời gian")
	}
	return p, nil
}

func (s *Service) stage1(ctx context.Context, q Query, plan explorerPlan, f *gen.ClickFilters, m store.Match) ([]store.Stage1Row, error) {
	var rows []store.Stage1Row
	var err error
	if plan.useStats {
		rows, err = s.st.ExplorerStats(ctx, m, plan.dims, MaxStage1Rows)
	} else {
		qq := q
		qq.M = m
		cq, cerr := s.ClickQuery(ctx, qq, f)
		if cerr != nil {
			return nil, cerr
		}
		rows, err = s.st.ExplorerClicks(ctx, cq, plan.dims, MaxStage1Rows)
	}
	if errors.Is(err, store.ErrTooManyGroups) {
		return nil, domain.Unprocessable("too_many_groups", "quá nhiều nhóm — hãy thu hẹp khoảng ngày / bộ lọc hoặc bớt chiều nhóm")
	}
	return rows, err
}

type agg struct {
	keys  []string
	sums  store.Sums
	links map[int64]bool
	first time.Time
	last  time.Time
}

// stage2: gom bước 1 theo bộ khoá cuối (thay link bằng tham số URL nếu cần), đếm link có click phân biệt.
func (s *Service) stage2(ctx context.Context, plan explorerPlan, rows []store.Stage1Row) (map[string]*agg, error) {
	paramVals := map[string]map[int64]string{}
	if len(plan.paramKeys) > 0 {
		ids := make([]int64, 0, len(rows))
		seen := map[int64]bool{}
		for _, r := range rows {
			if !seen[r.LinkID] {
				seen[r.LinkID] = true
				ids = append(ids, r.LinkID)
			}
		}
		for _, k := range plan.paramKeys {
			m, err := s.st.LinkParamValues(ctx, ids, k)
			if err != nil {
				return nil, err
			}
			paramVals[k] = m
		}
	}
	out := map[string]*agg{}
	for _, r := range rows {
		keys := make([]string, len(plan.dims))
		for i, d := range plan.dims {
			switch {
			case d == "link":
				keys[i] = strconv.FormatInt(r.LinkID, 10)
			case strings.HasPrefix(d, "param."):
				keys[i] = paramVals[strings.TrimPrefix(d, "param.")][r.LinkID]
			default:
				keys[i] = r.Keys[d]
			}
		}
		k := strings.Join(keys, "\x1f")
		a, ok := out[k]
		if !ok {
			a = &agg{keys: keys, links: map[int64]bool{}}
			out[k] = a
		}
		a.sums.Add(r.Sums)
		if r.Sums.Clicks > 0 {
			a.links[r.LinkID] = true
		}
		if !r.First.IsZero() && (a.first.IsZero() || r.First.Before(a.first)) {
			a.first = r.First
		}
		if r.Last.After(a.last) {
			a.last = r.Last
		}
	}
	return out, nil
}

func explorerTotals(rows []store.Stage1Row) gen.ExplorerTotals {
	var sums store.Sums
	links, owners, ctvs := map[int64]bool{}, map[string]bool{}, map[string]bool{}
	var first, last time.Time
	for _, r := range rows {
		sums.Add(r.Sums)
		if r.Sums.Clicks > 0 {
			links[r.LinkID] = true
			owners[r.Owner] = true
			if r.CTV != "" {
				ctvs[r.CTV] = true
			}
		}
		if !r.First.IsZero() && (first.IsZero() || r.First.Before(first)) {
			first = r.First
		}
		if r.Last.After(last) {
			last = r.Last
		}
	}
	return gen.ExplorerTotals{Metrics: explorerMetrics(sums, int64(len(links)), first, last), Accounts: int64(len(owners)), Ctvs: int64(len(ctvs))}
}

func explorerMetrics(s store.Sums, active int64, first, last time.Time) gen.ExplorerMetrics {
	m := gen.ExplorerMetrics{
		Clicks: s.Clicks, UniqueClicks: s.Unique, BotClicks: s.Bot, SuspiciousClicks: s.Susp,
		ActiveLinks: active, ClicksPerLink: ratio(s.Clicks, active),
	}
	if !first.IsZero() {
		m.FirstClickAt = &first
	}
	if !last.IsZero() {
		m.LastClickAt = &last
	}
	return m
}

func sortValue(m gen.ExplorerMetrics, by string) float64 {
	switch by {
	case "unique_clicks":
		return float64(m.UniqueClicks)
	case "bot_clicks":
		return float64(m.BotClicks)
	case "suspicious_clicks":
		return float64(m.SuspiciousClicks)
	case "active_links":
		return float64(m.ActiveLinks)
	case "clicks_per_link":
		return m.ClicksPerLink
	default:
		return float64(m.Clicks)
	}
}

var weekdayLabel = []string{"Chủ nhật", "Thứ 2", "Thứ 3", "Thứ 4", "Thứ 5", "Thứ 6", "Thứ 7"}

// Explorer: R6b.
func (s *Service) Explorer(ctx context.Context, q Query, body gen.ExplorerQuery) (gen.ExplorerResult, error) {
	plan, err := s.planExplorer(ctx, q, body)
	if err != nil {
		return gen.ExplorerResult{}, err
	}
	return cached(s, ctx, "explorer", q, body, func(ctx context.Context) (gen.ExplorerResult, error) {
		var cur, prev []store.Stage1Row
		g, gctx := group(ctx)
		g.Go(func() (err error) { cur, err = s.stage1(gctx, q, plan, body.Filters, q.M); return })
		if q.Comp {
			g.Go(func() (err error) { prev, err = s.stage1(gctx, q, plan, body.Filters, q.Prev); return })
		}
		if err := g.Wait(); err != nil {
			return gen.ExplorerResult{}, err
		}
		groups, err := s.stage2(ctx, plan, cur)
		if err != nil {
			return gen.ExplorerResult{}, err
		}
		totals := explorerTotals(cur)
		out := gen.ExplorerResult{
			Period: period(q.M), GroupBy: plan.dims, Totals: totals,
			Source: gen.ExplorerResultSource(map[bool]string{true: "stats", false: "clicks"}[plan.useStats]),
		}
		var prevGroups map[string]*agg
		hasTime := slices.ContainsFunc(plan.dims, func(d string) bool { return timeDims[d] })
		if q.Comp {
			pt := explorerTotals(prev)
			out.PreviousTotals = &pt
			if !hasTime { // nhóm theo thời gian: kỳ trước khác khoá → chỉ so tổng
				if prevGroups, err = s.stage2(ctx, plan, prev); err != nil {
					return gen.ExplorerResult{}, err
				}
			}
		}

		list := make([]*agg, 0, len(groups))
		for _, a := range groups {
			list = append(list, a)
		}
		by := "clicks"
		if body.SortBy != nil {
			by = string(*body.SortBy)
		}
		desc := body.Order == nil || *body.Order == gen.SortOrder("desc")
		if by == "key" && body.Order == nil {
			desc = false
		}
		sort.SliceStable(list, func(i, j int) bool {
			if by == "key" {
				a, b := strings.Join(list[i].keys, "\x1f"), strings.Join(list[j].keys, "\x1f")
				return less(a < b, a > b, desc)
			}
			mi := explorerMetrics(list[i].sums, int64(len(list[i].links)), time.Time{}, time.Time{})
			mj := explorerMetrics(list[j].sums, int64(len(list[j].links)), time.Time{}, time.Time{})
			a, b := sortValue(mi, by), sortValue(mj, by)
			if a == b {
				return strings.Join(list[i].keys, "\x1f") < strings.Join(list[j].keys, "\x1f")
			}
			return less(a < b, a > b, desc)
		})
		limit := 100
		if body.Limit != nil {
			limit = *body.Limit
		}
		n := len(list)
		out.GroupCount = &n
		head := list
		if len(list) > limit {
			head = list[:limit]
			out.Truncated = true
			var other agg
			other.links = map[int64]bool{}
			for _, a := range list[limit:] {
				other.sums.Add(a.sums)
				for l := range a.links {
					other.links[l] = true
				}
			}
			row := gen.ExplorerRow{
				Keys:    []gen.ExplorerKey{{Dim: "other", Value: "", Label: "Khác (" + strconv.Itoa(len(list)-limit) + " nhóm)"}},
				Metrics: explorerMetrics(other.sums, int64(len(other.links)), time.Time{}, time.Time{}),
				Share:   ratio(other.sums.Clicks, totals.Metrics.Clicks),
			}
			out.Other = &row
		}
		labels, err := s.explorerLabels(ctx, q, plan.dims, head)
		if err != nil {
			return gen.ExplorerResult{}, err
		}
		out.Rows = make([]gen.ExplorerRow, len(head))
		for i, a := range head {
			row := gen.ExplorerRow{
				Keys:    make([]gen.ExplorerKey, len(plan.dims)),
				Metrics: explorerMetrics(a.sums, int64(len(a.links)), a.first, a.last),
				Share:   ratio(a.sums.Clicks, totals.Metrics.Clicks),
			}
			for j, d := range plan.dims {
				row.Keys[j] = labels(d, a.keys[j])
			}
			if prevGroups != nil {
				var pc int64
				if p, ok := prevGroups[strings.Join(a.keys, "\x1f")]; ok {
					pc = p.sums.Clicks
				}
				row.PreviousClicks = &pc
				row.ChangeClicks = change(float64(a.sums.Clicks), float64(pc))
			}
			out.Rows[i] = row
		}
		return out, nil
	})
}

// explorerLabels: giá trị lọc + nhãn hiển thị (che SĐT, mã link, tên chiến dịch, thứ…).
func (s *Service) explorerLabels(ctx context.Context, q Query, dims []string, head []*agg) (func(dim, v string) gen.ExplorerKey, error) {
	linkIDs := []int64{}
	codes := []string{}
	for _, a := range head {
		for j, d := range dims {
			switch d {
			case "link":
				id, _ := strconv.ParseInt(a.keys[j], 10, 64)
				linkIDs = append(linkIDs, id)
			case "campaign":
				codes = append(codes, a.keys[j])
			}
		}
	}
	links, err := s.st.LinksByIDs(ctx, q.M, linkIDs)
	if err != nil {
		return nil, err
	}
	names, err := s.st.CampaignNames(ctx, codes)
	if err != nil {
		return nil, err
	}
	return func(dim, v string) gen.ExplorerKey {
		k := gen.ExplorerKey{Dim: dim, Value: v, Label: v}
		switch {
		case dim == "ctv":
			k.Label = q.CTVDisplay(v)
			if v == "" {
				k.Value = "-"
			} else {
				k.Value = s.ctvRef(v)
			}
		case dim == "campaign":
			if v == "" {
				k.Value, k.Label = "-", "(không gắn chiến dịch)"
			} else if n := names[v]; n != "" && n != v {
				k.Label = v + " · " + n
			}
		case dim == "link":
			id, _ := strconv.ParseInt(v, 10, 64)
			if l, ok := links[id]; ok {
				k.Value, k.Label = l.Code, "/"+l.Prefix+"/"+l.Code
			}
		case dim == "weekday":
			if i, err := strconv.Atoi(v); err == nil && i >= 0 && i < 7 {
				k.Label = weekdayLabel[i]
			}
		case dim == "hour":
			if h, err := strconv.Atoi(v); err == nil {
				k.Label = strconv.Itoa(h) + "h"
			}
		case dim == "link_prefix" || dim == "access_prefix":
			k.Label = "/" + v
		case strings.HasPrefix(dim, "param.") && v == "":
			k.Label = "(không có)"
		case v == "":
			k.Label = "(không rõ)"
		}
		return k
	}, nil
}

// ---------- facets ----------

func (s *Service) Facets(ctx context.Context, q Query, field, needle string, limit int) (gen.FacetList, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	out := gen.FacetList{Field: field, Items: []gen.FacetItem{}}
	var facets []store.Facet
	var err error
	if key, ok := strings.CutPrefix(field, "param."); ok {
		if s.pii(ctx)[key] != "" {
			return out, domain.Forbidden("pii_param", "không liệt kê giá trị tham số PII")
		}
		facets, err = s.st.ParamFacets(ctx, q.M, key, needle, int64(limit))
	} else {
		if field == "api_version" {
			field = "link_api_version"
		}
		if !store.FacetFields[field] {
			return out, domain.BadRequest("invalid_field", "trường không hỗ trợ: "+field)
		}
		facets, err = s.st.ClickFacets(ctx, q.M, field, needle, int64(limit))
	}
	if err != nil {
		return out, err
	}
	for _, f := range facets {
		label := f.Value
		if label == "" {
			label = "(không rõ)"
		}
		out.Items = append(out.Items, gen.FacetItem{Value: f.Value, Label: label, Clicks: f.Clicks})
	}
	return out, nil
}
