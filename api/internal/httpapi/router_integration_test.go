//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"am-shortlink-portal/api/internal/auth"
	"am-shortlink-portal/api/internal/cache"
	"am-shortlink-portal/api/internal/export"
	"am-shortlink-portal/api/internal/httpapi"
	"am-shortlink-portal/api/internal/httpapi/handler"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/savedreport"
	"am-shortlink-portal/api/internal/testutil"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testutil.Teardown()
	os.Exit(code)
}

// memCache: cache trong bộ nhớ để kiểm khoá cache theo phạm vi (thay Redis).
type memCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (c *memCache) Get(_ context.Context, k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}
func (c *memCache) Set(_ context.Context, k string, v []byte, _ time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
}
func (c *memCache) Del(context.Context, ...string) {}

var _ cache.Cache = (*memCache)(nil)

func server(t *testing.T) *httptest.Server {
	env := testutil.Seeded(t)
	m := mask.NewMasker(testutil.Salt)
	tokens := auth.NewTokens("integration-jwt-key-0123456789abcdef0123", "test", 15*time.Minute)
	rep := report.NewService(env.Store, m).WithShortURLBase("https://s.test").WithCache(&memCache{m: map[string][]byte{}}, time.Minute, time.Minute)
	h := &handler.Handler{
		Auth:    auth.NewService(env.Store, tokens, auth.Options{MaxFailedLogins: 5, LockDuration: time.Minute, RefreshTTL: time.Hour}),
		Store:   env.Store, Masker: m, Reports: rep,
		Saved:   savedreport.New(env.Store, nil),
		Exports: export.New(env.Store, rep, t.TempDir(), time.Hour, nil),
	}
	r, err := httpapi.NewRouter(httpapi.Deps{Handler: h, Tokens: tokens, Logger: slog.New(slog.DiscardHandler), Ready: env.DB.Ping, LoginRatePerMin: 1000})
	require.NoError(t, err)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

type client struct {
	t     *testing.T
	base  string
	token string
}

func (c *client) do(method, path string, body any) (int, map[string]any, http.Header) {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return res.StatusCode, out, res.Header
}

func login(t *testing.T, base, user, pass string) *client {
	c := &client{t: t, base: base}
	code, body, _ := c.do("POST", "/v1/auth/login", map[string]string{"username": user, "password": pass})
	require.Equal(t, 200, code, body)
	c.token = body["access_token"].(string)
	return c
}

const R = "from=2026-09-10&to=2026-10-07"

func TestHTTPAuthAndProblemJSON(t *testing.T) {
	srv := server(t)
	anon := &client{t: t, base: srv.URL}

	code, body, hdr := anon.do("GET", "/v1/me", nil)
	require.Equal(t, 401, code)
	require.Equal(t, "missing_token", body["code"])
	require.NotEmpty(t, body["request_id"])
	require.NotEmpty(t, hdr.Get("X-Request-Id"))

	code, body, _ = anon.do("POST", "/v1/auth/login", map[string]string{"username": "admin", "password": "wrong"})
	require.Equal(t, 401, code)
	require.Equal(t, "invalid_credentials", body["code"])

	u := login(t, srv.URL, "partner_a", "partner123")
	code, body, _ = u.do("GET", "/v1/reports/overview?from=2026-10-07", nil)
	require.Equal(t, 400, code, "thiếu `to` bị validator chặn")
	require.Equal(t, "validation_failed", body["code"])

	code, _, _ = u.do("GET", "/v1/reports/overview?"+R+"&granularity=year", nil)
	require.Equal(t, 400, code)

	code, body, _ = u.do("GET", "/v1/reports/overview?from=2024-01-01&to=2026-10-07", nil)
	require.Equal(t, 400, code)
	require.Equal(t, "range_too_long", body["code"])
}

func TestHTTPScopeEnforcedForEveryRole(t *testing.T) {
	srv := server(t)
	a := login(t, srv.URL, "partner_a", "partner123")
	v := login(t, srv.URL, "viewer", "viewer123")
	adm := login(t, srv.URL, "admin", "admin123")

	// Gọi thẳng API (bỏ qua giao diện) với tài khoản ngoài phạm vi → 403.
	for _, p := range []string{
		"/v1/reports/overview?" + R + "&account=partner_b",
		"/v1/reports/accounts/partner_b?" + R,
		"/v1/reports/campaigns?" + R + "&account=partner_c",
		"/v1/filters/campaigns?account=partner_b",
	} {
		code, body, _ := a.do("GET", p, nil)
		require.Equal(t, 403, code, p)
		require.Equal(t, "forbidden_scope", body["code"], p)
	}
	code, _, _ := v.do("GET", "/v1/reports/accounts/partner_c?"+R, nil)
	require.Equal(t, 403, code)

	// Admin-only.
	code, _, _ = a.do("GET", "/v1/reports/traffic-quality?"+R, nil)
	require.Equal(t, 403, code)
	code, _, _ = v.do("GET", "/v1/param-registry", nil)
	require.Equal(t, 403, code)
	code, _, _ = adm.do("GET", "/v1/reports/traffic-quality?"+R, nil)
	require.Equal(t, 200, code)

	// Cache theo phạm vi: admin gọi trước (cache toàn hệ thống), user gọi sau không được nhận số của admin.
	_, admOv, _ := adm.do("GET", "/v1/reports/overview?"+R, nil)
	_, userOv, _ := a.do("GET", "/v1/reports/overview?"+R, nil)
	ac := admOv["summary"].(map[string]any)["kpis"].(map[string]any)["current"].(map[string]any)["clicks"].(float64)
	uc := userOv["summary"].(map[string]any)["kpis"].(map[string]any)["current"].(map[string]any)["clicks"].(float64)
	require.Less(t, uc, ac, "user chỉ thấy số của mình dù admin đã làm nóng cache")
	_, again, _ := a.do("GET", "/v1/reports/overview?"+R, nil)
	require.Equal(t, uc, again["summary"].(map[string]any)["kpis"].(map[string]any)["current"].(map[string]any)["clicks"].(float64))

	// Không có SĐT bản rõ trong phản hồi của user thường.
	_, ctvs, _ := a.do("GET", "/v1/reports/ctvs?"+R, nil)
	b, _ := json.Marshal(ctvs)
	require.NotRegexp(t, `"ctv_display":"0\d{9}"`, string(b))
}

func TestHTTPQRCodeAndExplorer(t *testing.T) {
	srv := server(t)
	a := login(t, srv.URL, "partner_a", "partner123")
	_, top, _ := a.do("GET", "/v1/reports/links/top?"+R+"&page_size=1", nil)
	code := top["items"].([]any)[0].(map[string]any)["link"].(map[string]any)["code"].(string)

	req, _ := http.NewRequest("GET", srv.URL+"/v1/links/"+code+"/qrcode?size=200", nil)
	req.Header.Set("Authorization", "Bearer "+a.token)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	png, _ := io.ReadAll(res.Body)
	res.Body.Close()
	require.Equal(t, 200, res.StatusCode)
	require.Equal(t, "image/png", res.Header.Get("Content-Type"))
	require.True(t, bytes.HasPrefix(png, []byte("\x89PNG")))

	st, body, _ := a.do("POST", "/v1/reports/clicks/query", map[string]any{
		"from": "2026-09-20", "to": "2026-10-07", "group_by": []string{"device", "param.utm_source"},
		"filters": map[string]any{"quality": "valid", "device": map[string]any{"values": []string{"bot"}, "exclude": true}},
	})
	require.Equal(t, 200, st, body)
	require.Equal(t, "clicks", body["source"])

	st, body, _ = a.do("POST", "/v1/reports/clicks/query", map[string]any{"from": "2026-09-20", "to": "2026-10-07", "group_by": []string{"a", "b", "c", "d"}})
	require.Equal(t, 400, st, body)
	require.True(t, strings.Contains(strings.ToLower(body["code"].(string)), "valid"))
}
