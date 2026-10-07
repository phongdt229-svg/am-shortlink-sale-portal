// Package middleware: xác thực JWT, recover, access log, rate limit đăng nhập.
package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"am-shortlink-portal/api/internal/auth"
	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/problem"
)

// PublicPaths không cần JWT.
var PublicPaths = map[string]bool{
	"/v1/auth/login":   true,
	"/v1/auth/refresh": true,
	"/v1/auth/logout":  true,
}

// Authenticate kiểm Bearer JWT cho mọi /v1/* (trừ PublicPaths) và gắn Principal vào context.
func Authenticate(tokens *auth.Tokens) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if PublicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			h := r.Header.Get("Authorization")
			tok, ok := strings.CutPrefix(h, "Bearer ")
			if !ok || tok == "" {
				problem.Write(w, r, domain.Unauthorized("missing_token", "thiếu access token"))
				return
			}
			p, err := tokens.Verify(tok)
			if err != nil {
				problem.Write(w, r, domain.Unauthorized("invalid_token", "access token không hợp lệ hoặc đã hết hạn"))
				return
			}
			if h, ok := r.Context().Value(userHolderKey{}).(*string); ok {
				*h = p.Username
			}
			next.ServeHTTP(w, r.WithContext(domain.WithPrincipal(r.Context(), p)))
		})
	}
}

// Recover bắt panic → 500 problem+json, log stack phía server.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.ErrorContext(r.Context(), "panic", "panic", fmt.Sprint(rec), "stack", string(debug.Stack()),
					"request_id", chimw.GetReqID(r.Context()))
				problem.Write(w, r, fmt.Errorf("panic: %v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type userHolderKey struct{}

// AccessLog ghi 1 dòng / request (không ghi query string — có thể chứa SĐT).
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			var user string // Authenticate điền vào (principal nằm ở context bên trong)
			next.ServeHTTP(ww, r.WithContext(context.WithValue(r.Context(), userHolderKey{}, &user)))
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" {
				return
			}
			level := slog.LevelInfo
			if ww.Status() >= 500 {
				level = slog.LevelError
			}
			log.Log(r.Context(), level, "http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", chimw.GetReqID(r.Context()),
				"user", user,
				"client_ip", clientIP(r),
			)
		})
	}
}

// LoginRateLimit giới hạn số lần gọi đăng nhập theo IP.
// Có Redis: đếm chung mọi replica (cửa sổ cố định 1 phút, INCR + EXPIRE trong 1 pipeline);
// Redis lỗi hoặc không cấu hình → giới hạn trong bộ nhớ của pod.
func LoginRateLimit(perMinute int, rdb *goredis.Client, prefix string) func(http.Handler) http.Handler {
	type entry struct {
		lim  *rate.Limiter
		seen time.Time
	}
	var (
		mu      sync.Mutex
		entries = map[string]*entry{}
		lastGC  = time.Now()
	)
	every := rate.Every(time.Minute / time.Duration(max(perMinute, 1)))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/auth/login" {
				next.ServeHTTP(w, r)
				return
			}
			ip := clientIP(r)
			now := time.Now()
			if rdb != nil {
				key := fmt.Sprintf("%sratelimit:login:%s:%d", prefix, ip, now.Unix()/60)
				ctx, cancel := context.WithTimeout(r.Context(), 150*time.Millisecond)
				pipe := rdb.TxPipeline()
				incr := pipe.Incr(ctx, key)
				pipe.Expire(ctx, key, 70*time.Second)
				_, err := pipe.Exec(ctx)
				cancel()
				if err == nil {
					if incr.Val() > int64(perMinute) {
						w.Header().Set("Retry-After", "60")
						problem.Write(w, r, domain.TooManyRequests("rate_limited", "quá nhiều lần đăng nhập, thử lại sau"))
						return
					}
					next.ServeHTTP(w, r)
					return
				}
				slog.WarnContext(r.Context(), "rate limit redis failed, fallback in-memory", "err", err)
			}
			mu.Lock()
			if now.Sub(lastGC) > 5*time.Minute {
				for k, e := range entries {
					if now.Sub(e.seen) > 10*time.Minute {
						delete(entries, k)
					}
				}
				lastGC = now
			}
			e, ok := entries[ip]
			if !ok {
				e = &entry{lim: rate.NewLimiter(every, perMinute)}
				entries[ip] = e
			}
			e.seen = now
			allowed := e.lim.Allow()
			mu.Unlock()
			if !allowed {
				w.Header().Set("Retry-After", "60")
				problem.Write(w, r, domain.TooManyRequests("rate_limited", "quá nhiều lần đăng nhập, thử lại sau"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP: portal-api chỉ nhận request từ BFF trong cụm; BFF chuyển IP trình duyệt qua X-Forwarded-For.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ClientIP export cho handler (ghi vào refresh token / audit).
func ClientIP(r *http.Request) string { return clientIP(r) }
