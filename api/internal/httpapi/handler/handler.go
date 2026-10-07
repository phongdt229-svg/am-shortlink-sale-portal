// Package handler implement gen.StrictServerInterface.
// Handler mỏng: lấy Principal → gọi use-case / store qua scope → map sang DTO. Không dựng truy vấn Mongo.
// Lỗi trả về dạng error (domain.Error) — router chuyển thành problem+json.
package handler

import (
	"context"
	"net/http"

	"am-shortlink-portal/api/internal/audit"
	"am-shortlink-portal/api/internal/auth"
	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/httpapi/middleware"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/savedreport"
	"am-shortlink-portal/api/internal/scope"
	"am-shortlink-portal/api/internal/store"
)

type Handler struct {
	Auth    *auth.Service
	Store   *store.Store
	Masker  *mask.Masker
	Reports *report.Service
	Saved   *savedreport.Service
	Audit   *audit.Logger
}

var _ gen.StrictServerInterface = (*Handler)(nil)

// principal lấy người dùng đã xác thực (middleware Authenticate bảo đảm có).
func principal(ctx context.Context) (domain.Principal, error) {
	p, ok := domain.PrincipalFrom(ctx)
	if !ok {
		return domain.Principal{}, domain.Unauthorized("missing_token", "chưa đăng nhập")
	}
	return p, nil
}

func scopeOf(ctx context.Context) (domain.Principal, scope.Scope, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, scope.Scope{}, err
	}
	return p, scope.For(p), nil
}

func toMe(p domain.Principal) gen.Me {
	s := scope.For(p)
	me := gen.Me{
		Username:    p.Username,
		Role:        gen.Role(p.Role),
		AllAccounts: s.All(),
		Accounts:    s.Accounts(),
	}
	if me.Accounts == nil {
		me.Accounts = []string{}
	}
	if p.Email != "" {
		me.Email = &p.Email
	}
	return me
}

func deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// ---- client info (IP, User-Agent) cho đăng nhập ----

type clientInfoKey struct{}

// ClientInfo middleware gắn IP + UA vào context để strict handler dùng.
func ClientInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ci := auth.ClientInfo{IP: middleware.ClientIP(r), UserAgent: r.UserAgent()}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientInfoKey{}, ci)))
	})
}

func clientInfo(ctx context.Context) auth.ClientInfo {
	ci, _ := ctx.Value(clientInfoKey{}).(auth.ClientInfo)
	return ci
}
