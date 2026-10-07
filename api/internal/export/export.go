// Package export: R8 — job xuất CSV / XLSX chạy nền.
//
//   - Tạo job: lưu ảnh chụp người tạo (vai trò + tài khoản được xem) → worker chạy với ĐÚNG phạm vi đó,
//     số liệu lấy từ cùng use-case report (khớp màn hình), SĐT / IP che như trên màn hình.
//   - Worker: claim nguyên tử (findOneAndUpdate), heartbeat; worker chết → job được nhận lại.
//   - File giữ ExportTTL (7 ngày); bản ghi tự xoá bằng TTL index, file dọn định kỳ. Tối đa 1 triệu dòng.
package export

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"am-shortlink-portal/api/internal/audit"
	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/store"
)

const (
	MaxRows          = 1_000_000
	MaxActivePerUser = 3
	staleAfter       = 2 * time.Minute
)

var kindLabel = map[string]string{
	"accounts": "tai-khoan", "campaigns": "chien-dich", "ctvs": "ctv", "links_top": "link",
	"clicks": "click-tho", "explorer": "phan-tich-click",
}

type Service struct {
	st      *store.Store
	reports *report.Service
	dir     string
	ttl     time.Duration
	audit   *audit.Logger
	now     func() time.Time
}

func New(st *store.Store, r *report.Service, dir string, ttl time.Duration, a *audit.Logger) *Service {
	return &Service{st: st, reports: r, dir: dir, ttl: ttl, audit: a, now: time.Now}
}

func toDTO(j store.ExportJob) gen.ExportJob {
	out := gen.ExportJob{
		Id: j.ID.Hex(), Kind: j.Kind, Format: j.Format, Name: j.Name, Status: gen.ExportJobStatus(j.Status),
		CreatedBy: j.CreatedBy, CreatedAt: j.CreatedAt, ExpiresAt: j.ExpiresAt, FinishedAt: j.FinishedAt,
	}
	if j.Status == "done" && time.Now().After(j.ExpiresAt) {
		out.Status = "expired"
	}
	if j.Rows > 0 {
		out.Rows = &j.Rows
	}
	if j.SizeBytes > 0 {
		out.SizeBytes = &j.SizeBytes
	}
	if j.Error != "" {
		out.Error = &j.Error
	}
	return out
}

// Create: kiểm quyền + tham số ngay (lỗi trả về liền, không để job thất bại sau).
func (s *Service) Create(ctx context.Context, p domain.Principal, req gen.ExportRequest) (gen.ExportJob, error) {
	kind := string(req.Kind)
	if kind == "clicks" && p.Role == domain.RoleViewer {
		return gen.ExportJob{}, domain.Forbidden("raw_export_forbidden", "vai trò xem báo cáo không được xuất click thô")
	}
	if _, err := s.query(p, req.Params); err != nil {
		return gen.ExportJob{}, err
	}
	n, err := s.st.CountActiveExports(ctx, p.Username)
	if err != nil {
		return gen.ExportJob{}, err
	}
	if n >= MaxActivePerUser {
		return gen.ExportJob{}, domain.TooManyRequests("too_many_exports", "đang có 3 job chưa xong — chờ xong rồi tạo tiếp")
	}
	params := bson.M{}
	b, _ := json.Marshal(req.Params)
	_ = json.Unmarshal(b, &params)
	name := kindLabel[kind]
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}
	now := s.now().UTC()
	j := store.ExportJob{
		Kind: kind, Format: string(req.Format), Name: name, Params: params, Status: "queued",
		CreatedBy: p.Username, Role: string(p.Role), ViewerAccounts: p.ViewerAccounts,
		CreatedAt: now, ExpiresAt: now.Add(s.ttl),
	}
	if err := s.st.InsertExport(ctx, &j); err != nil {
		return gen.ExportJob{}, err
	}
	s.audit.Record(ctx, p, "export.create", j.ID.Hex(), map[string]any{"kind": kind, "format": j.Format, "params": params})
	return toDTO(j), nil
}

func (s *Service) List(ctx context.Context, p domain.Principal) (gen.ExportJobList, error) {
	owner := p.Username
	if p.IsAdmin() {
		owner = ""
	}
	jobs, err := s.st.ListExports(ctx, owner, 200)
	if err != nil {
		return gen.ExportJobList{}, err
	}
	out := gen.ExportJobList{Items: make([]gen.ExportJob, len(jobs))}
	for i, j := range jobs {
		out.Items[i] = toDTO(j)
	}
	return out, nil
}

var errNotFound = domain.NotFound("export_not_found", "không tìm thấy job xuất dữ liệu")

func (s *Service) visible(ctx context.Context, p domain.Principal, id string) (*store.ExportJob, error) {
	j, err := s.st.GetExport(ctx, id)
	if err != nil {
		return nil, err
	}
	if j == nil || (j.CreatedBy != p.Username && !p.IsAdmin()) {
		return nil, errNotFound
	}
	return j, nil
}

func (s *Service) Get(ctx context.Context, p domain.Principal, id string) (gen.ExportJob, error) {
	j, err := s.visible(ctx, p, id)
	if err != nil {
		return gen.ExportJob{}, err
	}
	return toDTO(*j), nil
}

// Download mở file; trả tên file gợi ý cho Content-Disposition.
func (s *Service) Download(ctx context.Context, p domain.Principal, id string) (*os.File, int64, string, error) {
	j, err := s.visible(ctx, p, id)
	if err != nil {
		return nil, 0, "", err
	}
	if j.Status != "done" {
		return nil, 0, "", domain.Conflict("export_not_ready", "file chưa sẵn sàng (trạng thái "+j.Status+")")
	}
	if s.now().After(j.ExpiresAt) {
		return nil, 0, "", &domain.Error{Status: 410, Code: "export_expired", Title: "Đã hết hạn", Detail: "file đã quá 7 ngày — hãy tạo job mới"}
	}
	f, err := os.Open(filepath.Join(s.dir, filepath.Base(j.File)))
	if err != nil {
		return nil, 0, "", &domain.Error{Status: 410, Code: "export_file_missing", Title: "Không còn file", Detail: "file đã bị dọn — hãy tạo job mới"}
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, "", err
	}
	s.audit.Record(ctx, p, "export.download", id, map[string]any{"kind": j.Kind})
	return f, st.Size(), fileName(j), nil
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func fileName(j *store.ExportJob) string {
	base := unsafeName.ReplaceAllString(asciiFold(j.Name), "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "export"
	}
	return fmt.Sprintf("%s_%s.%s", base, j.CreatedAt.In(store.VN).Format("20060102-1504"), j.Format)
}

// asciiFold bỏ dấu tiếng Việt cho tên file.
func asciiFold(s string) string {
	repl := strings.NewReplacer(
		"à", "a", "á", "a", "ả", "a", "ã", "a", "ạ", "a", "ă", "a", "ằ", "a", "ắ", "a", "ẳ", "a", "ẵ", "a", "ặ", "a",
		"â", "a", "ầ", "a", "ấ", "a", "ẩ", "a", "ẫ", "a", "ậ", "a", "đ", "d", "è", "e", "é", "e", "ẻ", "e", "ẽ", "e",
		"ẹ", "e", "ê", "e", "ề", "e", "ế", "e", "ể", "e", "ễ", "e", "ệ", "e", "ì", "i", "í", "i", "ỉ", "i", "ĩ", "i",
		"ị", "i", "ò", "o", "ó", "o", "ỏ", "o", "õ", "o", "ọ", "o", "ô", "o", "ồ", "o", "ố", "o", "ổ", "o", "ỗ", "o",
		"ộ", "o", "ơ", "o", "ờ", "o", "ớ", "o", "ở", "o", "ỡ", "o", "ợ", "o", "ù", "u", "ú", "u", "ủ", "u", "ũ", "u",
		"ụ", "u", "ư", "u", "ừ", "u", "ứ", "u", "ử", "u", "ữ", "u", "ự", "u", "ỳ", "y", "ý", "y", "ỷ", "y", "ỹ", "y", "ỵ", "y",
	)
	return repl.Replace(strings.ToLower(s))
}

// ---------- worker ----------

// RunWorker xử lý job cho đến khi ctx huỷ. Gọi trong goroutine riêng.
func (s *Service) RunWorker(ctx context.Context, id string) {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		slog.Error("export dir", "err", err)
		return
	}
	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	s.cleanup()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanup.C:
			s.cleanup()
		default:
		}
		j, err := s.st.ClaimExport(ctx, id, s.now().UTC(), staleAfter)
		if err != nil && ctx.Err() == nil {
			slog.Warn("claim export", "err", err)
		}
		if j == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		s.process(ctx, j)
	}
}

func (s *Service) process(ctx context.Context, j *store.ExportJob) {
	start := time.Now()
	file := j.ID.Hex() + "." + j.Format
	path := filepath.Join(s.dir, file)
	rows, err := s.write(ctx, j, path)
	now := s.now().UTC()
	if err != nil {
		_ = os.Remove(path)
		msg := err.Error()
		if de, ok := domain.AsError(err); ok {
			msg = de.Detail
		}
		slog.Warn("export failed", "id", j.ID.Hex(), "kind", j.Kind, "err", err)
		_ = s.st.FinishExport(context.WithoutCancel(ctx), j.ID, "failed", "", msg, rows, 0, now)
		return
	}
	var size int64
	if st, err := os.Stat(path); err == nil {
		size = st.Size()
	}
	_ = s.st.FinishExport(context.WithoutCancel(ctx), j.ID, "done", file, "", rows, size, now)
	slog.Info("export done", "id", j.ID.Hex(), "kind", j.Kind, "rows", rows, "bytes", size, "took_ms", time.Since(start).Milliseconds())
}

// cleanup xoá file quá hạn (bản ghi Mongo tự xoá bằng TTL).
func (s *Service) cleanup() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	cut := time.Now().Add(-s.ttl - time.Hour)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().Before(cut) {
			_ = os.Remove(filepath.Join(s.dir, e.Name()))
		}
	}
}

func principalOf(j *store.ExportJob) domain.Principal {
	p := domain.Principal{Username: j.CreatedBy, Role: domain.ParseRole(j.Role)}
	if p.Role == domain.RoleViewer {
		p.ViewerAccounts = j.ViewerAccounts
	}
	return p
}
