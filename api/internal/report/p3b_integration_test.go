//go:build integration

package report_test

import (
	"context"
	"net/http"
	"testing"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/require"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/savedreport"
	"am-shortlink-portal/api/internal/seed"
	"am-shortlink-portal/api/internal/testutil"
)

func explorer(t *testing.T, svc *report.Service, p domain.Principal, from, to string, body gen.ExplorerQuery) gen.ExplorerResult {
	t.Helper()
	body.From, body.To = openapi_types.Date{Time: day(from)}, openapi_types.Date{Time: day(to)}
	q, err := svc.Build(p, report.Params{From: day(from), To: day(to), Accounts: deref(body.Account), Campaigns: deref(body.Campaign), Compare: body.Compare != nil && *body.Compare})
	require.NoError(t, err)
	out, err := svc.Explorer(context.Background(), q, body)
	require.NoError(t, err)
	return out
}

func deref(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

func sumRows(r gen.ExplorerResult) int64 {
	var n int64
	for _, row := range r.Rows {
		n += row.Metrics.Clicks
	}
	if r.Other != nil {
		n += r.Other.Metrics.Clicks
	}
	return n
}

// Tổng nhóm Explorer = thẻ tổng = R2 (PG5 / §10).
func TestExplorerGroupsEqualKpisAndAccounts(t *testing.T) {
	env, svc := setup(t)
	ctx := context.Background()
	from, to := "2026-09-10", "2026-10-07"

	ex := explorer(t, svc, admin, from, to, gen.ExplorerQuery{GroupBy: []string{"account"}})
	require.Equal(t, "stats", string(ex.Source))
	q, _ := svc.Build(admin, report.Params{From: day(from), To: day(to)})
	k, err := svc.Kpis(ctx, q)
	require.NoError(t, err)
	acc, err := svc.Accounts(ctx, q, report.Paging{PageSize: 200})
	require.NoError(t, err)

	require.Equal(t, k.Current.Clicks, ex.Totals.Metrics.Clicks)
	require.Equal(t, k.Current.ActiveLinks, ex.Totals.Metrics.ActiveLinks)
	require.Equal(t, k.Current.Clicks, sumRows(ex))
	byAcc := map[string]int64{}
	for _, r := range acc.Items {
		byAcc[r.Username] = r.Metrics.Clicks
	}
	for _, r := range ex.Rows {
		require.Equal(t, byAcc[r.Keys[0].Value], r.Metrics.Clicks, "account "+r.Keys[0].Value)
	}
	_ = env
}

// Cùng câu hỏi, nguồn stats và nguồn clicks phải ra cùng số.
func TestExplorerStatsMatchesClicks(t *testing.T) {
	_, svc := setup(t)
	from, to := "2026-09-10", "2026-10-07"
	stats := explorer(t, svc, viewer, from, to, gen.ExplorerQuery{GroupBy: []string{"campaign", "week"}})
	all := gen.ClickFiltersQuality("valid")
	raw := explorer(t, svc, viewer, from, to, gen.ExplorerQuery{GroupBy: []string{"campaign", "week"}, Filters: &gen.ClickFilters{Quality: &all}})
	require.Equal(t, "stats", string(stats.Source))
	require.Equal(t, "clicks", string(raw.Source))
	require.Equal(t, stats.Totals.Metrics.Clicks, raw.Totals.Metrics.Clicks)
	require.Equal(t, stats.Totals.Metrics.UniqueClicks, raw.Totals.Metrics.UniqueClicks)
	require.Equal(t, stats.Totals.Metrics.SuspiciousClicks, raw.Totals.Metrics.SuspiciousClicks)
	require.Equal(t, stats.Totals.Metrics.ActiveLinks, raw.Totals.Metrics.ActiveLinks)
	m := map[string]int64{}
	for _, r := range stats.Rows {
		m[r.Keys[0].Value+"|"+r.Keys[1].Value] = r.Metrics.Clicks
	}
	for _, r := range raw.Rows {
		require.Equal(t, m[r.Keys[0].Value+"|"+r.Keys[1].Value], r.Metrics.Clicks)
	}
}

func TestExplorerDetailFiltersAndParamDimension(t *testing.T) {
	env, svc := setup(t)
	from, to := "2026-09-20", "2026-10-07"
	valid := gen.ClickFiltersQuality("valid")
	ex := explorer(t, svc, partnerA, from, to, gen.ExplorerQuery{
		GroupBy: []string{"device", "source_group"},
		Filters: &gen.ClickFilters{Quality: &valid, HourFrom: ptr(19), HourTo: ptr(22)},
		Limit:   ptr(3),
	})
	want := countOracle(env.Dataset, from, to, func(c *seed.Click, _ *seed.Link) bool {
		return c.Owner == "partner_a" && !c.IsBot && c.Hour >= 19 && c.Hour <= 22
	})
	require.EqualValues(t, want, ex.Totals.Metrics.Clicks)
	require.EqualValues(t, want, sumRows(ex), "top 3 + khác = tổng")
	require.True(t, ex.Truncated)
	require.Len(t, ex.Rows, 3)

	// Nhóm theo tham số URL.
	pe := explorer(t, svc, admin, from, to, gen.ExplorerQuery{GroupBy: []string{"param.utm_source"}})
	got := map[string]int64{}
	for _, r := range pe.Rows {
		got[r.Keys[0].Value] = r.Metrics.Clicks
	}
	wantBy := testutil.ClicksBy(env.Dataset, testutil.Filter{From: day(from), To: day(to)}, func(_ *seed.Click, l *seed.Link) string {
		for _, p := range l.Params {
			if p.Key == "utm_source" {
				return p.Value
			}
		}
		return ""
	})
	require.Equal(t, wantBy, got)

	// Không nhóm theo tham số PII; > 3 tháng chi tiết bị từ chối.
	q, _ := svc.Build(admin, report.Params{From: day(from), To: day(to)})
	_, err := svc.Explorer(context.Background(), q, gen.ExplorerQuery{GroupBy: []string{"param.phonenumber"}, From: openapi_types.Date{Time: day(from)}, To: openapi_types.Date{Time: day(to)}})
	require.Error(t, err)
	ql, _ := svc.Build(admin, report.Params{From: day("2026-06-01"), To: day(to)})
	_, err = svc.Explorer(context.Background(), ql, gen.ExplorerQuery{GroupBy: []string{"device"}, From: openapi_types.Date{Time: day("2026-06-01")}, To: openapi_types.Date{Time: day(to)}})
	require.Error(t, err)
}

func TestFacetsAndParamsScoped(t *testing.T) {
	env, svc := setup(t)
	ctx := context.Background()
	qa, _ := svc.ScopeOnly(partnerA, nil)
	f, err := svc.Facets(ctx, qa, "device", "", 50)
	require.NoError(t, err)
	require.NotEmpty(t, f.Items)
	_, err = svc.Facets(ctx, qa, "param.phonenumber", "", 50)
	require.Error(t, err)

	from, to := "2026-09-10", "2026-10-07"
	q, _ := svc.Build(admin, report.Params{From: day(from), To: day(to)})
	list, err := svc.Params(ctx, q)
	require.NoError(t, err)
	for _, it := range list.Items {
		require.NotEqual(t, "phonenumber", it.Key, "không liệt kê tham số PII")
	}
	pr, err := svc.Param(ctx, q, "utm_source", report.Paging{PageSize: 100})
	require.NoError(t, err)
	wantBy := testutil.ClicksBy(env.Dataset, testutil.Filter{From: day(from), To: day(to)}, func(_ *seed.Click, l *seed.Link) string {
		for _, p := range l.Params {
			if p.Key == "utm_source" {
				return p.Value
			}
		}
		return ""
	})
	for _, r := range pr.Rows {
		require.Equal(t, wantBy[r.Value], r.Metrics.Clicks, "utm_source="+r.Value)
	}
	require.LessOrEqual(t, len(pr.TopSeries), 5)
}

func TestSavedReportsOwnershipAndSharing(t *testing.T) {
	env, _ := setup(t)
	ctx := context.Background()
	s := savedreport.New(env.Store, nil)
	r, err := s.Create(ctx, partnerA, gen.SavedReportInput{Name: "Zalo theo tuần", Kind: gen.SavedReportInputKind("explorer"), Query: map[string]any{"group_by": []any{"week"}}})
	require.NoError(t, err)

	pb := domain.Principal{Username: "partner_b", Role: domain.RoleUser}
	_, err = s.Get(ctx, pb, r.Id)
	e, ok := domain.AsError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, e.Status, "người khác không đọc được theo id")

	sh, err := s.Share(ctx, partnerA, r.Id, true)
	require.NoError(t, err)
	require.NotNil(t, sh.ShareToken)
	got, err := s.GetShared(ctx, pb, *sh.ShareToken)
	require.NoError(t, err)
	require.False(t, got.IsMine)
	require.Nil(t, got.ShareToken, "người nhận không thấy token để chia sẻ tiếp")

	_, err = s.Share(ctx, partnerA, r.Id, false)
	require.NoError(t, err)
	_, err = s.GetShared(ctx, pb, *sh.ShareToken)
	require.Error(t, err, "tắt chia sẻ → link cũ hết hiệu lực")
	require.NoError(t, s.Delete(ctx, partnerA, r.Id))
}
