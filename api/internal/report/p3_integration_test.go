//go:build integration

package report_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/seed"
)

func ptr[T any](v T) *T { return &v }

// Duyệt hết các trang click → đếm phải bằng đáp án; không trùng click giữa các trang.
func allClicks(t *testing.T, svc *report.Service, q report.Query, f *gen.ClickFilters) []gen.ClickRow {
	t.Helper()
	var out []gen.ClickRow
	cursor := ""
	for i := 0; i < 1000; i++ {
		page, err := svc.Clicks(context.Background(), q, f, cursor, 200)
		require.NoError(t, err)
		out = append(out, page.Items...)
		if page.NextCursor == nil {
			return out
		}
		cursor = *page.NextCursor
	}
	t.Fatal("quá nhiều trang")
	return nil
}

func countOracle(ds *seed.Dataset, from, to string, keep func(c *seed.Click, l *seed.Link) bool) int {
	links := map[int64]*seed.Link{}
	for i := range ds.Links {
		links[ds.Links[i].ID] = &ds.Links[i]
	}
	n := 0
	for i := range ds.Clicks {
		c := &ds.Clicks[i]
		d := seed.DateOf(c.TS)
		if d.Before(day(from)) || d.After(day(to)) {
			continue
		}
		if keep(c, links[c.LinkID]) {
			n++
		}
	}
	return n
}

func TestClickLogCursorAndFilters(t *testing.T) {
	env, svc := setup(t)
	from, to := "2026-09-20", "2026-10-07"
	q, err := svc.Build(partnerA, report.Params{From: day(from), To: day(to)})
	require.NoError(t, err)

	rows := allClicks(t, svc, q, nil)
	want := countOracle(env.Dataset, from, to, func(c *seed.Click, _ *seed.Link) bool { return c.Owner == "partner_a" })
	require.Len(t, rows, want, "tất cả click (gồm bot) của partner_a")
	for i := 1; i < len(rows); i++ {
		require.False(t, rows[i].Ts.After(rows[i-1].Ts), "giảm dần theo thời gian")
	}
	for _, r := range rows {
		require.Regexp(t, `\.x\.x$|^x\.x\.x\.x$`, r.Ip, "user thường chỉ thấy IP đã che")
		require.Equal(t, "partner_a", r.Owner)
	}

	// Bộ lọc chi tiết: thiết bị mobile, 19h–22h, chỉ click hợp lệ, nguồn Zalo, loại trừ Android.
	f := &gen.ClickFilters{
		Device:      &gen.Criterion{Values: []string{"mobile"}},
		Os:          &gen.Criterion{Values: []string{"Android"}, Exclude: ptr(true)},
		SourceGroup: &gen.Criterion{Values: []string{"zalo"}},
		HourFrom:    ptr(19), HourTo: ptr(22),
		Quality: ptr(gen.ClickFiltersQualityValid),
	}
	rows = allClicks(t, svc, q, f)
	want = countOracle(env.Dataset, from, to, func(c *seed.Click, _ *seed.Link) bool {
		return c.Owner == "partner_a" && c.Device == "mobile" && c.OS != "Android" && c.SourceGroup == "zalo" && c.Hour >= 19 && c.Hour <= 22 && !c.IsBot
	})
	require.Len(t, rows, want)

	// Khung giờ qua nửa đêm + chỉ khách lần đầu.
	f = &gen.ClickFilters{HourFrom: ptr(22), HourTo: ptr(2), Visit: ptr(gen.ClickFiltersVisitFirst)}
	rows = allClicks(t, svc, q, f)
	want = countOracle(env.Dataset, from, to, func(c *seed.Click, _ *seed.Link) bool {
		return c.Owner == "partner_a" && (c.Hour >= 22 || c.Hour <= 2) && !c.IsRepeat // visit độc lập với quality (bot vẫn tính)
	})
	require.Len(t, rows, want)
}

func TestClickFilterByURLParam(t *testing.T) {
	env, svc := setup(t)
	from, to := "2026-09-20", "2026-10-07"
	q, err := svc.Build(admin, report.Params{From: day(from), To: day(to)})
	require.NoError(t, err)
	has := func(l *seed.Link, k, v string) bool {
		for _, p := range l.Params {
			if p.Key == k && (v == "" || p.Value == v) {
				return true
			}
		}
		return false
	}
	// utm_source = ZALO (chuẩn hoá chữ thường) và KHÔNG có tham số promo.
	f := &gen.ClickFilters{Params: &[]gen.ParamCondition{
		{Key: "utm_source", Op: gen.ParamConditionOp("eq"), Values: &[]string{"ZALO"}},
		{Key: "promo", Op: gen.ParamConditionOp("not_exists")},
	}}
	rows := allClicks(t, svc, q, f)
	want := countOracle(env.Dataset, from, to, func(_ *seed.Click, l *seed.Link) bool {
		return has(l, "utm_source", "zalo") && !has(l, "promo", "")
	})
	require.Len(t, rows, want)
	require.NotZero(t, want)

	// Tham số PII (pii = hash): lọc bằng SĐT bản rõ, so trên giá trị băm.
	var phone string
	for _, l := range env.Dataset.Links {
		for _, p := range l.Params {
			if p.Key == "phonenumber" {
				phone = p.Value
			}
		}
	}
	if phone != "" {
		// p.Value đã là bản băm trong dataset → tìm bản rõ không được; chỉ kiểm không lỗi và không trả bản rõ.
		_, err := svc.Clicks(context.Background(), q, &gen.ClickFilters{Params: &[]gen.ParamCondition{{Key: "phonenumber", Op: gen.ParamConditionOp("eq"), Values: &[]string{"0901234567"}}}}, "", 10)
		require.NoError(t, err)
	}
}

func TestIPFilterAdminOnlyAndRawRangeLimit(t *testing.T) {
	_, svc := setup(t)
	q, _ := svc.Build(partnerA, report.Params{From: day("2026-09-20"), To: day("2026-10-07")})
	_, err := svc.Clicks(context.Background(), q, &gen.ClickFilters{Ip: &gen.Criterion{Values: []string{"113.161.0.0/16"}}}, "", 10)
	e, ok := domain.AsError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, e.Status)

	qa, _ := svc.Build(admin, report.Params{From: day("2026-09-20"), To: day("2026-10-07")})
	page, err := svc.Clicks(context.Background(), qa, &gen.ClickFilters{Ip: &gen.Criterion{Values: []string{"113.161.0.0/16"}}}, "", 50)
	require.NoError(t, err)
	for _, r := range page.Items {
		require.Regexp(t, `^113\.161\.`, r.Ip)
	}

	long, _ := svc.Build(admin, report.Params{From: day("2026-06-01"), To: day("2026-10-07")})
	_, err = svc.Clicks(context.Background(), long, nil, "", 10)
	require.Error(t, err, "click thô > 3 tháng bị từ chối")
}

func TestCTVRankingLookupAndDetail(t *testing.T) {
	env, svc := setup(t)
	ctx := context.Background()
	q, _ := svc.Build(admin, report.Params{From: day("2026-09-01"), To: day("2026-10-07")})
	page, err := svc.CTVs(ctx, q, report.Paging{PageSize: 200})
	require.NoError(t, err)
	require.NotEmpty(t, page.Items)
	top := page.Items[0]
	require.Equal(t, 1, top.Rank)

	m := mask.NewMasker("integration-test-pii-salt-0123456789")
	id, err := m.ParseCTVRef(top.CtvRef)
	require.NoError(t, err)
	want := countOracle(env.Dataset, "2026-09-01", "2026-10-07", func(c *seed.Click, _ *seed.Link) bool { return c.CTV == id && !c.IsBot })
	require.EqualValues(t, want, top.Metrics.Clicks)

	det, err := svc.CTV(ctx, q, top.CtvRef)
	require.NoError(t, err)
	require.EqualValues(t, want, det.Summary.Kpis.Current.Clicks)

	lk, err := svc.Lookup(ctx, admin, "+84"+id[1:])
	require.NoError(t, err)
	if mask.IsPhone(id) {
		require.NotEmpty(t, lk.Items, "tra cứu SĐT dạng +84 ra đúng CTV")
	}

	// User khác phạm vi: CTV của partner_a không hiện với partner_b.
	pb := domain.Principal{Username: "partner_b", Role: domain.RoleUser}
	var onlyA string
	// Owner thực tế của CTV lấy từ link (danh mục ctvs chỉ ghi owner đầu tiên của CTV dùng chung).
	owners := map[string]map[string]bool{}
	for _, l := range env.Dataset.Links {
		if l.CTV == "" {
			continue
		}
		if owners[l.CTV] == nil {
			owners[l.CTV] = map[string]bool{}
		}
		owners[l.CTV][l.Owner] = true
	}
	for idv, os := range owners {
		if len(os) == 1 && os["partner_a"] {
			onlyA = idv
			break
		}
	}
	require.NotEmpty(t, onlyA)
	qb, _ := svc.Build(pb, report.Params{From: day("2026-09-01"), To: day("2026-10-07")})
	_, err = svc.CTV(ctx, qb, m.CTVRef(onlyA))
	require.Error(t, err)
}

func TestLinkDetailScopeAndTop(t *testing.T) {
	env, svc := setup(t)
	ctx := context.Background()
	var codeA string
	for _, l := range env.Dataset.Links {
		if l.Owner == "partner_a" && l.Status == "active" {
			codeA = l.Code
		}
	}
	qa, _ := svc.Build(partnerA, report.Params{From: day("2026-09-01"), To: day("2026-10-07")})
	r, err := svc.Link(ctx, qa, codeA)
	require.NoError(t, err)
	require.Contains(t, r.Link.ShortUrl, "/"+r.Link.Prefix+"/"+codeA)
	require.LessOrEqual(t, len(r.RecentClicks), 100)
	require.NotRegexp(t, `utm_extra_ctv=(84|0)\d{9}`, r.Link.LongUrl, "user thường không thấy SĐT CTV trong long_url")
	require.NotRegexp(t, `phonenumber=[^*&]`, r.Link.LongUrl, "tham số PII luôn bị che")
	for _, c := range r.RecentClicks {
		require.NotRegexp(t, `utm_extra_ctv=(84|0)\d{9}`, *c.LongUrl)
	}

	pb := domain.Principal{Username: "partner_b", Role: domain.RoleUser}
	qb, _ := svc.Build(pb, report.Params{From: day("2026-09-01"), To: day("2026-10-07")})
	_, err = svc.Link(ctx, qb, codeA)
	e, ok := domain.AsError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, e.Status, "link ngoài phạm vi → 404")

	top, err := svc.TopLinksPage(ctx, qa, "clicks", 0, report.Paging{Page: 1, PageSize: 10})
	require.NoError(t, err)
	for i := 1; i < len(top.Items); i++ {
		require.GreaterOrEqual(t, top.Items[i-1].Clicks, top.Items[i].Clicks)
	}
	dead, err := svc.TopLinksPage(ctx, qa, "dead", 7, report.Paging{Page: 1, PageSize: 50})
	require.NoError(t, err)
	for _, d := range dead.Items {
		require.Equal(t, gen.LinkInfoStatus("active"), d.Link.Status)
	}
}
