// Package savedreport: "báo cáo của tôi" — lưu trạng thái bộ lọc / Explorer, chia sẻ qua link.
// Query chỉ là trạng thái màn hình; người mở chạy lại với PHẠM VI CỦA CHÍNH HỌ (scope áp lúc truy vấn).
package savedreport

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"am-shortlink-portal/api/internal/audit"
	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/store"
)

// MaxQueryBytes: giới hạn kích thước trạng thái lưu.
const MaxQueryBytes = 32 << 10

var errNotFound = domain.NotFound("saved_report_not_found", "không tìm thấy báo cáo đã lưu")

type Service struct {
	st    *store.Store
	audit *audit.Logger
}

func New(st *store.Store, a *audit.Logger) *Service { return &Service{st: st, audit: a} }

func toDTO(r store.SavedReport, viewer string) gen.SavedReport {
	q := map[string]any(r.Query)
	if q == nil {
		q = map[string]any{}
	}
	out := gen.SavedReport{
		Id: r.ID.Hex(), Name: r.Name, Kind: gen.SavedReportKind(r.Kind), Query: q, Owner: r.Owner,
		IsMine: r.Owner == viewer, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if r.Description != "" {
		out.Description = &r.Description
	}
	if r.ShareToken != "" && r.Owner == viewer {
		out.ShareToken = &r.ShareToken
	}
	return out
}

func validate(in gen.SavedReportInput) (bson.M, error) {
	b, err := json.Marshal(in.Query)
	if err != nil || len(b) > MaxQueryBytes {
		return nil, domain.BadRequest("query_too_large", "trạng thái báo cáo quá lớn (tối đa 32KB)")
	}
	var m bson.M
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, domain.BadRequest("invalid_query", "trạng thái báo cáo không hợp lệ")
	}
	return m, nil
}

func (s *Service) List(ctx context.Context, p domain.Principal) (gen.SavedReportList, error) {
	rows, err := s.st.ListSaved(ctx, p.Username)
	if err != nil {
		return gen.SavedReportList{}, err
	}
	out := gen.SavedReportList{Items: make([]gen.SavedReport, len(rows))}
	for i, r := range rows {
		out.Items[i] = toDTO(r, p.Username)
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, p domain.Principal, in gen.SavedReportInput) (gen.SavedReport, error) {
	q, err := validate(in)
	if err != nil {
		return gen.SavedReport{}, err
	}
	n, err := s.st.CountSaved(ctx, p.Username)
	if err != nil {
		return gen.SavedReport{}, err
	}
	if n >= store.MaxSavedPerUser {
		return gen.SavedReport{}, domain.Unprocessable("too_many_saved_reports", "đã đạt tối đa 200 báo cáo đã lưu — hãy xoá bớt")
	}
	now := time.Now().UTC()
	r := store.SavedReport{Owner: p.Username, Name: in.Name, Kind: string(in.Kind), Query: q, CreatedAt: now, UpdatedAt: now}
	if in.Description != nil {
		r.Description = *in.Description
	}
	if err := s.st.InsertSaved(ctx, &r); err != nil {
		return gen.SavedReport{}, err
	}
	return toDTO(r, p.Username), nil
}

func (s *Service) Get(ctx context.Context, p domain.Principal, id string) (gen.SavedReport, error) {
	r, err := s.st.GetSaved(ctx, p.Username, id)
	if err != nil {
		return gen.SavedReport{}, err
	}
	if r == nil {
		return gen.SavedReport{}, errNotFound
	}
	return toDTO(*r, p.Username), nil
}

func (s *Service) GetShared(ctx context.Context, p domain.Principal, token string) (gen.SavedReport, error) {
	r, err := s.st.GetSavedByToken(ctx, token)
	if err != nil {
		return gen.SavedReport{}, err
	}
	if r == nil {
		return gen.SavedReport{}, errNotFound
	}
	return toDTO(*r, p.Username), nil
}

func (s *Service) Update(ctx context.Context, p domain.Principal, id string, in gen.SavedReportInput) (gen.SavedReport, error) {
	r, err := s.st.GetSaved(ctx, p.Username, id)
	if err != nil {
		return gen.SavedReport{}, err
	}
	if r == nil {
		return gen.SavedReport{}, errNotFound
	}
	q, err := validate(in)
	if err != nil {
		return gen.SavedReport{}, err
	}
	r.Name, r.Kind, r.Query, r.UpdatedAt = in.Name, string(in.Kind), q, time.Now().UTC()
	r.Description = ""
	if in.Description != nil {
		r.Description = *in.Description
	}
	ok, err := s.st.UpdateSaved(ctx, r)
	if err != nil {
		return gen.SavedReport{}, err
	}
	if !ok {
		return gen.SavedReport{}, errNotFound
	}
	return toDTO(*r, p.Username), nil
}

func (s *Service) Delete(ctx context.Context, p domain.Principal, id string) error {
	ok, err := s.st.DeleteSaved(ctx, p.Username, id)
	if err != nil {
		return err
	}
	if !ok {
		return errNotFound
	}
	return nil
}

// Share bật (sinh token ngẫu nhiên) / tắt (thu hồi token) chia sẻ.
func (s *Service) Share(ctx context.Context, p domain.Principal, id string, enabled bool) (gen.SavedReport, error) {
	r, err := s.st.GetSaved(ctx, p.Username, id)
	if err != nil {
		return gen.SavedReport{}, err
	}
	if r == nil {
		return gen.SavedReport{}, errNotFound
	}
	r.ShareToken = ""
	if enabled {
		b := make([]byte, 18)
		if _, err := rand.Read(b); err != nil {
			return gen.SavedReport{}, err
		}
		r.ShareToken = base64.RawURLEncoding.EncodeToString(b)
	}
	r.UpdatedAt = time.Now().UTC()
	if _, err := s.st.UpdateSaved(ctx, r); err != nil {
		return gen.SavedReport{}, err
	}
	s.audit.Record(ctx, p, "saved_report.share", r.ID.Hex(), map[string]any{"enabled": enabled, "name": r.Name})
	return toDTO(*r, p.Username), nil
}
