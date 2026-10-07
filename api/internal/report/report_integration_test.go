//go:build integration

package report_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/seed"
	"am-shortlink-portal/api/internal/store"
	"am-shortlink-portal/api/internal/testutil"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testutil.Teardown()
	os.Exit(code)
}

var (
	admin    = domain.Principal{Username: "admin", Role: domain.RoleAdmin}
	viewer   = domain.Principal{Username: "viewer", Role: domain.RoleViewer, ViewerAccounts: []string{"partner_a", "partner_b"}}
	partnerA = domain.Principal{Username: "partner_a", Role: domain.RoleUser}
)

func day(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func setup(t *testing.T) (*testutil.Env, *report.Service) {
	env := testutil.Seeded(t)
	return env, report.NewService(env.Store, mask.NewMasker(testutil.Salt)).WithShortURLBase("https://s.test")
}

func requireMetrics(t *testing.T, want testutil.Metrics, got gen.Metrics, msg string) {
	t.Helper()
	require.Equal(t, want.Clicks, got.Clicks, msg+": clicks")
	require.Equal(t, want.Unique, got.UniqueClicks, msg+": unique_clicks")
	require.Equal(t, want.Bot, got.BotClicks, msg+": bot_clicks")
	require.Equal(t, want.Susp, got.SuspiciousClicks, msg+": suspicious_clicks")
	require.Equal(t, want.Active, got.ActiveLinks, msg+": active_links")
	require.Equal(t, want.New, got.NewLinks, msg+": new_links")
	if got.TotalLinks != nil {
		require.Equal(t, want.Total, *got.TotalLinks, msg+": total_links")
	}
}

// KPI khớp định nghĩa §5.2 ở MỌI mức tổng hợp (system / owner / campaign / ctv / link).
func TestKpisMatchOracleAcrossLevels(t *testing.T) {
	env, svc := setup(t)
	ctx := context.Background()
	from, to := day("2026-09-10"), day("2026-10-07")
	cases := []struct {
		name string
		p    domain.Principal
		prm  report.Params
		f    testutil.Filter
	}{
		{"system", admin, report.Params{}, testutil.Filter{}},
		{"owner (user)", partnerA, report.Params{}, testutil.Filter{Owners: []string{"partner_a"}}},
		{"viewer 2 accounts", viewer, report.Params{}, testutil.Filter{Owners: []string{"partner_a", "partner_b"}}},
		{"campaign", admin, report.Params{Campaigns: []string{"FPT_PLAY_T9"}}, testutil.Filter{Campaigns: []string{"FPT_PLAY_T9"}}},
		{"no campaign", partnerA, report.Params{Campaigns: []string{"-"}}, testutil.Filter{Owners: []string{"partner_a"}, Campaigns: []string{""}}},
		{"unidentified ctv", admin, report.Params{CTVs: []string{"-"}}, testutil.Filter{CTVs: []string{""}}},
		{"prefix lm", admin, report.Params{Prefixes: []string{"lm"}}, testutil.Filter{Prefixes: []string{"lm"}}},
		{"prefix + account", viewer, report.Params{Accounts: []string{"partner_b"}, Prefixes: []string{"sale"}}, testutil.Filter{Owners: []string{"partner_b"}, Prefixes: []string{"sale"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.prm.From, c.prm.To = from, to
			c.f.From, c.f.To = from, to
			q, err := svc.Build(c.p, c.prm)
			require.NoError(t, err)
			k, err := svc.Kpis(ctx, q)
			require.NoError(t, err)
			requireMetrics(t, testutil.Compute(env.Dataset, c.f), k.Current, c.name)
		})
	}
}

func TestCTVFilterAcceptsPhoneFormatsAndRef(t *testing.T) {
	env, svc := setup(t)
	ctx := context.Background()
	var phone string
	for _, c := range env.Dataset.CTVs {
		if mask.IsPhone(c.ID) && c.Owner == "partner_a" {
			phone = c.ID
			break
		}
	}
	require.NotEmpty(t, phone)
	want := testutil.Compute(env.Dataset, testutil.Filter{From: day("2026-09-01"), To: day("2026-10-07"), CTVs: []string{phone}})
	m := mask.NewMasker(testutil.Salt)
	for _, in := range []string{phone, "+84" + phone[1:], phone[:4] + " " + phone[4:], m.CTVRef(phone)} {
		q, err := svc.Build(admin, report.Params{From: day("2026-09-01"), To: day("2026-10-07"), CTVs: []string{in}})
		require.NoError(t, err)
		k, err := svc.Kpis(ctx, q)
		require.NoError(t, err)
		require.Equal(t, want.Clicks, k.Current.Clicks, in)
	}
}

// Tổng theo nhóm (series, phân rã, heatmap, bảng tài khoản / chiến dịch / CTV) = thẻ tổng.
func TestGroupTotalsEqualKpis(t *testing.T) {
	env, svc := setup(t)
	ctx := context.Background()
	q, err := svc.Build(admin, report.Params{From: day("2026-09-01"), To: day("2026-10-07"), Compare: true, Granularity: store.Week})
	require.NoError(t, err)
	sum, err := svc.Summary(ctx, q)
	require.NoError(t, err)
	k := sum.Kpis.Current

	var sc, sn int64
	for _, p := range sum.Series.Points {
		sc += p.Clicks
		sn += p.NewLinks
	}
	require.Equal(t, k.Clicks, sc, "series clicks")
	require.Equal(t, k.NewLinks, sn, "series new_links")
	require.Len(t, *sum.Series.Previous, len(sum.Series.Points))
	var pc int64
	for _, p := range *sum.Series.Previous {
		pc += p.Clicks
	}
	require.Equal(t, sum.Kpis.Previous.Clicks, pc, "chuỗi kỳ trước (đã dịch vào bucket hiện tại) = KPI kỳ trước")

	var dev, hm int64
	for _, b := range sum.Breakdowns.Device {
		dev += b.Clicks
	}
	for _, c := range sum.Heatmap {
		hm += c.Clicks
	}
	require.Equal(t, k.Clicks, dev, "device breakdown")
	require.Equal(t, k.Clicks, hm, "heatmap")

	wantDev := testutil.ClicksBy(env.Dataset, testutil.Filter{From: day("2026-09-01"), To: day("2026-10-07")},
		func(c *seed.Click, _ *seed.Link) string { return c.Device })
	for _, b := range sum.Breakdowns.Device {
		require.Equal(t, wantDev[b.Key], b.Clicks, "device "+b.Key)
	}

	acc, err := svc.Accounts(ctx, q, report.Paging{PageSize: 200})
	require.NoError(t, err)
	require.Equal(t, k.Clicks, acc.Totals.Clicks)
	require.Equal(t, k.ActiveLinks, acc.Totals.ActiveLinks)
	require.Equal(t, *k.TotalLinks, *acc.Totals.TotalLinks)
	for _, r := range acc.Items {
		want := testutil.Compute(env.Dataset, testutil.Filter{From: day("2026-09-01"), To: day("2026-10-07"), Owners: []string{r.Username}})
		requireMetrics(t, want, r.Metrics, "account "+r.Username)
	}

	camps, err := svc.Campaigns(ctx, q, report.Paging{PageSize: 200})
	require.NoError(t, err)
	require.Equal(t, k.Clicks, camps.Totals.Clicks, "campaign rows (gồm dòng không gắn chiến dịch) = tổng")
	for _, r := range camps.Items {
		want := testutil.Compute(env.Dataset, testutil.Filter{From: day("2026-09-01"), To: day("2026-10-07"), Campaigns: []string{r.Code}})
		requireMetrics(t, want, r.Metrics, "campaign "+r.Code)
	}

	rank, err := svc.CTVRanking(ctx, q, 0)
	require.NoError(t, err)
	var rc int64
	for _, r := range rank {
		rc += r.Metrics.Clicks
	}
	require.Equal(t, k.Clicks, rc, "ctv ranking (gồm chưa định danh) = tổng")
}

func TestScopeEnforced(t *testing.T) {
	_, svc := setup(t)
	ctx := context.Background()
	_, err := svc.Build(partnerA, report.Params{From: day("2026-09-01"), To: day("2026-10-07"), Accounts: []string{"partner_b"}})
	e, ok := domain.AsError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, e.Status)

	q, err := svc.Build(viewer, report.Params{From: day("2026-09-01"), To: day("2026-10-07")})
	require.NoError(t, err)
	_, err = svc.Account(ctx, q, "partner_c")
	require.Error(t, err)

	acc, err := svc.Accounts(ctx, q, report.Paging{PageSize: 200})
	require.NoError(t, err)
	for _, r := range acc.Items {
		require.Contains(t, []string{"partner_a", "partner_b"}, r.Username)
	}

	// User thường: SĐT CTV bị che.
	qa, _ := svc.Build(partnerA, report.Params{From: day("2026-09-01"), To: day("2026-10-07")})
	top, err := svc.TopCTVs(ctx, qa, 10)
	require.NoError(t, err)
	for _, it := range top {
		require.NotRegexp(t, `0\d{9}`, it.Label)
	}
}

func TestRangeLimits(t *testing.T) {
	_, svc := setup(t)
	_, err := svc.Build(admin, report.Params{From: day("2025-01-01"), To: day("2026-10-07")})
	require.Error(t, err)
	_, err = svc.Build(admin, report.Params{From: day("2026-10-07"), To: day("2026-10-01")})
	require.Error(t, err)
}
