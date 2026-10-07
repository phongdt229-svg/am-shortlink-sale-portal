package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// LinkByCode: link theo mã (không phân biệt prefix). Không có → nil. Phạm vi do report kiểm.
func (s *Store) LinkByCode(ctx context.Context, code string) (*LinkInfo, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	var l LinkInfo
	err := s.core(CollLinks).FindOne(ctx, bson.D{{Key: "code", Value: code}}, options.FindOne().SetProjection(linkInfoProjection)).Decode(&l)
	if isNoDocs(err) {
		return nil, nil
	}
	return &l, err
}

// LinkIDsByCodes: mã → id, chỉ link thuộc phạm vi m.
func (s *Store) LinkIDsByCodes(ctx context.Context, m Match, codes []string) ([]int64, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := append(ownersCond("owner_username", m), bson.E{Key: "code", Value: bson.D{{Key: "$in", Value: codes}}})
	cur, err := s.core(CollLinks).Find(ctx, filter, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID int64 `bson:"_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	ids := make([]int64, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	return ids, nil
}

// CTVLinkGroup: tổng hợp link của 1 CTV theo tài khoản (tra cứu CTV).
type CTVLinkGroup struct {
	Owner       string     `bson:"_id"`
	Links       int64      `bson:"links"`
	ClicksTotal int64      `bson:"clicks"`
	First       time.Time  `bson:"first"`
	Last        time.Time  `bson:"last"`
	Sample      []LinkInfo `bson:"sample"`
}

// LinksByCTV: tra theo index (ctv_id, created_at) — thay LIKE '%...%' ~15s của hệ cũ.
func (s *Store) LinksByCTV(ctx context.Context, m Match, ctvID string) ([]CTVLinkGroup, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(bson.D{{Key: "ctv_id", Value: ctvID}}, ownersCond("owner_username", m)...)
	cur, err := s.core(CollLinks).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$sort", Value: bson.D{{Key: "clicks", Value: -1}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$owner_username"},
			{Key: "links", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "clicks", Value: bson.D{{Key: "$sum", Value: "$clicks"}}},
			{Key: "first", Value: bson.D{{Key: "$min", Value: "$created_at"}}},
			{Key: "last", Value: bson.D{{Key: "$max", Value: "$created_at"}}},
			{Key: "sample", Value: bson.D{{Key: "$push", Value: bson.D{
				{Key: "_id", Value: "$_id"}, {Key: "code", Value: "$code"}, {Key: "long_url", Value: "$long_url"}, {Key: "dest_host", Value: "$dest_host"},
				{Key: "owner_username", Value: "$owner_username"}, {Key: "campaign_code", Value: "$campaign_code"}, {Key: "ctv_id", Value: "$ctv_id"},
				{Key: "status", Value: "$status"}, {Key: "prefix", Value: "$prefix"}, {Key: "is_custom", Value: "$is_custom"},
				{Key: "api_version", Value: "$api_version"}, {Key: "created_at", Value: "$created_at"},
			}}}},
		}}},
		{{Key: "$project", Value: bson.D{{Key: "links", Value: 1}, {Key: "clicks", Value: 1}, {Key: "first", Value: 1}, {Key: "last", Value: 1}, {Key: "sample", Value: bson.D{{Key: "$slice", Value: bson.A{"$sample", 10}}}}}}},
		{{Key: "$sort", Value: bson.D{{Key: "clicks", Value: -1}}}},
	})
	if err != nil {
		return nil, err
	}
	out := []CTVLinkGroup{}
	return out, cur.All(ctx, &out)
}

// FindLinks: danh sách link theo điều kiện (phân trang, mới nhất trước) + tổng số.
func (s *Store) FindLinks(ctx context.Context, m Match, extra bson.D, skip, limit int64) ([]LinkInfo, int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := append(linksMatch(m), extra...)
	total, err := s.core(CollLinks).CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cur, err := s.core(CollLinks).Find(ctx, filter, options.Find().
		SetProjection(linkInfoProjection).SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(skip).SetLimit(limit))
	if err != nil {
		return nil, 0, err
	}
	out := []LinkInfo{}
	return out, total, cur.All(ctx, &out)
}

// UnidentifiedFilter: link chưa định danh CTV.
func UnidentifiedFilter() bson.D {
	return bson.D{{Key: "ctv_id", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}}}
}

// ActiveLinkIDs: link có click (không bot) trong kỳ m.
func (s *Store) ActiveLinkIDs(ctx context.Context, m Match) ([]int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(statsMatch(m), bson.E{Key: "clicks", Value: bson.D{{Key: "$gt", Value: 0}}})
	cur, err := s.report(CollStatsLinkDaily).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$link_id"}}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID int64 `bson:"_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	ids := make([]int64, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	return ids, nil
}

// DeadLinks: link đang active, tạo trước cửa sổ, KHÔNG có click trong [windowFrom, m.To].
func (s *Store) DeadLinks(ctx context.Context, m Match, windowFrom time.Time, skip, limit int64) ([]LinkInfo, int64, error) {
	wm := m
	wm.From = windowFrom
	active, err := s.ActiveLinkIDs(ctx, wm)
	if err != nil {
		return nil, 0, err
	}
	if active == nil {
		active = []int64{}
	}
	extra := bson.D{
		{Key: "status", Value: "active"},
		{Key: "created_at", Value: bson.D{{Key: "$lt", Value: DayStart(windowFrom)}}},
		{Key: "_id", Value: bson.D{{Key: "$nin", Value: active}}},
	}
	mm := m
	mm.LinkIDs = nil
	return s.FindLinks(ctx, mm, extra, skip, limit)
}

// LastClickDates: ngày có click gần nhất (≤ to) của các link.
func (s *Store) LastClickDates(ctx context.Context, ids []int64, to time.Time) (map[int64]time.Time, error) {
	out := map[int64]time.Time{}
	if len(ids) == 0 {
		return out, nil
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollStatsLinkDaily).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: bson.D{
			{Key: "link_id", Value: bson.D{{Key: "$in", Value: ids}}},
			{Key: "date", Value: bson.D{{Key: "$lte", Value: to}}},
			{Key: "clicks", Value: bson.D{{Key: "$gt", Value: 0}}},
		}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$link_id"}, {Key: "d", Value: bson.D{{Key: "$max", Value: "$date"}}}}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID int64     `bson:"_id"`
		D  time.Time `bson:"d"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		out[d.ID] = d.D
	}
	return out, nil
}

// ParamLinkIDs: link có tham số URL thoả điều kiện (đọc link_params, trong phạm vi owner).
// op: eq | in | contains | exists. (not_exists = exists + loại trừ — xử lý ở report.)
func (s *Store) ParamLinkIDs(ctx context.Context, m Match, key, op string, values []string) ([]int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := append(ownersCond("owner", m), bson.E{Key: "key", Value: key})
	switch op {
	case "eq", "in":
		filter = append(filter, bson.E{Key: "value", Value: bson.D{{Key: "$in", Value: values}}})
	case "contains":
		or := bson.A{}
		for _, v := range values {
			or = append(or, bson.D{{Key: "value", Value: containsRegex(v)}})
		}
		filter = append(filter, bson.E{Key: "$or", Value: or})
	}
	ids := []int64{}
	cur, err := s.report(CollLinkParams).Find(ctx, filter, options.Find().SetProjection(bson.D{{Key: "link_id", Value: 1}, {Key: "_id", Value: 0}}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID int64 `bson:"link_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	for _, d := range docs {
		if !seen[d.ID] {
			seen[d.ID] = true
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}
