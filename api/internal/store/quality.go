package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type IPStat struct {
	IP       string    `bson:"_id"`
	Clicks   int64     `bson:"clicks"`
	Bot      int64     `bson:"bot"`
	Susp     int64     `bson:"susp"`
	Links    int64     `bson:"links"`
	Accounts int64     `bson:"accounts"`
	Country  string    `bson:"country"`
	LastSeen time.Time `bson:"last"`
}

// TopIPs: IP nhiều lượt truy cập nhất (gồm bot) trong kỳ.
func (s *Store) TopIPs(ctx context.Context, q ClickQuery, limit int64) ([]IPStat, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cnt := func(cond any) bson.D {
		return bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{cond, 1, 0}}}}}
	}
	cur, err := s.core(CollClicks).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: clickMatch(q)}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$ip"},
			{Key: "clicks", Value: cnt(bson.D{{Key: "$eq", Value: bson.A{"$is_bot", false}}})},
			{Key: "bot", Value: cnt(bson.D{{Key: "$eq", Value: bson.A{"$is_bot", true}}})},
			{Key: "susp", Value: cnt(bson.D{{Key: "$eq", Value: bson.A{"$is_suspicious", true}}})},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "l", Value: bson.D{{Key: "$addToSet", Value: "$meta.link_id"}}},
			{Key: "a", Value: bson.D{{Key: "$addToSet", Value: "$meta.owner"}}},
			{Key: "country", Value: bson.D{{Key: "$first", Value: "$country"}}},
			{Key: "last", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "total", Value: -1}, {Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: limit}},
		{{Key: "$project", Value: bson.D{
			{Key: "clicks", Value: 1}, {Key: "bot", Value: 1}, {Key: "susp", Value: 1}, {Key: "country", Value: 1}, {Key: "last", Value: 1},
			{Key: "links", Value: bson.D{{Key: "$size", Value: "$l"}}}, {Key: "accounts", Value: bson.D{{Key: "$size", Value: "$a"}}},
		}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	out := []IPStat{}
	return out, cur.All(ctx, &out)
}

type SourceBot struct {
	Source string `bson:"_id"`
	Total  int64  `bson:"total"`
	Bot    int64  `bson:"bot"`
}

// BotBySource: tỉ lệ bot theo nhóm nguồn.
func (s *Store) BotBySource(ctx context.Context, q ClickQuery) ([]SourceBot, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.core(CollClicks).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: clickMatch(q)}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$source_group", ""}}}},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "bot", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{"$is_bot", 1, 0}}}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "total", Value: -1}}}},
	})
	if err != nil {
		return nil, err
	}
	out := []SourceBot{}
	return out, cur.All(ctx, &out)
}

// FlaggedClicks: click bot / nghi vấn mới nhất.
func (s *Store) FlaggedClicks(ctx context.Context, q ClickQuery, limit int64) ([]ClickDoc, []string, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := append(clickMatch(q), bson.E{Key: "$or", Value: bson.A{
		bson.D{{Key: "is_bot", Value: true}}, bson.D{{Key: "is_suspicious", Value: true}},
	}})
	cur, err := s.core(CollClicks).Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "ts", Value: -1}}).SetLimit(limit).
		SetProjection(bson.D{{Key: "event_id", Value: 0}, {Key: "referer", Value: 0}}))
	if err != nil {
		return nil, nil, err
	}
	var docs []struct {
		ClickDoc  `bson:",inline"`
		UserAgent string `bson:"user_agent"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, nil, err
	}
	out := make([]ClickDoc, len(docs))
	uas := make([]string, len(docs))
	for i, d := range docs {
		out[i], uas[i] = d.ClickDoc, d.UserAgent
	}
	return out, uas, nil
}

// ---- danh mục tham số (§4.1 c3) ----

type ParamSeen struct {
	Key     string   `bson:"_id"`
	Links   int64    `bson:"links"`
	Values  int64    `bson:"values"`
	Samples []string `bson:"samples"`
}

// ParamsSeen: tham số đã xuất hiện trên link (mọi tài khoản) + số link, số giá trị (tối đa 10.000), ví dụ.
func (s *Store) ParamsSeen(ctx context.Context) ([]ParamSeen, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollLinkParams).Aggregate(ctx, []bson.D{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$key"},
			{Key: "links", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "vals", Value: bson.D{{Key: "$addToSet", Value: "$value"}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "links", Value: 1},
			{Key: "values", Value: bson.D{{Key: "$min", Value: bson.A{bson.D{{Key: "$size", Value: "$vals"}}, 10000}}}},
			{Key: "samples", Value: bson.D{{Key: "$slice", Value: bson.A{"$vals", 3}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "links", Value: -1}}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	out := []ParamSeen{}
	return out, cur.All(ctx, &out)
}

// UpsertRegistry: admin ghi param_registry (Portal được ghi collection này — §3.1).
// startBackfill: chuyển sang tracked → đặt backfill pending để job của Service chạy.
func (s *Store) UpsertRegistry(ctx context.Context, e RegistryEntry, startBackfill bool, now time.Time) (*RegistryEntry, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	set := bson.D{
		{Key: "key", Value: e.Key}, {Key: "label", Value: e.Label}, {Key: "tracked", Value: e.Tracked},
		{Key: "pii", Value: e.PII}, {Key: "max_values", Value: e.MaxValues}, {Key: "status", Value: e.Status},
		{Key: "updated_by", Value: e.UpdatedBy}, {Key: "updated_at", Value: now},
	}
	if startBackfill {
		set = append(set, bson.E{Key: "backfill", Value: bson.D{{Key: "status", Value: "pending"}, {Key: "progress", Value: 0.0}, {Key: "requested_at", Value: now}}})
	}
	upd := bson.D{
		{Key: "$set", Value: set},
		{Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: e.Key}, {Key: "created_by", Value: e.UpdatedBy}, {Key: "created_at", Value: now}, {Key: "normalize", Value: bson.A{"trim"}}}},
	}
	var out RegistryEntry
	err := s.report(CollParamRegistry).FindOneAndUpdate(ctx, bson.D{{Key: "key", Value: e.Key}}, upd,
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&out)
	return &out, err
}
