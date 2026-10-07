package store

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// VN: múi giờ nghiệp vụ. Trường `date` của stats_* là 00:00Z của ngày lịch VN (date-only).
var VN = loadVN()

func loadVN() *time.Location {
	if l, err := time.LoadLocation("Asia/Ho_Chi_Minh"); err == nil {
		return l
	}
	return time.FixedZone("ICT", 7*3600)
}

// Level: mức tổng hợp của stats_*_daily. Truy vấn chọn mức THÔ NHẤT đủ chiều cho bộ lọc (ít document nhất).
type Level int

const (
	LevelSystem   Level = iota // stats_system_daily   (date)
	LevelOwner                 // stats_owner_daily    (owner, date)
	LevelCampaign              // stats_campaign_daily (owner, campaign_code, date)
	LevelCTV                   // stats_ctv_daily      (ctv_id, owner, campaign_code, date) — không có by_*
	LevelLink                  // stats_link_daily     (link_id, date) + owner, campaign_code, ctv_id, prefix — không có new_links
)

type Dim uint8

const (
	DimOwner Dim = 1 << iota
	DimCampaign
	DimCTV
	DimPrefix
	DimLink
)

func (l Level) Collection() string {
	return [...]string{CollStatsSystemDaily, CollStatsOwnerDaily, CollStatsCampaignDaily, CollStatsCTVDaily, CollStatsLinkDaily}[l]
}

func (l Level) Dims() Dim {
	return [...]Dim{0, DimOwner, DimOwner | DimCampaign, DimOwner | DimCampaign | DimCTV, DimOwner | DimCampaign | DimCTV | DimPrefix | DimLink}[l]
}

func (l Level) HasBreakdowns() bool { return l != LevelCTV }
func (l Level) HasNewLinks() bool   { return l != LevelLink }

// Match: điều kiện lọc chung (đã qua scope). Chuỗi rỗng trong Campaigns / CTVs = "không gắn" / "chưa định danh".
type Match struct {
	From, To  time.Time // date-only, gồm cả 2 đầu
	Owners    []string
	AllOwners bool // admin không lọc tài khoản → không cần điều kiện owner
	Campaigns []string
	CTVs      []string
	Prefixes  []string
	LinkIDs   []int64
}

// Needs: các chiều bộ lọc đòi hỏi.
func (m Match) Needs() Dim {
	var d Dim
	if !m.AllOwners {
		d |= DimOwner
	}
	if len(m.Campaigns) > 0 {
		d |= DimCampaign
	}
	if len(m.CTVs) > 0 {
		d |= DimCTV
	}
	if len(m.Prefixes) > 0 {
		d |= DimPrefix
	}
	if len(m.LinkIDs) > 0 {
		d |= DimLink
	}
	return d
}

// ChooseLevel: mức tổng hợp thô nhất có đủ chiều (bộ lọc + group) và (nếu cần) có phân rã by_*.
func ChooseLevel(m Match, group Dim, breakdowns bool) Level {
	need := m.Needs() | group
	for _, l := range []Level{LevelSystem, LevelOwner, LevelCampaign, LevelCTV, LevelLink} {
		if l.Dims()&need == need && (!breakdowns || l.HasBreakdowns()) {
			return l
		}
	}
	return LevelLink
}

// ---- dựng điều kiện ----

func in[T any](field string, vals []T) bson.E {
	if len(vals) == 1 {
		return bson.E{Key: field, Value: vals[0]}
	}
	return bson.E{Key: field, Value: bson.D{{Key: "$in", Value: vals}}}
}

func ownersCond(field string, m Match) bson.D {
	if m.AllOwners {
		return bson.D{}
	}
	owners := m.Owners
	if owners == nil {
		owners = []string{}
	}
	return bson.D{in(field, owners)}
}

// statsMatch: điều kiện trên stats_* (trường owner, campaign_code, ctv_id, prefix, link_id, date).
func statsMatch(m Match) bson.D {
	d := ownersCond("owner", m)
	if len(m.Campaigns) > 0 {
		d = append(d, in("campaign_code", m.Campaigns))
	}
	if len(m.CTVs) > 0 {
		d = append(d, in("ctv_id", m.CTVs))
	}
	if len(m.Prefixes) > 0 {
		d = append(d, in("prefix", m.Prefixes))
	}
	if len(m.LinkIDs) > 0 {
		d = append(d, in("link_id", m.LinkIDs))
	}
	d = append(d, bson.E{Key: "date", Value: bson.D{{Key: "$gte", Value: m.From}, {Key: "$lte", Value: m.To}}})
	return d
}

// linksMatch: điều kiện trên links (không gồm thời gian).
func linksMatch(m Match) bson.D {
	d := ownersCond("owner_username", m)
	if len(m.Campaigns) > 0 {
		d = append(d, in("campaign_code", m.Campaigns))
	}
	if len(m.CTVs) > 0 {
		d = append(d, in("ctv_id", m.CTVs))
	}
	if len(m.Prefixes) > 0 {
		d = append(d, in("prefix", m.Prefixes))
	}
	if len(m.LinkIDs) > 0 {
		d = append(d, in("_id", m.LinkIDs))
	}
	return d
}

// DayStart: 00:00 giờ VN của ngày lịch d (date-only) dưới dạng thời điểm.
func DayStart(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, VN)
}

// ---- kết quả ----

type Sums struct {
	Clicks, Unique, Bot, Susp, NewLinks int64
}

func (a *Sums) Add(b Sums) {
	a.Clicks += b.Clicks
	a.Unique += b.Unique
	a.Bot += b.Bot
	a.Susp += b.Susp
	a.NewLinks += b.NewLinks
}

type sumDoc struct {
	ID       any   `bson:"_id"`
	Clicks   int64 `bson:"clicks"`
	Unique   int64 `bson:"unique_clicks"`
	Bot      int64 `bson:"bot_clicks"`
	Susp     int64 `bson:"suspicious_clicks"`
	NewLinks int64 `bson:"new_links"`
}

func (d sumDoc) sums() Sums {
	return Sums{Clicks: d.Clicks, Unique: d.Unique, Bot: d.Bot, Susp: d.Susp, NewLinks: d.NewLinks}
}

func sumFields(l Level) bson.D {
	g := bson.D{
		{Key: "clicks", Value: bson.D{{Key: "$sum", Value: "$clicks"}}},
		{Key: "unique_clicks", Value: bson.D{{Key: "$sum", Value: "$unique_clicks"}}},
		{Key: "bot_clicks", Value: bson.D{{Key: "$sum", Value: "$bot_clicks"}}},
		{Key: "suspicious_clicks", Value: bson.D{{Key: "$sum", Value: "$suspicious_clicks"}}},
	}
	if l.HasNewLinks() {
		g = append(g, bson.E{Key: "new_links", Value: bson.D{{Key: "$sum", Value: "$new_links"}}})
	}
	return g
}

// SumStats: tổng clicks / unique / bot / suspicious (+ new_links nếu mức có) trong kỳ.
func (s *Store) SumStats(ctx context.Context, l Level, m Match) (Sums, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	group := append(bson.D{{Key: "_id", Value: nil}}, sumFields(l)...)
	cur, err := s.report(l.Collection()).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: statsMatch(m)}},
		{{Key: "$group", Value: group}},
	})
	if err != nil {
		return Sums{}, err
	}
	var out []sumDoc
	if err := cur.All(ctx, &out); err != nil || len(out) == 0 {
		return Sums{}, err
	}
	return out[0].sums(), nil
}

// ActiveLinks: số link có ≥ 1 click (không bot) trong kỳ — đếm phân biệt trên stats_link_daily.
func (s *Store) ActiveLinks(ctx context.Context, m Match) (int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(statsMatch(m), bson.E{Key: "clicks", Value: bson.D{{Key: "$gt", Value: 0}}})
	cur, err := s.report(CollStatsLinkDaily).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$link_id"}}}},
		{{Key: "$count", Value: "n"}},
	})
	if err != nil {
		return 0, err
	}
	var out []struct {
		N int64 `bson:"n"`
	}
	if err := cur.All(ctx, &out); err != nil || len(out) == 0 {
		return 0, err
	}
	return out[0].N, nil
}

// LinkCounts: total_links (chưa xoá, tạo đến hết kỳ) và new_links (tạo trong kỳ) đọc từ links.
func (s *Store) LinkCounts(ctx context.Context, m Match) (total, created int64, err error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	end := DayStart(m.To).AddDate(0, 0, 1)
	base := linksMatch(m)
	total, err = s.core(CollLinks).CountDocuments(ctx, append(base,
		bson.E{Key: "status", Value: bson.D{{Key: "$ne", Value: "deleted"}}},
		bson.E{Key: "created_at", Value: bson.D{{Key: "$lt", Value: end}}},
	))
	if err != nil {
		return 0, 0, err
	}
	created, err = s.core(CollLinks).CountDocuments(ctx, append(linksMatch(m),
		bson.E{Key: "created_at", Value: bson.D{{Key: "$gte", Value: DayStart(m.From)}, {Key: "$lt", Value: end}}},
	))
	return total, created, err
}

// ---- chuỗi thời gian ----

type Granularity string

const (
	Day   Granularity = "day"
	Week  Granularity = "week"
	Month Granularity = "month"
)

// bucketExpr: ngày đầu bucket (trên trường date-only). shift > 0: dịch ngày trước khi gom —
// dùng cho chuỗi kỳ trước để điểm i rơi đúng bucket i của kỳ hiện tại.
func bucketExpr(field string, g Granularity, shift int) any {
	var date any = field
	if shift != 0 {
		date = bson.D{{Key: "$dateAdd", Value: bson.D{{Key: "startDate", Value: field}, {Key: "unit", Value: "day"}, {Key: "amount", Value: shift}}}}
	}
	field2 := date
	switch g {
	case Week:
		return bson.D{{Key: "$dateTrunc", Value: bson.D{{Key: "date", Value: field2}, {Key: "unit", Value: "week"}, {Key: "startOfWeek", Value: "monday"}}}}
	case Month:
		return bson.D{{Key: "$dateTrunc", Value: bson.D{{Key: "date", Value: field2}, {Key: "unit", Value: "month"}}}}
	default:
		return field2
	}
}

func dateKey(expr any) bson.D {
	return bson.D{{Key: "$dateToString", Value: bson.D{{Key: "format", Value: "%Y-%m-%d"}, {Key: "date", Value: expr}}}}
}

// SeriesStats: tổng theo bucket ("2006-01-02" → Sums).
func (s *Store) SeriesStats(ctx context.Context, l Level, m Match, g Granularity, shift int) (map[string]Sums, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	group := append(bson.D{{Key: "_id", Value: dateKey(bucketExpr("$date", g, shift))}}, sumFields(l)...)
	cur, err := s.report(l.Collection()).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: statsMatch(m)}},
		{{Key: "$group", Value: group}},
	})
	if err != nil {
		return nil, err
	}
	var docs []sumDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make(map[string]Sums, len(docs))
	for _, d := range docs {
		out[d.ID.(string)] = d.sums()
	}
	return out, nil
}

// ActiveSeries: số link có click (phân biệt) theo bucket.
func (s *Store) ActiveSeries(ctx context.Context, m Match, g Granularity, shift int) (map[string]int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(statsMatch(m), bson.E{Key: "clicks", Value: bson.D{{Key: "$gt", Value: 0}}})
	cur, err := s.report(CollStatsLinkDaily).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: bson.D{{Key: "b", Value: dateKey(bucketExpr("$date", g, shift))}, {Key: "l", Value: "$link_id"}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$_id.b"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	})
	if err != nil {
		return nil, err
	}
	return decodeCountMap(ctx, cur)
}

// NewLinkSeries: link tạo theo bucket (giờ VN) — dùng khi mức tổng hợp không có new_links.
func (s *Store) NewLinkSeries(ctx context.Context, m Match, g Granularity, shift int) (map[string]int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	unit := map[Granularity]string{Day: "day", Week: "week", Month: "month"}[g]
	var created any = "$created_at"
	if shift != 0 {
		created = bson.D{{Key: "$dateAdd", Value: bson.D{{Key: "startDate", Value: "$created_at"}, {Key: "unit", Value: "day"}, {Key: "amount", Value: shift}, {Key: "timezone", Value: VN.String()}}}}
	}
	trunc := bson.D{{Key: "date", Value: created}, {Key: "unit", Value: unit}, {Key: "timezone", Value: VN.String()}}
	if g == Week {
		trunc = append(trunc, bson.E{Key: "startOfWeek", Value: "monday"})
	}
	key := bson.D{{Key: "$dateToString", Value: bson.D{
		{Key: "format", Value: "%Y-%m-%d"}, {Key: "timezone", Value: VN.String()},
		{Key: "date", Value: bson.D{{Key: "$dateTrunc", Value: trunc}}},
	}}}
	match := append(linksMatch(m), bson.E{Key: "created_at", Value: bson.D{
		{Key: "$gte", Value: DayStart(m.From)}, {Key: "$lt", Value: DayStart(m.To).AddDate(0, 0, 1)},
	}})
	cur, err := s.core(CollLinks).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: key}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	})
	if err != nil {
		return nil, err
	}
	return decodeCountMap(ctx, cur)
}

type countDoc struct {
	ID string `bson:"_id"`
	N  int64  `bson:"n"`
}

func decodeCountMap(ctx context.Context, cur interface {
	All(context.Context, any) error
}) (map[string]int64, error) {
	var docs []countDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(docs))
	for _, d := range docs {
		out[d.ID] = d.N
	}
	return out, nil
}

// ---- phân rã & heatmap ----

// BreakdownFields: trường by_* → tên chiều trả về.
var BreakdownFields = map[string]string{
	"device": "by_device", "os": "by_os", "browser": "by_browser", "source_group": "by_source_group",
	"referer": "by_referer", "country": "by_country", "access_prefix": "by_prefix",
}

// Breakdowns: chiều → giá trị → clicks (gộp các map by_* trong kỳ).
func (s *Store) Breakdowns(ctx context.Context, l Level, m Match) (map[string]map[string]int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	arr := bson.A{}
	for dim, field := range BreakdownFields {
		arr = append(arr, bson.D{{Key: "f", Value: dim}, {Key: "kv", Value: bson.D{{Key: "$objectToArray", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$" + field, bson.D{}}}}}}}})
	}
	cur, err := s.report(l.Collection()).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: statsMatch(m)}},
		{{Key: "$project", Value: bson.D{{Key: "_id", Value: 0}, {Key: "a", Value: arr}}}},
		{{Key: "$unwind", Value: "$a"}},
		{{Key: "$unwind", Value: "$a.kv"}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "f", Value: "$a.f"}, {Key: "k", Value: "$a.kv.k"}}},
			{Key: "n", Value: bson.D{{Key: "$sum", Value: "$a.kv.v"}}},
		}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID struct{ F, K string } `bson:"_id"`
		N  int64                 `bson:"n"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := map[string]map[string]int64{}
	for dim := range BreakdownFields {
		out[dim] = map[string]int64{}
	}
	for _, d := range docs {
		out[d.ID.F][DecodeKey(d.ID.K)] += d.N
	}
	return out, nil
}

// DecodeKey: khoá map by_* thay "." bằng "．" (U+FF0E) — hợp đồng schema với Service.
func DecodeKey(k string) string { return strings.ReplaceAll(k, "．", ".") }

// Heatmap: [thứ 0=CN..6][giờ 0..23] → clicks, từ by_hour[24] của từng ngày.
func (s *Store) Heatmap(ctx context.Context, l Level, m Match) ([7][24]int64, error) {
	var out [7][24]int64
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(l.Collection()).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: statsMatch(m)}},
		{{Key: "$project", Value: bson.D{{Key: "dow", Value: bson.D{{Key: "$dayOfWeek", Value: "$date"}}}, {Key: "h", Value: "$by_hour"}}}},
		{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$h"}, {Key: "includeArrayIndex", Value: "hour"}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "d", Value: "$dow"}, {Key: "h", Value: "$hour"}}},
			{Key: "n", Value: bson.D{{Key: "$sum", Value: "$h"}}},
		}}},
	})
	if err != nil {
		return out, err
	}
	var docs []struct {
		ID struct {
			D int   `bson:"d"`
			H int64 `bson:"h"`
		} `bson:"_id"`
		N int64 `bson:"n"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return out, err
	}
	for _, d := range docs {
		if d.ID.D >= 1 && d.ID.D <= 7 && d.ID.H >= 0 && d.ID.H < 24 {
			out[d.ID.D-1][d.ID.H] += d.N
		}
	}
	return out, nil
}

// ---- nhóm theo chiều (bảng / top) ----

// GroupField: trường nhóm trên stats_* theo chiều.
func GroupField(d Dim) string {
	switch d {
	case DimOwner:
		return "owner"
	case DimCampaign:
		return "campaign_code"
	case DimCTV:
		return "ctv_id"
	case DimPrefix:
		return "prefix"
	default:
		return "link_id"
	}
}

type GroupRow struct {
	Key       string
	LinkID    int64
	Sums      Sums
	FirstDate time.Time // ngày đầu có hoạt động (click hoặc link mới)
	LastClick time.Time // ngày cuối có click
	AnyOwner  string    // một owner của nhóm (để hiện link về tài khoản)
}

type GroupOpts struct {
	SortBy string // clicks | unique_clicks | suspicious_clicks | new_links (rỗng = không sort)
	Limit  int64  // 0 = tất cả
}

// GroupBy: tổng theo chiều d trên mức l (l phải có chiều d).
func (s *Store) GroupBy(ctx context.Context, l Level, m Match, d Dim, o GroupOpts) ([]GroupRow, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	field := GroupField(d)
	activity := bson.D{{Key: "$gt", Value: bson.A{"$clicks", 0}}}
	if l.HasNewLinks() {
		activity = bson.D{{Key: "$or", Value: bson.A{bson.D{{Key: "$gt", Value: bson.A{"$clicks", 0}}}, bson.D{{Key: "$gt", Value: bson.A{"$new_links", 0}}}}}}
	}
	group := append(bson.D{{Key: "_id", Value: "$" + field}}, sumFields(l)...)
	group = append(group,
		bson.E{Key: "first", Value: bson.D{{Key: "$min", Value: bson.D{{Key: "$cond", Value: bson.A{activity, "$date", nil}}}}}},
		bson.E{Key: "last", Value: bson.D{{Key: "$max", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$gt", Value: bson.A{"$clicks", 0}}}, "$date", nil}}}}}},
		bson.E{Key: "owner", Value: bson.D{{Key: "$first", Value: "$owner"}}},
	)
	pipe := []bson.D{
		{{Key: "$match", Value: statsMatch(m)}},
		{{Key: "$group", Value: group}},
	}
	if o.SortBy != "" {
		pipe = append(pipe, bson.D{{Key: "$sort", Value: bson.D{{Key: o.SortBy, Value: -1}, {Key: "_id", Value: 1}}}})
	}
	if o.Limit > 0 {
		pipe = append(pipe, bson.D{{Key: "$limit", Value: o.Limit}})
	}
	cur, err := s.report(l.Collection()).Aggregate(ctx, pipe, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	// Trường tường minh: struct nhúng không export (sumDoc) bị driver bỏ qua khi decode.
	var docs []struct {
		ID       any        `bson:"_id"`
		Clicks   int64      `bson:"clicks"`
		Unique   int64      `bson:"unique_clicks"`
		Bot      int64      `bson:"bot_clicks"`
		Susp     int64      `bson:"suspicious_clicks"`
		NewLinks int64      `bson:"new_links"`
		First    *time.Time `bson:"first"`
		Last     *time.Time `bson:"last"`
		Owner    string     `bson:"owner"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]GroupRow, len(docs))
	for i, doc := range docs {
		r := GroupRow{Sums: Sums{Clicks: doc.Clicks, Unique: doc.Unique, Bot: doc.Bot, Susp: doc.Susp, NewLinks: doc.NewLinks}, AnyOwner: doc.Owner}
		switch v := doc.ID.(type) {
		case string:
			r.Key = v
		case int64:
			r.LinkID = v
		case int32:
			r.LinkID = int64(v)
		}
		if doc.First != nil {
			r.FirstDate = *doc.First
		}
		if doc.Last != nil {
			r.LastClick = *doc.Last
		}
		out[i] = r
	}
	return out, nil
}

// ActiveBy: số link có click theo chiều d (đọc stats_link_daily).
func (s *Store) ActiveBy(ctx context.Context, m Match, d Dim) (map[string]int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(statsMatch(m), bson.E{Key: "clicks", Value: bson.D{{Key: "$gt", Value: 0}}})
	cur, err := s.report(CollStatsLinkDaily).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: bson.D{{Key: "g", Value: "$" + GroupField(d)}, {Key: "l", Value: "$link_id"}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$_id.g"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	return decodeCountMap(ctx, cur)
}

// TotalLinksBy: total_links (chưa xoá, tạo đến hết kỳ) và new_links theo chiều owner / campaign / ctv.
func (s *Store) TotalLinksBy(ctx context.Context, m Match, d Dim) (total, created map[string]int64, err error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	field := map[Dim]string{DimOwner: "$owner_username", DimCampaign: "$campaign_code", DimCTV: "$ctv_id", DimPrefix: "$prefix"}[d]
	start, end := DayStart(m.From), DayStart(m.To).AddDate(0, 0, 1)
	match := append(linksMatch(m), bson.E{Key: "created_at", Value: bson.D{{Key: "$lt", Value: end}}})
	cur, err := s.core(CollLinks).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: field},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$ne", Value: bson.A{"$status", "deleted"}}}, 1, 0}}}}}},
			{Key: "created", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{bson.D{{Key: "$gte", Value: bson.A{"$created_at", start}}}, 1, 0}}}}}},
		}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, nil, err
	}
	var docs []struct {
		ID      string `bson:"_id"`
		Total   int64  `bson:"total"`
		Created int64  `bson:"created"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, nil, err
	}
	total, created = map[string]int64{}, map[string]int64{}
	for _, doc := range docs {
		total[doc.ID] = doc.Total
		created[doc.ID] = doc.Created
	}
	return total, created, nil
}

// DistinctCountBy: số giá trị phân biệt (khác rỗng) của trường count theo trường group, chỉ dòng có hoạt động.
// Vd số CTV theo chiến dịch: DistinctCountBy(LevelCTV, m, "campaign_code", "ctv_id").
func (s *Store) DistinctCountBy(ctx context.Context, l Level, m Match, groupField, countField string) (map[string]int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	act := bson.A{bson.D{{Key: "clicks", Value: bson.D{{Key: "$gt", Value: 0}}}}}
	if l.HasNewLinks() {
		act = append(act, bson.D{{Key: "new_links", Value: bson.D{{Key: "$gt", Value: 0}}}})
	}
	match := append(statsMatch(m),
		bson.E{Key: "$or", Value: act},
		bson.E{Key: countField, Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}},
	)
	cur, err := s.report(l.Collection()).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: bson.D{{Key: "g", Value: "$" + groupField}, {Key: "c", Value: "$" + countField}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$_id.g"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	return decodeCountMap(ctx, cur)
}
