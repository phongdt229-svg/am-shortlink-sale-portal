package report

import (
	"context"
	"sort"
	"strings"

	openapi_types "github.com/oapi-codegen/runtime/types"
	qrcode "github.com/skip2/go-qrcode"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/scope"
	"am-shortlink-portal/api/internal/store"
)

var errLinkNotFound = domain.NotFound("link_not_found", "không tìm thấy link trong phạm vi được xem")

// ShortURL: <base>/<prefix>/<code> — đúng prefix cấp cho link.
func (s *Service) ShortURL(l store.LinkInfo) string {
	base := strings.TrimRight(s.shortBase, "/")
	if l.Prefix == "" {
		return base + "/" + l.Code
	}
	return base + "/" + l.Prefix + "/" + l.Code
}

func (s *Service) linkInfo(q Query, l store.LinkInfo) gen.LinkInfo {
	isCustom := l.IsCustom
	out := gen.LinkInfo{
		Code: l.Code, ShortUrl: s.ShortURL(l), LongUrl: maskURL(l.LongURL, s.piiSnapshot(), q.admin), Owner: l.Owner,
		CampaignCode: l.Campaign, Prefix: l.Prefix, Status: gen.LinkInfoStatus(l.Status),
		CreatedAt: l.CreatedAt, CtvDisplay: q.CTVDisplay(l.CTV), IsCustom: &isCustom,
	}
	if l.CTV != "" {
		out.CtvRef = s.ctvRef(l.CTV)
	}
	if l.DestHost != "" {
		out.DestHost = &l.DestHost
	}
	if l.APIVersion != "" {
		out.ApiVersion = &l.APIVersion
	}
	if out.Status == "" {
		out.Status = gen.LinkInfoStatus("active")
	}
	return out
}

// scopedLink: link theo mã, chỉ khi owner thuộc phạm vi (ngoài phạm vi → 404, không lộ sự tồn tại).
func (s *Service) scopedLink(ctx context.Context, p domain.Principal, code string) (*store.LinkInfo, error) {
	l, err := s.st.LinkByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if l == nil || !scope.For(p).Allows(l.Owner) {
		return nil, errLinkNotFound
	}
	return l, nil
}

// Link: R5 chi tiết — KPI, timeline, phân rã, 100 click gần nhất.
func (s *Service) Link(ctx context.Context, q Query, code string) (gen.LinkReport, error) {
	l, err := s.scopedLink(ctx, q.P, code)
	if err != nil {
		return gen.LinkReport{}, err
	}
	only := func(m store.Match) store.Match {
		return store.Match{From: m.From, To: m.To, Owners: []string{l.Owner}, LinkIDs: []int64{l.ID}}
	}
	q.M = only(q.M)
	if q.Comp {
		q.Prev = only(q.Prev)
	}
	return cached(s, ctx, "link", q, code, func(ctx context.Context) (gen.LinkReport, error) {
		out := gen.LinkReport{Link: s.linkInfo(q, *l)}
		g, gctx := group(ctx)
		g.Go(func() (err error) { out.Summary, err = s.Summary(gctx, q); return })
		g.Go(func() error {
			cq := store.ClickQuery{M: clampRaw(q.M)}
			docs, err := s.st.ListClicks(gctx, cq, nil, 100)
			if err != nil {
				return err
			}
			out.RecentClicks = s.clickRows(q, docs, map[int64]store.LinkInfo{l.ID: *l})
			return nil
		})
		return out, g.Wait()
	})
}

// QRCode: PNG của short URL.
func (s *Service) QRCode(ctx context.Context, p domain.Principal, code string, size int) ([]byte, error) {
	l, err := s.scopedLink(ctx, p, code)
	if err != nil {
		return nil, err
	}
	return qrcode.Encode(s.ShortURL(*l), qrcode.Medium, size)
}

// TopLinks: R5 — mode clicks | growth | dead.
func (s *Service) TopLinksPage(ctx context.Context, q Query, mode string, deadDays int, pg Paging) (gen.LinkRowPage, error) {
	return cached(s, ctx, "links_top", q, []any{mode, deadDays, pg}, func(ctx context.Context) (gen.LinkRowPage, error) {
		switch mode {
		case "dead":
			return s.deadLinks(ctx, q, deadDays, pg)
		case "growth":
			return s.growthLinks(ctx, q, pg)
		default:
			return s.topLinks(ctx, q, pg)
		}
	})
}

func (s *Service) pageLinks(ctx context.Context, q Query, rows []store.GroupRow, prev map[int64]int64, total int, pg Paging) (gen.LinkRowPage, error) {
	start, end := pg.bounds(len(rows))
	page := rows[start:end]
	ids := make([]int64, len(page))
	for i, r := range page {
		ids[i] = r.LinkID
	}
	links, err := s.st.LinksByIDs(ctx, q.M, ids)
	if err != nil {
		return gen.LinkRowPage{}, err
	}
	items := make([]gen.LinkRow, 0, len(page))
	for _, r := range page {
		l, ok := links[r.LinkID]
		if !ok {
			continue
		}
		row := gen.LinkRow{Link: s.linkInfo(q, l), Clicks: r.Sums.Clicks, UniqueClicks: r.Sums.Unique, SuspiciousClicks: ptr(r.Sums.Susp)}
		if !r.LastClick.IsZero() {
			row.LastClickDate = &openapi_types.Date{Time: r.LastClick}
		}
		if prev != nil {
			p := prev[r.LinkID]
			row.PreviousClicks = &p
			row.Growth = change(float64(r.Sums.Clicks), float64(p))
		}
		items = append(items, row)
	}
	if total < 0 {
		total = len(rows)
	}
	return gen.LinkRowPage{Items: items, Total: total, Page: max(pg.Page, 1), PageSize: len(items)}, nil
}

func ptr[T any](v T) *T { return &v }

func (s *Service) topLinks(ctx context.Context, q Query, pg Paging) (gen.LinkRowPage, error) {
	rows, err := s.st.GroupBy(ctx, store.LevelLink, q.M, store.DimLink, store.GroupOpts{SortBy: "clicks"})
	if err != nil {
		return gen.LinkRowPage{}, err
	}
	kept := rows[:0]
	for _, r := range rows {
		if r.Sums.Clicks > 0 {
			kept = append(kept, r)
		}
	}
	return s.pageLinks(ctx, q, kept, nil, -1, pg)
}

// growthLinks: xếp theo số click tăng thêm so kỳ trước liền kề.
func (s *Service) growthLinks(ctx context.Context, q Query, pg Paging) (gen.LinkRowPage, error) {
	prevM := previous(q.M)
	var cur, prev []store.GroupRow
	g, gctx := group(ctx)
	g.Go(func() (err error) {
		cur, err = s.st.GroupBy(gctx, store.LevelLink, q.M, store.DimLink, store.GroupOpts{})
		return
	})
	g.Go(func() (err error) {
		prev, err = s.st.GroupBy(gctx, store.LevelLink, prevM, store.DimLink, store.GroupOpts{})
		return
	})
	if err := g.Wait(); err != nil {
		return gen.LinkRowPage{}, err
	}
	pm := map[int64]int64{}
	for _, p := range prev {
		pm[p.LinkID] = p.Sums.Clicks
	}
	rows := cur[:0]
	for _, r := range cur {
		if r.Sums.Clicks > pm[r.LinkID] {
			rows = append(rows, r)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		di, dj := rows[i].Sums.Clicks-pm[rows[i].LinkID], rows[j].Sums.Clicks-pm[rows[j].LinkID]
		if di != dj {
			return di > dj
		}
		return rows[i].LinkID < rows[j].LinkID
	})
	return s.pageLinks(ctx, q, rows, pm, -1, pg)
}

// deadLinks: link đang hoạt động, không có click trong deadDays ngày gần nhất (tính đến `to`).
func (s *Service) deadLinks(ctx context.Context, q Query, deadDays int, pg Paging) (gen.LinkRowPage, error) {
	if deadDays <= 0 {
		deadDays = 14
	}
	windowFrom := q.M.To.AddDate(0, 0, -(deadDays - 1))
	size := pg.PageSize
	if size <= 0 {
		size = 50
	}
	skip := int64((max(pg.Page, 1) - 1) * size)
	links, total, err := s.st.DeadLinks(ctx, q.M, windowFrom, skip, int64(size))
	if err != nil {
		return gen.LinkRowPage{}, err
	}
	ids := make([]int64, len(links))
	for i, l := range links {
		ids[i] = l.ID
	}
	last, err := s.st.LastClickDates(ctx, ids, q.M.To)
	if err != nil {
		return gen.LinkRowPage{}, err
	}
	items := make([]gen.LinkRow, len(links))
	for i, l := range links {
		items[i] = gen.LinkRow{Link: s.linkInfo(q, l)}
		if d, ok := last[l.ID]; ok {
			items[i].LastClickDate = &openapi_types.Date{Time: d}
		}
	}
	return gen.LinkRowPage{Items: items, Total: int(total), Page: max(pg.Page, 1), PageSize: len(items)}, nil
}

// clampRaw: click thô tối đa 3 tháng — lấy MaxRawDays ngày cuối của kỳ.
func clampRaw(m store.Match) store.Match {
	if days(m.From, m.To) > MaxRawDays {
		m.From = m.To.AddDate(0, 0, -(MaxRawDays - 1))
	}
	return m
}
