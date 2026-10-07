package store

import (
	"context"
	"time"
)

// AuditEntry: portal_audit_log.
type AuditEntry struct {
	TS     time.Time      `bson:"ts"`
	Actor  string         `bson:"actor"`
	Role   string         `bson:"role"`
	Action string         `bson:"action"`
	Target string         `bson:"target"`
	Detail map[string]any `bson:"detail,omitempty"`
}

func (s *Store) InsertAudit(ctx context.Context, e AuditEntry) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	_, err := s.report(CollAuditLog).InsertOne(ctx, e)
	return err
}
