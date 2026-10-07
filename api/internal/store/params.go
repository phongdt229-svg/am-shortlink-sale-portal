package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RegistryEntry: param_registry (hợp đồng §5.4b).
type RegistryEntry struct {
	Key       string    `bson:"key"`
	Label     string    `bson:"label"`
	Tracked   bool      `bson:"tracked"`
	PII       string    `bson:"pii"`
	Normalize []string  `bson:"normalize"`
	MaxValues int       `bson:"max_values"`
	Status    string    `bson:"status"`
	CreatedBy string    `bson:"created_by"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedBy string    `bson:"updated_by,omitempty"`
	UpdatedAt time.Time `bson:"updated_at,omitempty"`
	Backfill  *struct {
		Status   string    `bson:"status"`
		Progress float64   `bson:"progress"`
		From     time.Time `bson:"from"`
		Error    string    `bson:"error,omitempty"`
	} `bson:"backfill,omitempty"`
}

func (s *Store) Registry(ctx context.Context) ([]RegistryEntry, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollParamRegistry).Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "key", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []RegistryEntry{}
	return out, cur.All(ctx, &out)
}

func (s *Store) RegistryEntry(ctx context.Context, key string) (*RegistryEntry, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	var e RegistryEntry
	err := s.report(CollParamRegistry).FindOne(ctx, bson.D{{Key: "key", Value: key}}).Decode(&e)
	if isNoDocs(err) {
		return nil, nil
	}
	return &e, err
}

// paramMatch: stats_param_daily theo phạm vi + chiến dịch + kỳ.
func paramMatch(m Match, keys ...string) bson.D {
	d := ownersCond("owner", m)
	if len(m.Campaigns) > 0 {
		d = append(d, in("campaign_code", m.Campaigns))
	}
	if len(keys) == 1 {
		d = append(d, bson.E{Key: "key", Value: keys[0]})
	} else if len(keys) > 1 {
		d = append(d, in("key", keys))
	}
	return append(d, bson.E{Key: "date", Value: bson.D{{Key: "$gte", Value: m.From}, {Key: "$lte", Value: m.To}}})
}

type ParamKeyStat struct {
	Key    string `bson:"_id"`
	Clicks int64  `bson:"clicks"`
	Values int64  `bson:"values"`
}

// ParamKeyStats: theo key — tổng click và số giá trị có click trong kỳ.
func (s *Store) ParamKeyStats(ctx context.Context, m Match, keys []string) (map[string]ParamKeyStat, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollStatsParamDaily).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: append(paramMatch(m, keys...), bson.E{Key: "clicks", Value: bson.D{{Key: "$gt", Value: 0}}})}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: bson.D{{Key: "k", Value: "$key"}, {Key: "v", Value: "$value"}}}, {Key: "clicks", Value: bson.D{{Key: "$sum", Value: "$clicks"}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$_id.k"}, {Key: "clicks", Value: bson.D{{Key: "$sum", Value: "$clicks"}}}, {Key: "values", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	var docs []ParamKeyStat
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := map[string]ParamKeyStat{}
	for _, d := range docs {
		out[d.Key] = d
	}
	return out, nil
}

// ParamLinkCounts: số link mang từng key (trong phạm vi).
func (s *Store) ParamLinkCounts(ctx context.Context, m Match, keys []string) (map[string]int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(ownersCond("owner", m), in("key", keys))
	if len(m.Campaigns) > 0 {
		match = append(match, in("campaign_code", m.Campaigns))
	}
	cur, err := s.report(CollLinkParams).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$key"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	})
	if err != nil {
		return nil, err
	}
	return decodeCountMap(ctx, cur)
}

type ParamValueStat struct {
	Value     string `bson:"_id"`
	Clicks    int64  `bson:"clicks"`
	Unique    int64  `bson:"unique_clicks"`
	Bot       int64  `bson:"bot_clicks"`
	Susp      int64  `bson:"suspicious_clicks"`
	NewLinks  int64  `bson:"new_links"`
	Accounts  int64  `bson:"accounts"`
	Campaigns int64  `bson:"campaigns"`
}

// ParamValueStats: theo giá trị của key — chỉ số + số tài khoản / chiến dịch có click.
func (s *Store) ParamValueStats(ctx context.Context, m Match, key string) ([]ParamValueStat, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	sum := func(f string) bson.D { return bson.D{{Key: "$sum", Value: f}} }
	hasClick := bson.D{{Key: "$gt", Value: bson.A{"$clicks", 0}}}
	cur, err := s.report(CollStatsParamDaily).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: paramMatch(m, key)}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$value"},
			{Key: "clicks", Value: sum("$clicks")}, {Key: "unique_clicks", Value: sum("$unique_clicks")},
			{Key: "bot_clicks", Value: sum("$bot_clicks")}, {Key: "suspicious_clicks", Value: sum("$suspicious_clicks")},
			{Key: "new_links", Value: sum("$new_links")},
			{Key: "acc", Value: bson.D{{Key: "$addToSet", Value: bson.D{{Key: "$cond", Value: bson.A{hasClick, "$owner", "$$REMOVE"}}}}}},
			{Key: "camp", Value: bson.D{{Key: "$addToSet", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$and", Value: bson.A{hasClick, bson.D{{Key: "$ne", Value: bson.A{"$campaign_code", ""}}}}}}, "$campaign_code", "$$REMOVE"}}}}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "clicks", Value: 1}, {Key: "unique_clicks", Value: 1}, {Key: "bot_clicks", Value: 1}, {Key: "suspicious_clicks", Value: 1}, {Key: "new_links", Value: 1},
			{Key: "accounts", Value: bson.D{{Key: "$size", Value: "$acc"}}}, {Key: "campaigns", Value: bson.D{{Key: "$size", Value: "$camp"}}},
		}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	out := []ParamValueStat{}
	return out, cur.All(ctx, &out)
}

// ParamValueActiveLinks: số link có click theo giá trị (link_params ∩ link có click trong kỳ).
func (s *Store) ParamValueActiveLinks(ctx context.Context, m Match, key string) (map[string]int64, error) {
	active, err := s.ActiveLinkIDs(ctx, m)
	if err != nil || len(active) == 0 {
		return map[string]int64{}, err
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollLinkParams).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: bson.D{{Key: "key", Value: key}, {Key: "link_id", Value: bson.D{{Key: "$in", Value: active}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$value"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	}, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	return decodeCountMap(ctx, cur)
}

// ParamValueSeries: (giá trị, bucket) → clicks cho các giá trị chỉ định.
func (s *Store) ParamValueSeries(ctx context.Context, m Match, key string, values []string, g Granularity) (map[string]map[string]Sums, error) {
	out := map[string]map[string]Sums{}
	if len(values) == 0 {
		return out, nil
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(paramMatch(m, key), in("value", values))
	cur, err := s.report(CollStatsParamDaily).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "v", Value: "$value"}, {Key: "b", Value: dateKey(bucketExpr("$date", g, 0))}}},
			{Key: "clicks", Value: bson.D{{Key: "$sum", Value: "$clicks"}}},
			{Key: "unique_clicks", Value: bson.D{{Key: "$sum", Value: "$unique_clicks"}}},
			{Key: "bot_clicks", Value: bson.D{{Key: "$sum", Value: "$bot_clicks"}}},
			{Key: "new_links", Value: bson.D{{Key: "$sum", Value: "$new_links"}}},
		}}},
	})
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID struct {
			V string `bson:"v"`
			B string `bson:"b"`
		} `bson:"_id"`
		Clicks   int64 `bson:"clicks"`
		Unique   int64 `bson:"unique_clicks"`
		Bot      int64 `bson:"bot_clicks"`
		NewLinks int64 `bson:"new_links"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		if out[d.ID.V] == nil {
			out[d.ID.V] = map[string]Sums{}
		}
		out[d.ID.V][d.ID.B] = Sums{Clicks: d.Clicks, Unique: d.Unique, Bot: d.Bot, NewLinks: d.NewLinks}
	}
	return out, nil
}
