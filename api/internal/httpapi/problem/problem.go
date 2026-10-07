// Package problem ghi lỗi theo RFC 9457 (application/problem+json) kèm request_id.
// Không bao giờ trả stack trace / chi tiết lỗi nội bộ ra client.
package problem

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/store"
)

const ContentType = "application/problem+json"

// Write chuyển err thành problem+json. Lỗi không phải domain.Error → 500 (log đầy đủ phía server).
func Write(w http.ResponseWriter, r *http.Request, err error) {
	reqID := middleware.GetReqID(r.Context())
	p := gen.Problem{Type: "about:blank", RequestId: &reqID}

	if de, ok := domain.AsError(err); ok {
		p.Status = de.Status
		p.Title = de.Title
		p.Code = strPtr(de.Code)
		if de.Detail != "" {
			p.Detail = strPtr(de.Detail)
		}
		if len(de.Fields) > 0 {
			fe := make([]struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			}, len(de.Fields))
			for i, f := range de.Fields {
				fe[i].Field, fe[i].Message = f.Field, f.Message
			}
			p.Errors = &fe
		}
	} else if store.IsTimeout(err) {
		p.Status = http.StatusGatewayTimeout
		p.Title = "Truy vấn quá thời gian"
		p.Code = strPtr("query_timeout")
		p.Detail = strPtr("truy vấn vượt giới hạn thời gian — hãy thu hẹp khoảng ngày hoặc bộ lọc")
		slog.WarnContext(r.Context(), "query timeout", "path", r.URL.Path, "err", err)
	} else if r.Context().Err() != nil {
		// Client huỷ request (người dùng huỷ truy vấn dài) — không phải lỗi server.
		p.Status = 499
		p.Title = "Đã huỷ"
		p.Code = strPtr("client_closed_request")
	} else {
		p.Status = http.StatusInternalServerError
		p.Title = "Lỗi hệ thống"
		p.Code = strPtr("internal_error")
		slog.ErrorContext(r.Context(), "unhandled error", "path", r.URL.Path, "err", err, "request_id", reqID)
	}

	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

func strPtr(s string) *string { return &s }
