package report

import (
	"context"
	"encoding/base64"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/mask"
	"am-shortlink-portal/api/internal/store"
)

// MaxRawDays: click thô / lọc chi tiết tối đa 3 tháng.
const MaxRawDays = 92

func crit(c *gen.Criterion) store.In {
	if c == nil {
		return store.In{}
	}
	vals := dedupe(c.Values)
	return store.In{Values: vals, Exclude: c.Exclude != nil && *c.Exclude}
}

// ClickQuery chuyển ClickFilters (API) → store.ClickQuery: kiểm giới hạn 3 tháng, quyền lọc IP,
// đổi mã link / điều kiện tham số URL thành tập link_id (trong phạm vi).
func (s *Service) ClickQuery(ctx context.Context, q Query, f *gen.ClickFilters) (store.ClickQuery, error) {
	if days(q.M.From, q.M.To) > MaxRawDays {
		return store.ClickQuery{}, domain.BadRequest("range_too_long", "click thô / lọc chi tiết tối đa 3 tháng — hãy thu hẹp khoảng ngày")
	}
	cq := store.ClickQuery{M: q.M}
	if f == nil {
		return cq, nil
	}
	cq.AccessPrefix, cq.APIVersion, cq.DestHost = crit(f.AccessPrefix), crit(f.ApiVersion), crit(f.DestHost)
	cq.Device, cq.OS, cq.Browser, cq.SourceGroup = crit(f.Device), crit(f.Os), crit(f.Browser), crit(f.SourceGroup)
	cq.RefererHost, cq.Country, cq.Province = crit(f.RefererHost), crit(f.Country), crit(f.Province)
	if f.Weekdays != nil {
		cq.Weekdays = *f.Weekdays
	}
	if f.HourFrom != nil || f.HourTo != nil {
		from, to := deref(f.HourFrom, 0), deref(f.HourTo, 23)
		cq.HourFrom, cq.HourTo = &from, &to
	}
	cq.IsCustom = f.IsCustom
	if f.LinkCreatedFrom != nil {
		cq.LinkCreatedFrom = &f.LinkCreatedFrom.Time
	}
	if f.LinkCreatedTo != nil {
		cq.LinkCreatedTo = &f.LinkCreatedTo.Time
	}
	cq.Quality = string(deref(f.Quality, gen.ClickFiltersQualityAll))
	cq.Visit = string(deref(f.Visit, gen.ClickFiltersVisitAll))
	cq.CampaignPresence = string(deref(f.CampaignPresence, gen.ClickFiltersCampaignPresenceAny))
	cq.CTVPresence = string(deref(f.CtvPresence, gen.ClickFiltersCtvPresenceAny))

	if ip := crit(f.Ip); len(ip.Values) > 0 {
		if !q.admin {
			return cq, domain.Forbidden("admin_only", "chỉ admin được lọc theo IP")
		}
		if err := parseIPs(ip, &cq); err != nil {
			return cq, err
		}
	}
	if l := crit(f.Links); len(l.Values) > 0 {
		ids, err := s.st.LinkIDsByCodes(ctx, q.M, l.Values)
		if err != nil {
			return cq, err
		}
		cq.LinkSets = append(cq.LinkSets, store.IDSet{IDs: ids, Exclude: l.Exclude})
	}
	if f.Params != nil {
		for _, pc := range *f.Params {
			op := string(pc.Op)
			vals := deref(pc.Values, nil)
			if op != "exists" && op != "not_exists" && len(vals) == 0 {
				return cq, domain.BadRequest("invalid_param_condition", "điều kiện tham số "+pc.Key+" thiếu giá trị")
			}
			lookup := op
			if op == "not_exists" {
				lookup = "exists"
			}
			ids, err := s.st.ParamLinkIDs(ctx, q.M, pc.Key, lookup, s.normParamValues(ctx, pc.Key, vals))
			if err != nil {
				return cq, err
			}
			exclude := (op == "not_exists") != (pc.Exclude != nil && *pc.Exclude)
			cq.LinkSets = append(cq.LinkSets, store.IDSet{IDs: ids, Exclude: exclude})
		}
	}
	return cq, nil
}

// normParamValues: giá trị lọc tham số theo cùng quy tắc chuẩn hoá lúc Service ghi link_params.
func (s *Service) normParamValues(ctx context.Context, key string, vals []string) []string {
	pii := s.piiHashed(ctx, key)
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		switch key {
		case "utm_extra_ctv":
			v = mask.NormalizePhone(v)
		case "utm_source", "utm_medium", "utm_campaign", "promo":
			v = strings.ToLower(v)
		}
		if pii {
			v = s.masker.HashPII(mask.NormalizePhone(v)) // pii = hash: tra cứu chính xác bằng băm
		}
		out = append(out, v)
	}
	return out
}

// parseIPs: IP đơn hoặc CIDR IPv4 theo octet (/8, /16, /24, /32).
func parseIPs(c store.In, cq *store.ClickQuery) error {
	exact := store.In{Exclude: c.Exclude}
	for _, v := range c.Values {
		if strings.Contains(v, "/") {
			p, err := netip.ParsePrefix(v)
			if err != nil || !p.Addr().Is4() || p.Bits()%8 != 0 {
				return domain.BadRequest("invalid_ip", "CIDR chỉ hỗ trợ IPv4 /8, /16, /24, /32: "+v)
			}
			if p.Bits() == 32 {
				exact.Values = append(exact.Values, p.Addr().String())
				continue
			}
			parts := strings.Split(p.Masked().Addr().String(), ".")
			cq.IPPrefixes = append(cq.IPPrefixes, strings.Join(parts[:p.Bits()/8], ".")+".")
			cq.IPPrefixExclude = c.Exclude
			continue
		}
		a, err := netip.ParseAddr(v)
		if err != nil {
			return domain.BadRequest("invalid_ip", "IP không hợp lệ: "+v)
		}
		exact.Values = append(exact.Values, a.String())
	}
	cq.IPExact = exact
	return nil
}

// ---- con trỏ phân trang: base64url("<unix nano>.<objectid hex>") ----

func encodeCursor(c store.ClickDoc) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(c.TS.UnixNano(), 10) + "." + c.ID.Hex()))
}

func decodeCursor(s string) (*store.ClickCursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	bad := domain.BadRequest("invalid_cursor", "con trỏ phân trang không hợp lệ")
	if err != nil {
		return nil, bad
	}
	ts, id, ok := strings.Cut(string(b), ".")
	if !ok {
		return nil, bad
	}
	n, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, bad
	}
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, bad
	}
	return &store.ClickCursor{TS: time.Unix(0, n).UTC(), ID: oid}, nil
}

// Clicks: R6 nhật ký click thô (không cache — dữ liệu theo con trỏ).
func (s *Service) Clicks(ctx context.Context, q Query, f *gen.ClickFilters, cursor string, size int) (gen.ClickPage, error) {
	cq, err := s.ClickQuery(ctx, q, f)
	if err != nil {
		return gen.ClickPage{}, err
	}
	s.pii(ctx)
	after, err := decodeCursor(cursor)
	if err != nil {
		return gen.ClickPage{}, err
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	docs, err := s.st.ListClicks(ctx, cq, after, int64(size+1))
	if err != nil {
		return gen.ClickPage{}, err
	}
	out := gen.ClickPage{}
	if len(docs) > size {
		docs = docs[:size]
		next := encodeCursor(docs[len(docs)-1])
		out.NextCursor = &next
	}
	ids := map[int64]bool{}
	for _, d := range docs {
		ids[d.Meta.LinkID] = true
	}
	list := make([]int64, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	links, err := s.st.LinksByIDs(ctx, q.M, list)
	if err != nil {
		return gen.ClickPage{}, err
	}
	out.Items = s.clickRows(q, docs, links)
	return out, nil
}

// clickRows: map click thô → DTO, che IP / SĐT theo vai trò.
func (s *Service) clickRows(q Query, docs []store.ClickDoc, links map[int64]store.LinkInfo) []gen.ClickRow {
	out := make([]gen.ClickRow, len(docs))
	for i, d := range docs {
		l := links[d.Meta.LinkID]
		ip := d.IP
		if !q.admin {
			ip = mask.IP(ip)
		}
		r := gen.ClickRow{
			Ts: d.TS, Code: l.Code, Owner: d.Meta.Owner, Device: d.Device, Os: d.OS, Browser: d.Browser,
			SourceGroup: d.SourceGroup, Country: d.Country, Ip: ip,
			IsBot: d.IsBot, IsSuspicious: d.IsSuspicious, IsRepeat: d.IsRepeat,
		}
		r.Prefix, r.AccessPrefix = strPtr(d.LinkPrefix), strPtr(d.AccessPrefix)
		r.LongUrl = strPtr(maskURL(l.LongURL, s.piiSnapshot(), q.admin))
		r.CampaignCode = strPtr(d.Meta.Campaign)
		r.CtvDisplay = strPtr(q.CTVDisplay(d.Meta.CTV))
		if d.Meta.CTV != "" {
			r.CtvRef = strPtr(s.ctvRef(d.Meta.CTV))
		}
		r.RefererHost, r.Province = strPtr(d.RefererHost), strPtr(d.Province)
		out[i] = r
	}
	return out
}

func strPtr(s string) *string { return &s }

func deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}
