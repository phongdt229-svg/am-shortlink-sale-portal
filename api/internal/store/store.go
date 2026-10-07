// Package store là repository MongoDB của portal-api — NƠI DUY NHẤT import driver.
//
//   - am_shortlink (Core): CHỈ ĐỌC — users, links, campaigns, prefixes, clicks.
//   - am_shortlink_report (Report): đọc stats_*, link_params…; ghi chỉ portal_* và param_registry.
//
// Mọi truy vấn đi qua s.ctx(): đặt deadline = MONGODB_MAX_TIME → driver gửi maxTimeMS (CSOT).
package store

import (
	"context"
	"errors"
	"regexp"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"am-shortlink-portal/api/internal/platform/mongodb"
)

// Tên collection — hợp đồng schema với Service (docs/PLAN_SERVICE_API.md §4, §5.4, §5.4b).
const (
	// am_shortlink
	CollUsers     = "users"
	CollLinks     = "links"
	CollCampaigns = "campaigns"
	CollPrefixes  = "prefixes"
	CollClicks    = "clicks"

	// am_shortlink_report — Service ghi
	CollStatsSystemDaily   = "stats_system_daily"
	CollStatsOwnerDaily    = "stats_owner_daily"
	CollStatsCampaignDaily = "stats_campaign_daily"
	CollStatsCTVDaily      = "stats_ctv_daily"
	CollStatsLinkDaily     = "stats_link_daily"
	CollStatsParamDaily    = "stats_param_daily"
	CollLinkParams         = "link_params"
	CollParamValues        = "param_values"
	CollClickFacets        = "click_facets"
	CollCTVs               = "ctvs"
	CollParamRegistry      = "param_registry" // Portal (admin) ghi

	// am_shortlink_report — Portal sở hữu
	CollRefreshTokens = "portal_refresh_tokens"
	CollLoginAttempts = "portal_login_attempts"
	CollSavedReports  = "portal_saved_reports"
	CollExports       = "portal_exports"
	CollAuditLog      = "portal_audit_log"
	CollMigrations    = "portal_schema_migrations"
)

type Store struct {
	db *mongodb.DB
}

func New(db *mongodb.DB) *Store { return &Store{db: db} }

func (s *Store) core(name string) *mongo.Collection   { return s.db.Core.Collection(name) }
func (s *Store) report(name string) *mongo.Collection { return s.db.Report.Collection(name) }

// ctx đặt giới hạn thời gian truy vấn (maxTimeMS); huỷ theo context của request.
func (s *Store) ctx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, s.db.MaxTime)
}

// containsRegex: tìm chuỗi con không phân biệt hoa thường, đã escape.
func containsRegex(q string) bson.Regex {
	return bson.Regex{Pattern: regexp.QuoteMeta(q), Options: "i"}
}

func isNoDocs(err error) bool { return errors.Is(err, mongo.ErrNoDocuments) }

// TimeoutError chuyển lỗi vượt maxTimeMS thành lỗi nghiệp vụ để gợi ý thu hẹp bộ lọc.
func IsTimeout(err error) bool {
	return mongo.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded)
}

func and(parts ...bson.D) bson.D {
	out := bson.D{}
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func clampLimit(limit, def, max int) int64 {
	if limit <= 0 {
		return int64(def)
	}
	if limit > max {
		return int64(max)
	}
	return int64(limit)
}
