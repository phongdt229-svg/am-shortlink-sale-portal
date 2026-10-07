//go:build integration

package export

import (
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/report"
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
	partnerB = domain.Principal{Username: "partner_b", Role: domain.RoleUser}
)

func setup(t *testing.T) (*Service, *report.Service) {
	env := testutil.Seeded(t)
	r := report.NewService(env.Store, mask.NewMasker(testutil.Salt)).WithShortURLBase("https://s.test")
	return New(env.Store, r, t.TempDir(), 7*24*time.Hour, nil), r
}

// runOne: worker xử lý đúng 1 job rồi dừng.
func runOne(t *testing.T, s *Service) {
	t.Helper()
	j, err := s.st.ClaimExport(context.Background(), "test", time.Now().UTC(), staleAfter)
	require.NoError(t, err)
	require.NotNil(t, j)
	s.process(context.Background(), j)
}

func TestExportAccountsCSVMatchesReport(t *testing.T) {
	s, r := setup(t)
	ctx := context.Background()
	params := map[string]any{"from": "2026-09-10", "to": "2026-10-07"}
	job, err := s.Create(ctx, viewer, gen.ExportRequest{Kind: "accounts", Format: "csv", Params: params})
	require.NoError(t, err)
	require.Equal(t, "queued", string(job.Status))
	runOne(t, s)

	got, err := s.Get(ctx, viewer, job.Id)
	require.NoError(t, err)
	require.Equal(t, "done", string(got.Status), "error: %v", got.Error)

	f, _, name, err := s.Download(ctx, viewer, job.Id)
	require.NoError(t, err)
	defer f.Close()
	require.True(t, strings.HasSuffix(name, ".csv"))
	b, _ := io.ReadAll(f)
	require.Equal(t, []byte{0xEF, 0xBB, 0xBF}, b[:3], "có BOM UTF-8")
	recs, err := csv.NewReader(strings.NewReader(string(b[3:]))).ReadAll()
	require.NoError(t, err)

	q, _ := r.Build(viewer, report.Params{From: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)})
	page, err := r.Accounts(ctx, q, report.Paging{Page: 1, PageSize: 1000, Desc: true})
	require.NoError(t, err)
	require.Len(t, recs, len(page.Items)+1, "header + 1 dòng / tài khoản")
	for i, it := range page.Items {
		require.Equal(t, it.Username, recs[i+1][0])
	}

	// Người khác không thấy / không tải được.
	_, _, _, err = s.Download(ctx, partnerB, job.Id)
	e, ok := domain.AsError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, e.Status)
}

func TestExportClicksXLSXMasked(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	job, err := s.Create(ctx, partnerA, gen.ExportRequest{Kind: "clicks", Format: "xlsx", Params: map[string]any{
		"from": "2026-10-01", "to": "2026-10-07", "filters": map[string]any{"quality": "valid"},
	}})
	require.NoError(t, err)
	runOne(t, s)
	got, _ := s.Get(ctx, partnerA, job.Id)
	require.Equal(t, "done", string(got.Status), "error: %v", got.Error)
	require.NotNil(t, got.Rows)

	f, _, _, err := s.Download(ctx, partnerA, job.Id)
	require.NoError(t, err)
	x, err := excelize.OpenReader(f)
	require.NoError(t, err)
	rows, err := x.GetRows("Sheet1")
	require.NoError(t, err)
	require.EqualValues(t, *got.Rows+1, len(rows))
	for _, row := range rows[1:] {
		require.Regexp(t, `\.x\.x$|^x`, row[14], "IP che với user")
		require.NotRegexp(t, `0\d{9}`, row[6], "SĐT CTV che")
		require.Equal(t, "0", row[15], "chỉ click hợp lệ (không bot)")
	}
}

func TestExportPermissionsAndLimits(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	_, err := s.Create(ctx, viewer, gen.ExportRequest{Kind: "clicks", Format: "csv", Params: map[string]any{"from": "2026-10-01", "to": "2026-10-07"}})
	require.Error(t, err, "viewer không xuất click thô")

	_, err = s.Create(ctx, partnerA, gen.ExportRequest{Kind: "accounts", Format: "csv", Params: map[string]any{"from": "x"}})
	require.Error(t, err, "tham số sai báo lỗi ngay")

	_, err = s.Create(ctx, partnerA, gen.ExportRequest{Kind: "accounts", Format: "csv", Params: map[string]any{"from": "2026-10-01", "to": "2026-10-07", "account": []string{"partner_b"}}})
	require.Error(t, err, "ngoài phạm vi bị chặn khi tạo job")

	for i := 0; i < MaxActivePerUser; i++ {
		_, err = s.Create(ctx, partnerB, gen.ExportRequest{Kind: "campaigns", Format: "csv", Params: map[string]any{"from": "2026-10-01", "to": "2026-10-07"}})
		require.NoError(t, err)
	}
	_, err = s.Create(ctx, partnerB, gen.ExportRequest{Kind: "campaigns", Format: "csv", Params: map[string]any{"from": "2026-10-01", "to": "2026-10-07"}})
	e, ok := domain.AsError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusTooManyRequests, e.Status)
	for i := 0; i < MaxActivePerUser; i++ {
		runOne(t, s)
	}
}

func TestTrafficQualityAndRegistry(t *testing.T) {
	_, r := setup(t)
	ctx := context.Background()
	from, to := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

	qa, _ := r.Build(partnerA, report.Params{From: from, To: to})
	_, err := r.TrafficQuality(ctx, qa)
	require.Error(t, err, "R7 chỉ admin")

	q, _ := r.Build(admin, report.Params{From: from, To: to})
	tq, err := r.TrafficQuality(ctx, q)
	require.NoError(t, err)
	k, _ := r.Kpis(ctx, q)
	require.Equal(t, k.Current.Clicks, tq.Totals.Clicks)
	require.Equal(t, k.Current.BotClicks, tq.Totals.BotClicks)
	require.NotEmpty(t, tq.TopIps)
	for _, f := range tq.Flagged {
		require.True(t, f.IsBot || f.IsSuspicious)
		require.NotEmpty(t, f.Reasons)
	}

	_, err = r.Registry(ctx, partnerA)
	require.Error(t, err)
	list, err := r.Registry(ctx, admin)
	require.NoError(t, err)
	var sawRef bool
	for _, it := range list.Items {
		if it.Key == "ref" {
			sawRef = true
			require.False(t, it.Tracked)
		}
		if it.Key == "phonenumber" {
			require.Equal(t, []string{"*** (PII)"}, *it.Samples, "không lộ giá trị PII")
		}
	}
	require.True(t, sawRef)

	tr := true
	upd, err := r.UpdateRegistry(ctx, admin, "ref", gen.ParamRegistryUpdate{Tracked: &tr})
	require.NoError(t, err)
	require.True(t, upd.Tracked)
	require.NotNil(t, upd.Backfill)
	require.Equal(t, "pending", *upd.Backfill.Status, "bật mới → Service chạy backfill")

	_, err = r.UpdateRegistry(ctx, admin, "phonenumber", gen.ParamRegistryUpdate{Tracked: &tr})
	require.Error(t, err, "không bật theo dõi tham số PII")
	f := false
	_, err = r.UpdateRegistry(ctx, admin, "ref", gen.ParamRegistryUpdate{Tracked: &f})
	require.NoError(t, err)
}
