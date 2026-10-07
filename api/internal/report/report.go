// Package report: use-case báo cáo R1–R5. Một nơi tính số liệu (PG5) theo định nghĩa PLAN_SERVICE_API.md §5.2.
//
// Luồng: Principal + tham số → scope (403 nếu ngoài phạm vi) → store.Match → chọn mức tổng hợp (store.ChooseLevel)
// → chạy song song các truy vấn → ghép thành DTO (gen.*). Use-case trả thẳng DTO sinh từ OpenAPI để tránh
// một lớp mapping trùng lặp; handler vẫn mỏng.
package report

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"am-shortlink-portal/api/internal/audit"
	"am-shortlink-portal/api/internal/cache"
	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/scope"
	"am-shortlink-portal/api/internal/store"
)

// MaxRangeDays: số liệu tổng hợp tối đa 12 tháng.
const MaxRangeDays = 366

type Service struct {
	st      *store.Store
	masker  *mask.Masker
	now     func() time.Time
	cache   cache.Cache
	ttl     time.Duration
	ttlPast time.Duration

	shortBase string // SHORT_URL_BASE, vd https://fpt.vn

	piiMu     sync.Mutex
	piiKeys   map[string]string // tham số PII → hash | drop (đọc param_registry, làm mới 5 phút)
	piiLoaded time.Time

	audit *audit.Logger
}

// WithAudit: ghi nhật ký thao tác quản trị (param_registry).
func (s *Service) WithAudit(a *audit.Logger) *Service {
	s.audit = a
	return s
}

// WithShortURLBase: domain dựng short URL <base>/<prefix>/<code>.
func (s *Service) WithShortURLBase(base string) *Service {
	s.shortBase = base
	return s
}

// pii: bảng tham số PII hiện hành (làm mới 5 phút; lỗi đọc → giữ bản cũ / danh sách mặc định an toàn).
func (s *Service) pii(ctx context.Context) map[string]string {
	s.piiMu.Lock()
	defer s.piiMu.Unlock()
	if s.piiKeys == nil || time.Since(s.piiLoaded) > 5*time.Minute {
		if keys, err := s.st.PIIKeys(ctx); err == nil {
			s.piiKeys, s.piiLoaded = keys, time.Now()
		} else if s.piiKeys == nil {
			s.piiKeys = map[string]string{"phonenumber": "hash", "fullname": "drop", "email": "hash", "guestid": "hash"}
		}
	}
	return s.piiKeys
}

// piiHashed: tham số có pii = hash → giá trị lọc phải băm trước khi so (link_params chỉ lưu bản băm).
func (s *Service) piiHashed(ctx context.Context, key string) bool { return s.pii(ctx)[key] == "hash" }

// MaskURL che dữ liệu nhạy cảm trong long_url trước khi trả:
// tham số PII (hash / drop) → "***" với MỌI vai trò; utm_extra_ctv → SĐT che (đủ với admin).
func (s *Service) MaskURL(ctx context.Context, q Query, raw string) string {
	return maskURL(raw, s.pii(ctx), q.admin)
}

func NewService(st *store.Store, m *mask.Masker) *Service {
	return &Service{st: st, masker: m, now: time.Now, cache: cache.Noop{}}
}

// WithCache bật cache kết quả báo cáo. ttl: kỳ có hôm nay; ttlPast: kỳ đã kết thúc (số liệu không đổi nữa).
func (s *Service) WithCache(c cache.Cache, ttl, ttlPast time.Duration) *Service {
	s.cache, s.ttl, s.ttlPast = c, ttl, ttlPast
	return s
}

func (s *Service) ttlFor(q Query) time.Duration {
	today := dateOnly(s.now().In(store.VN))
	if q.M.To.Before(today) {
		return s.ttlPast
	}
	return s.ttl
}

// cached: khoá = tên báo cáo + phạm vi đã áp quyền + bộ lọc + cờ admin (quyết định che SĐT) + tham số riêng.
func cached[T any](s *Service, ctx context.Context, name string, q Query, extra any, fn func(context.Context) (T, error)) (T, error) {
	s.pii(ctx) // nạp bảng PII trước khi map DTO (che long_url)
	key := cache.Key(name, q.M, q.Comp, q.Prev, q.Gran, q.admin, extra)
	return cache.Do(ctx, s.cache, name, key, s.ttlFor(q), fn)
}

// Params: bộ lọc chung từ query.
type Params struct {
	From, To    time.Time
	Accounts    []string
	Campaigns   []string
	CTVs        []string
	Prefixes    []string
	Compare     bool
	Granularity store.Granularity
}

// Query: bộ lọc đã qua scope + người xem.
type Query struct {
	P     domain.Principal
	M     store.Match
	Prev  store.Match // kỳ trước (khi Compare)
	Comp  bool
	Gran  store.Granularity
	admin bool
}

// Build kiểm khoảng ngày, áp phạm vi và chuẩn hoá bộ lọc.
func (s *Service) Build(p domain.Principal, prm Params) (Query, error) {
	from, to := dateOnly(prm.From), dateOnly(prm.To)
	if to.Before(from) {
		return Query{}, domain.BadRequest("invalid_range", "ngày kết thúc phải sau ngày bắt đầu")
	}
	if days(from, to) > MaxRangeDays {
		return Query{}, domain.BadRequest("range_too_long", "khoảng ngày tối đa 12 tháng — hãy thu hẹp")
	}
	owners, all, err := scope.For(p).Resolve(prm.Accounts)
	if err != nil {
		return Query{}, err
	}
	slices.Sort(owners) // khoá cache ổn định theo thứ tự
	m := store.Match{
		From: from, To: to, Owners: owners, AllOwners: all,
		Campaigns: normCampaigns(prm.Campaigns),
		CTVs:      s.normCTVs(prm.CTVs),
		Prefixes:  dedupe(prm.Prefixes),
	}
	g := prm.Granularity
	if g == "" {
		g = store.Day
	}
	q := Query{P: p, M: m, Comp: prm.Compare, Gran: g, admin: p.IsAdmin()}
	if prm.Compare {
		q.Prev = previous(m)
	}
	return q, nil
}

// ScopeOnly: chỉ phạm vi tài khoản (facets — không theo kỳ).
func (s *Service) ScopeOnly(p domain.Principal, accounts []string) (Query, error) {
	owners, all, err := scope.For(p).Resolve(accounts)
	if err != nil {
		return Query{}, err
	}
	slices.Sort(owners)
	return Query{P: p, M: store.Match{Owners: owners, AllOwners: all}, admin: p.IsAdmin()}, nil
}

// previous: kỳ liền trước, cùng số ngày.
func previous(m store.Match) store.Match {
	n := days(m.From, m.To)
	pm := m
	pm.To = m.From.AddDate(0, 0, -1)
	pm.From = pm.To.AddDate(0, 0, -(n - 1))
	return pm
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func days(from, to time.Time) int { return int(to.Sub(from).Hours()/24) + 1 }

// "-" = không gắn chiến dịch.
func normCampaigns(in []string) []string {
	out := make([]string, 0, len(in))
	for _, c := range dedupe(in) {
		if c == "-" {
			c = ""
		}
		out = append(out, c)
	}
	return out
}

// CTV nhập dạng nào cũng được: ctv_ref (từ URL), SĐT mọi định dạng, hash; "-" = chưa định danh.
func (s *Service) normCTVs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range dedupe(in) {
		switch {
		case v == "-":
			out = append(out, "")
		default:
			if id, err := s.masker.ParseCTVRef(v); err == nil {
				out = append(out, id)
			} else {
				out = append(out, mask.NormalizePhone(v))
			}
		}
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// CTVDisplay: SĐT che với mọi vai trò trừ admin; hash giữ nguyên.
func (q Query) CTVDisplay(id string) string {
	if id == "" {
		return "(chưa định danh)"
	}
	if q.admin {
		return id
	}
	return mask.Phone(id)
}

func (s *Service) ctvRef(id string) string { return s.masker.CTVRef(id) }

// group chạy các truy vấn song song, huỷ tất cả khi một cái lỗi.
func group(ctx context.Context) (*errgroup.Group, context.Context) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	return g, ctx
}
