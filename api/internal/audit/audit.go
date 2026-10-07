// Package audit ghi nhật ký thao tác vào portal_audit_log (ai, làm gì, lúc nào, với đối tượng nào).
// Lỗi ghi nhật ký không làm hỏng thao tác chính — chỉ log cảnh báo.
package audit

import (
	"context"
	"log/slog"
	"time"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/store"
)

type Logger struct{ st *store.Store }

func New(st *store.Store) *Logger { return &Logger{st: st} }

// Record: action vd "saved_report.share", "param_registry.update", "export.create".
func (l *Logger) Record(ctx context.Context, p domain.Principal, action, target string, detail map[string]any) {
	if l == nil {
		return
	}
	err := l.st.InsertAudit(ctx, store.AuditEntry{
		TS: time.Now(), Actor: p.Username, Role: string(p.Role), Action: action, Target: target, Detail: detail,
	})
	if err != nil {
		slog.WarnContext(ctx, "audit log failed", "action", action, "err", err)
	}
}
