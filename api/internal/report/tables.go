package report

import (
	"context"
	"sort"
	"strings"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/scope"
	"am-shortlink-portal/api/internal/store"
)

// ---------- Top ----------

// TopAccounts / TopCampaigns / TopCTVs / TopLinks: top N theo clicks trong phạm vi.
func (s *Service) TopAccounts(ctx context.Context, q Query, n int64) ([]gen.TopItem, error) {
	rows, err := s.st.GroupBy(ctx, store.ChooseLevel(q.M, store.DimOwner, false), q.M, store.DimOwner, store.GroupOpts{SortBy: "clicks", Limit: n})
	if err != nil {
		return nil, err
	}
	out := []gen.TopItem{}
	for _, r := range rows {
		if r.Sums.Clicks > 0 {
			out = append(out, gen.TopItem{Key: r.Key, Label: r.Key, Clicks: r.Sums.Clicks, UniqueClicks: r.Sums.Unique, SuspiciousClicks: &r.Sums.Susp})
		}
	}
	return out, nil
}

func (s *Service) TopCampaigns(ctx context.Context, q Query, n int64) ([]gen.TopItem, error) {
	rows, err := s.st.GroupBy(ctx, store.ChooseLevel(q.M, store.DimCampaign, false), q.M, store.DimCampaign, store.GroupOpts{SortBy: "clicks", Limit: n + 1})
	if err != nil {
		return nil, err
	}
	codes := []string{}
	for _, r := range rows {
		codes = append(codes, r.Key)
	}
	names, err := s.st.CampaignNames(ctx, codes)
	if err != nil {
		return nil, err
	}
	out := []gen.TopItem{}
	for _, r := range rows {
		if r.Key == "" || r.Sums.Clicks == 0 || int64(len(out)) >= n {
			continue // link không gắn chiến dịch không vào top
		}
		out = append(out, gen.TopItem{Key: r.Key, Label: names[r.Key], Clicks: r.Sums.Clicks, UniqueClicks: r.Sums.Unique, SuspiciousClicks: &r.Sums.Susp})
	}
	return out, nil
}

func (s *Service) TopCTVs(ctx context.Context, q Query, n int64) ([]gen.TopItem, error) {
	rows, err := s.st.GroupBy(ctx, store.ChooseLevel(q.M, store.DimCTV, false), q.M, store.DimCTV, store.GroupOpts{SortBy: "clicks", Limit: n + 1})
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, r.Key)
	}
	names, err := s.st.CTVNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := []gen.TopItem{}
	for _, r := range rows {
		if r.Key == "" || r.Sums.Clicks == 0 || int64(len(out)) >= n {
			continue
		}
		label := q.CTVDisplay(r.Key)
		if nm := names[r.Key]; nm != "" {
			label += " · " + nm
		}
		owner := r.AnyOwner
		out = append(out, gen.TopItem{Key: s.ctvRef(r.Key), Label: label, Clicks: r.Sums.Clicks, UniqueClicks: r.Sums.Unique, SuspiciousClicks: &r.Sums.Susp, Owner: &owner})
	}
	return out, nil
}

func (s *Service) TopLinks(ctx context.Context, q Query, n int64) ([]gen.TopItem, error) {
	rows, err := s.st.GroupBy(ctx, store.LevelLink, q.M, store.DimLink, store.GroupOpts{SortBy: "clicks", Limit: n})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.LinkID
	}
	links, err := s.st.LinksByIDs(ctx, q.M, ids)
	if err != nil {
		return nil, err
	}
	out := []gen.TopItem{}
	for _, r := range rows {
		l, ok := links[r.LinkID]
		if !ok || r.Sums.Clicks == 0 {
			continue
		}
		owner := l.Owner
		out = append(out, gen.TopItem{Key: l.Code, Label: "/" + l.Prefix + "/" + l.Code, Clicks: r.Sums.Clicks, UniqueClicks: r.Sums.Unique, SuspiciousClicks: &r.Sums.Susp, Owner: &owner})
	}
	return out, nil
}

// Overview: R1.
func (s *Service) Overview(ctx context.Context, q Query) (gen.OverviewReport, error) {
	return cached(s, ctx, "overview", q, nil, func(ctx context.Context) (gen.OverviewReport, error) { return s.overview(ctx, q) })
}

func (s *Service) overview(ctx context.Context, q Query) (gen.OverviewReport, error) {
	out := gen.OverviewReport{TopAccounts: []gen.TopItem{}}
	g, gctx := group(ctx)
	g.Go(func() (err error) { out.Summary, err = s.Summary(gctx, q); return })
	if q.M.AllOwners || len(q.M.Owners) > 1 {
		g.Go(func() (err error) { out.TopAccounts, err = s.TopAccounts(gctx, q, 10); return })
	}
	g.Go(func() (err error) { out.TopCampaigns, err = s.TopCampaigns(gctx, q, 10); return })
	g.Go(func() (err error) { out.TopCtvs, err = s.TopCTVs(gctx, q, 10); return })
	g.Go(func() (err error) { out.TopLinks, err = s.TopLinks(gctx, q, 10); return })
	return out, g.Wait()
}

// ---------- Bảng theo chiều ----------

type row struct {
	key       string
	sums      store.Sums
	active    int64
	total     int64
	created   int64
	prevClick int64
	first     string
	last      string
	ctvCount  int64
}

// dimRows: tổng hợp chỉ số đầy đủ theo chiều d (owner / campaign / ctv) cho kỳ hiện tại (+ clicks kỳ trước).
func (s *Service) dimRows(ctx context.Context, q Query, d store.Dim) (map[string]*row, error) {
	lvl := store.ChooseLevel(q.M, d, false)
	var (
		cur, prev      []store.GroupRow
		active         map[string]int64
		total, created map[string]int64
	)
	g, gctx := group(ctx)
	g.Go(func() (err error) { cur, err = s.st.GroupBy(gctx, lvl, q.M, d, store.GroupOpts{}); return })
	g.Go(func() (err error) { active, err = s.st.ActiveBy(gctx, q.M, d); return })
	g.Go(func() (err error) { total, created, err = s.st.TotalLinksBy(gctx, q.M, d); return })
	if q.Comp {
		g.Go(func() (err error) {
			prev, err = s.st.GroupBy(gctx, store.ChooseLevel(q.Prev, d, false), q.Prev, d, store.GroupOpts{})
			return
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	rows := map[string]*row{}
	get := func(k string) *row {
		r, ok := rows[k]
		if !ok {
			r = &row{key: k}
			rows[k] = r
		}
		return r
	}
	for _, c := range cur {
		r := get(c.Key)
		r.sums = c.Sums
		if !c.FirstDate.IsZero() {
			r.first = c.FirstDate.Format("2006-01-02")
		}
		if !c.LastClick.IsZero() {
			r.last = c.LastClick.Format("2006-01-02")
		}
	}
	for k, v := range total {
		get(k).total = v
	}
	for k, v := range created {
		r := get(k)
		r.created = v
		if !lvl.HasNewLinks() {
			r.sums.NewLinks = v
		}
	}
	for k, v := range active {
		get(k).active = v
	}
	for _, p := range prev {
		get(p.Key).prevClick = p.Sums.Clicks
	}
	return rows, nil
}

func (r *row) metrics() gen.Metrics {
	t := r.total
	return toMetrics(r.sums, r.sums.NewLinks, r.active, &t)
}

func (r *row) changeClicks(comp bool) *float64 {
	if !comp {
		return nil
	}
	return change(float64(r.sums.Clicks), float64(r.prevClick))
}

func sumRows(rows []*row) gen.Metrics {
	var t row
	for _, r := range rows {
		t.sums.Add(r.sums)
		t.active += r.active
		t.total += r.total
	}
	return t.metrics()
}

// metricValue: giá trị để sắp xếp.
func metricValue(m gen.Metrics, field string, chg *float64) (float64, bool) {
	switch field {
	case "clicks":
		return float64(m.Clicks), true
	case "unique_clicks":
		return float64(m.UniqueClicks), true
	case "bot_clicks":
		return float64(m.BotClicks), true
	case "suspicious_clicks":
		return float64(m.SuspiciousClicks), true
	case "new_links":
		return float64(m.NewLinks), true
	case "active_links":
		return float64(m.ActiveLinks), true
	case "total_links":
		if m.TotalLinks != nil {
			return float64(*m.TotalLinks), true
		}
		return 0, true
	case "ctr_per_link":
		return m.CtrPerLink, true
	case "change_clicks":
		if chg == nil {
			return -1e18, true
		}
		return *chg, true
	}
	return 0, false
}

type Paging struct {
	Q        string
	Page     int
	PageSize int
	Sort     string
	Desc     bool
}

func (p Paging) bounds(n int) (int, int) {
	size := p.PageSize
	if size <= 0 {
		size = 50
	}
	page := max(p.Page, 1)
	start := min((page-1)*size, n)
	return start, min(start+size, n)
}

// Accounts: R2 danh sách.
func (s *Service) Accounts(ctx context.Context, q Query, pg Paging) (gen.AccountRowPage, error) {
	return cached(s, ctx, "accounts", q, pg, func(ctx context.Context) (gen.AccountRowPage, error) { return s.accounts(ctx, q, pg) })
}

func (s *Service) accounts(ctx context.Context, q Query, pg Paging) (gen.AccountRowPage, error) {
	rows, err := s.dimRows(ctx, q, store.DimOwner)
	if err != nil {
		return gen.AccountRowPage{}, err
	}
	list := filterRows(rows, pg.Q, func(r *row) string { return r.key }, func(r *row) bool { return r.key != "" })
	items := make([]gen.AccountRow, len(list))
	for i, r := range list {
		items[i] = gen.AccountRow{Username: r.key, Metrics: r.metrics(), ChangeClicks: r.changeClicks(q.Comp)}
	}
	sortBy := pg.Sort
	if sortBy == "" {
		sortBy = "clicks"
	}
	sort.SliceStable(items, func(i, j int) bool {
		if sortBy == "username" {
			return less(items[i].Username < items[j].Username, items[i].Username > items[j].Username, pg.Desc)
		}
		a, _ := metricValue(items[i].Metrics, sortBy, items[i].ChangeClicks)
		b, _ := metricValue(items[j].Metrics, sortBy, items[j].ChangeClicks)
		if a == b {
			return items[i].Username < items[j].Username
		}
		return less(a < b, a > b, pg.Desc)
	})
	start, end := pg.bounds(len(items))
	return gen.AccountRowPage{Items: items[start:end], Total: len(items), Page: max(pg.Page, 1), PageSize: end - start, Totals: sumRows(list)}, nil
}

func less(asc, desc, isDesc bool) bool {
	if isDesc {
		return desc
	}
	return asc
}

func filterRows(rows map[string]*row, q string, label func(*row) string, keep func(*row) bool) []*row {
	q = strings.ToLower(strings.TrimSpace(q))
	out := make([]*row, 0, len(rows))
	for _, r := range rows {
		if !keep(r) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(label(r)), q) {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// campaignRows: bảng chiến dịch (kèm số CTV, tên).
func (s *Service) campaignRows(ctx context.Context, q Query) ([]gen.CampaignRow, []*row, error) {
	var (
		rows     map[string]*row
		ctvCount map[string]int64
	)
	g, gctx := group(ctx)
	g.Go(func() (err error) { rows, err = s.dimRows(gctx, q, store.DimCampaign); return })
	g.Go(func() (err error) {
		ctvCount, err = s.st.DistinctCountBy(gctx, store.ChooseLevel(q.M, store.DimCampaign|store.DimCTV, false), q.M, "campaign_code", "ctv_id")
		return
	})
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}
	codes := make([]string, 0, len(rows))
	for k := range rows {
		codes = append(codes, k)
	}
	names, err := s.st.CampaignNames(ctx, codes)
	if err != nil {
		return nil, nil, err
	}
	list := make([]*row, 0, len(rows))
	out := make([]gen.CampaignRow, 0, len(rows))
	for _, r := range rows {
		// bỏ chiến dịch không có hoạt động / link trong kỳ
		if r.sums.Clicks == 0 && r.sums.Bot == 0 && r.sums.NewLinks == 0 && r.total == 0 {
			continue
		}
		list = append(list, r)
		name := names[r.key]
		if r.key == "" {
			name = "(không gắn chiến dịch)"
		}
		cr := gen.CampaignRow{Code: r.key, Name: name, Metrics: r.metrics(), CtvCount: ctvCount[r.key], ChangeClicks: r.changeClicks(q.Comp)}
		if r.first != "" {
			cr.FirstDate = parseDate(r.first)
		}
		if r.last != "" {
			cr.LastClickDate = parseDate(r.last)
		}
		out = append(out, cr)
	}
	return out, list, nil
}

func parseDate(s string) *openapi_types.Date {
	var d openapi_types.Date
	if err := d.UnmarshalText([]byte(s)); err != nil {
		return nil
	}
	return &d
}

func sortCampaigns(items []gen.CampaignRow, field string, desc bool) {
	if field == "" {
		field = "clicks"
	}
	sort.SliceStable(items, func(i, j int) bool {
		switch field {
		case "code", "name":
			a, b := items[i].Code, items[j].Code
			if field == "name" {
				a, b = items[i].Name, items[j].Name
			}
			return less(a < b, a > b, desc)
		case "ctv_count":
			a, b := items[i].CtvCount, items[j].CtvCount
			if a != b {
				return less(a < b, a > b, desc)
			}
		default:
			a, _ := metricValue(items[i].Metrics, field, items[i].ChangeClicks)
			b, _ := metricValue(items[j].Metrics, field, items[j].ChangeClicks)
			if a != b {
				return less(a < b, a > b, desc)
			}
		}
		return items[i].Code < items[j].Code
	})
}

// Campaigns: R3 danh sách.
func (s *Service) Campaigns(ctx context.Context, q Query, pg Paging) (gen.CampaignRowPage, error) {
	return cached(s, ctx, "campaigns", q, pg, func(ctx context.Context) (gen.CampaignRowPage, error) { return s.campaigns(ctx, q, pg) })
}

func (s *Service) campaigns(ctx context.Context, q Query, pg Paging) (gen.CampaignRowPage, error) {
	items, list, err := s.campaignRows(ctx, q)
	if err != nil {
		return gen.CampaignRowPage{}, err
	}
	if qq := strings.ToLower(strings.TrimSpace(pg.Q)); qq != "" {
		kept := items[:0]
		for _, it := range items {
			if strings.Contains(strings.ToLower(it.Code+" "+it.Name), qq) {
				kept = append(kept, it)
			}
		}
		items = kept
	}
	sortCampaigns(items, pg.Sort, pg.Desc)
	start, end := pg.bounds(len(items))
	return gen.CampaignRowPage{Items: items[start:end], Total: len(items), Page: max(pg.Page, 1), PageSize: end - start, Totals: sumRows(list)}, nil
}

// Account: R2 chi tiết (username phải nằm trong phạm vi).
func (s *Service) Account(ctx context.Context, q Query, username string) (gen.AccountReport, error) {
	if !scope.For(q.P).Allows(username) {
		return gen.AccountReport{}, domain.Forbidden("forbidden_scope", "tài khoản nằm ngoài phạm vi được xem")
	}
	return cached(s, ctx, "account", q, username, func(ctx context.Context) (gen.AccountReport, error) { return s.account(ctx, q, username) })
}

func (s *Service) account(ctx context.Context, q Query, username string) (gen.AccountReport, error) {
	q.M.Owners, q.M.AllOwners = []string{username}, false
	if q.Comp {
		q.Prev.Owners, q.Prev.AllOwners = []string{username}, false
	}
	out := gen.AccountReport{Username: username}
	g, gctx := group(ctx)
	g.Go(func() (err error) { out.Summary, err = s.Summary(gctx, q); return })
	g.Go(func() (err error) {
		out.Campaigns, _, err = s.campaignRows(gctx, q)
		sortCampaigns(out.Campaigns, "clicks", true)
		return
	})
	g.Go(func() (err error) { out.TopCtvs, err = s.TopCTVs(gctx, q, 10); return })
	g.Go(func() (err error) { out.TopLinks, err = s.TopLinks(gctx, q, 10); return })
	return out, g.Wait()
}

// Campaign: R3 chi tiết. code "-" = link không gắn chiến dịch.
func (s *Service) Campaign(ctx context.Context, q Query, code string) (gen.CampaignReport, error) {
	return cached(s, ctx, "campaign", q, code, func(ctx context.Context) (gen.CampaignReport, error) { return s.campaign(ctx, q, code) })
}

func (s *Service) campaign(ctx context.Context, q Query, code string) (gen.CampaignReport, error) {
	if code == "-" {
		code = ""
	}
	q.M.Campaigns = []string{code}
	if q.Comp {
		q.Prev.Campaigns = []string{code}
	}
	names, err := s.st.CampaignNames(ctx, []string{code})
	if err != nil {
		return gen.CampaignReport{}, err
	}
	name := names[code]
	if code == "" {
		name = "(không gắn chiến dịch)"
	}
	out := gen.CampaignReport{Code: code, Name: name}
	g, gctx := group(ctx)
	g.Go(func() (err error) { out.Summary, err = s.Summary(gctx, q); return })
	g.Go(func() (err error) { out.CtvRanking, err = s.CTVRanking(gctx, q, 100); return })
	g.Go(func() (err error) { out.TopLinks, err = s.TopLinks(gctx, q, 10); return })
	return out, g.Wait()
}

// CTVRanking: bảng xếp hạng CTV theo clicks (gồm dòng "chưa định danh").
func (s *Service) CTVRanking(ctx context.Context, q Query, limit int) ([]gen.CTVRankRow, error) {
	rows, err := s.dimRows(ctx, q, store.DimCTV)
	if err != nil {
		return nil, err
	}
	list := make([]*row, 0, len(rows))
	for _, r := range rows {
		if r.sums.Clicks > 0 || r.sums.NewLinks > 0 || r.sums.Bot > 0 {
			list = append(list, r)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].sums.Clicks != list[j].sums.Clicks {
			return list[i].sums.Clicks > list[j].sums.Clicks
		}
		return list[i].key < list[j].key
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	ids := make([]string, len(list))
	for i, r := range list {
		ids[i] = r.key
	}
	names, err := s.st.CTVNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]gen.CTVRankRow, len(list))
	for i, r := range list {
		out[i] = gen.CTVRankRow{CtvRef: s.ctvRef(r.key), CtvDisplay: q.CTVDisplay(r.key), Rank: i + 1, Metrics: r.metrics()}
		if nm := names[r.key]; nm != "" {
			out[i].Name = &nm
		}
	}
	return out, nil
}

// Compare: R3 so sánh 2–5 chiến dịch.
func (s *Service) Compare(ctx context.Context, q Query, codes []string) (gen.CampaignCompare, error) {
	return cached(s, ctx, "compare", q, codes, func(ctx context.Context) (gen.CampaignCompare, error) { return s.compare(ctx, q, codes) })
}

func (s *Service) compare(ctx context.Context, q Query, codes []string) (gen.CampaignCompare, error) {
	codes = normCampaigns(codes)
	names, err := s.st.CampaignNames(ctx, codes)
	if err != nil {
		return gen.CampaignCompare{}, err
	}
	items := make([]gen.CampaignCompareItem, len(codes))
	g, gctx := group(ctx)
	for i, c := range codes {
		m := q.M
		m.Campaigns = []string{c}
		g.Go(func() error {
			met, err := s.metrics(gctx, m)
			if err != nil {
				return err
			}
			ser, err := s.series(gctx, m, m, q.Gran)
			if err != nil {
				return err
			}
			items[i] = gen.CampaignCompareItem{Code: c, Name: names[c], Metrics: met, Series: ser}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return gen.CampaignCompare{}, err
	}
	return gen.CampaignCompare{Period: period(q.M), Granularity: gen.Granularity(q.Gran), Items: items}, nil
}
