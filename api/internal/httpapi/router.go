// Package httpapi lắp router chi: /v1/* (sinh từ OpenAPI), /healthz, /readyz, /metrics.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	nethttpmw "github.com/oapi-codegen/nethttp-middleware"
	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"am-shortlink-portal/api/internal/auth"
	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/httpapi/handler"
	mw "am-shortlink-portal/api/internal/httpapi/middleware"
	"am-shortlink-portal/api/internal/httpapi/problem"
)

type Deps struct {
	Handler         *handler.Handler
	Tokens          *auth.Tokens
	Logger          *slog.Logger
	Ready           func(ctx context.Context) error
	Metrics         http.Handler
	LoginRatePerMin int
	Redis           *goredis.Client // nil = không có Redis
	RedisPrefix     string
	// Extra cho phép gắn route ngoài spec (vd tải file export dạng stream).
	Extra func(r chi.Router)
}

func NewRouter(d Deps) (http.Handler, error) {
	spec, err := gen.GetSwagger()
	if err != nil {
		return nil, err
	}
	spec.Servers = nil // validator không so host

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(spanRouteName)
	r.Use(requestIDHeader)
	r.Use(mw.Recover)
	r.Use(mw.AccessLog(d.Logger))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		problem.Write(w, r, domain.NotFound("route_not_found", "không có endpoint này"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		problem.Write(w, r, &domain.Error{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Title: "Sai phương thức"})
	})

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Ready(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	if d.Metrics != nil {
		r.Handle("/metrics", d.Metrics)
	}

	r.Group(func(r chi.Router) {
		r.Use(mw.LoginRateLimit(d.LoginRatePerMin, d.Redis, d.RedisPrefix))
		r.Use(mw.Authenticate(d.Tokens))
		r.Use(handler.ClientInfo)
		r.Use(nethttpmw.OapiRequestValidatorWithOptions(spec, &nethttpmw.Options{
			Options: openapi3filter.Options{
				// Xác thực do mw.Authenticate làm; validator chỉ kiểm tham số / body.
				AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
				MultiError:         true,
			},
			SilenceServersWarning: true,
			ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmw.ErrorHandlerOpts) {
				if opts.StatusCode == http.StatusNotFound {
					problem.Write(w, r, domain.NotFound("route_not_found", "không có endpoint này"))
					return
				}
				problem.Write(w, r, validationError(err))
			},
		}))
		if d.Extra != nil {
			d.Extra(r)
		}
		strict := gen.NewStrictHandlerWithOptions(d.Handler, nil, gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				problem.Write(w, r, domain.BadRequest("invalid_request", "dữ liệu gửi lên không hợp lệ"))
			},
			ResponseErrorHandlerFunc: problem.Write,
		})
		gen.HandlerWithOptions(strict, gen.ChiServerOptions{
			BaseRouter: r,
			ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				problem.Write(w, r, domain.BadRequest("invalid_parameter", err.Error()))
			},
		})
	})

	return otelhttp.NewHandler(r, "portal-api",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method }),
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && r.URL.Path != "/metrics"
		}),
	), nil
}

// spanRouteName đổi tên span theo route template (/v1/reports/links/{code}), không theo URL thật.
func spanRouteName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
			trace.SpanFromContext(r.Context()).SetName(r.Method + " " + rc.RoutePattern())
		}
	})
}

// requestIDHeader trả X-Request-Id cho client (BFF hiển thị khi báo lỗi).
func requestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", chimw.GetReqID(r.Context()))
		next.ServeHTTP(w, r)
	})
}

// validationError chuyển lỗi kin-openapi thành danh sách trường lỗi dễ đọc.
func validationError(err error) error {
	de := domain.BadRequest("validation_failed", "tham số hoặc dữ liệu không hợp lệ")
	var multi openapi3.MultiError
	errs := []error{err}
	if errors.As(err, &multi) {
		errs = multi
	}
	for _, e := range errs {
		field, msg := "", e.Error()
		var re *openapi3filter.RequestError
		if errors.As(e, &re) {
			if re.Parameter != nil {
				field = re.Parameter.Name
			} else if re.RequestBody != nil {
				field = "body"
			}
			msg = re.Reason
			var se *openapi3.SchemaError
			if errors.As(re.Err, &se) {
				if p := se.JSONPointer(); len(p) > 0 {
					field = strings.Join(p, ".")
				}
				msg = se.Reason
			}
			if msg == "" && re.Err != nil {
				msg = re.Err.Error()
			}
		}
		de.Fields = append(de.Fields, domain.FieldError{Field: field, Message: msg})
	}
	return de
}
