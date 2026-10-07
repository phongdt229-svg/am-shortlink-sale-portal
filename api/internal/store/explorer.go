package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ErrTooManyGroups: số nhóm vượt giới hạn — gợi ý thu hẹp bộ lọc / bớt chiều.
var ErrTooManyGroups = errors.New("too many groups")

// Stage1Row: nhóm theo (các chiều + link). Bước 2 (gom tiếp, tính link có click, nhóm theo tham số URL) làm ở report.
type Stage1Row struct {
	Keys   map[string]string
	LinkID int64
	Owner  string
	CTV    string
	Sums   Sums
	First  time.Time
	Last   time.Time
}

// ClickDims: chiều nhóm trên collection clicks → biểu thức.
func clickDimExpr(dim string) (any, bool) {
	fields := map[string]string{
		"account": "$meta.owner", "campaign": "$meta.campaign_code", "ctv": "$meta.ctv_id",
		"link_prefix": "$link_prefix", "access_prefix": "$access_prefix", "api_version": "$link_api_version",
		"dest_host": "$dest_host", "device": "$device", "os": "$os", "browser": "$browser",
		"source_group": "$source_group", "referer_host": "$referer_host", "country": "$country", "province": "$province",
	}
	if f, ok := fields[dim]; ok {
		return bson.D{{Key: "$ifNull", Value: bson.A{f, ""}}}, true
	}
	switch dim {
	case "hour":
		return bson.D{{Key: "$toString", Value: "$hour"}}, true
	case "weekday":
		return bson.D{{Key: "$toString", Value: "$weekday"}}, true
	case "day", "week", "month":
		trunc := bson.D{{Key: "date", Value: "$ts"}, {Key: "unit", Value: dim}, {Key: "timezone", Value: VN.String()}}
		if dim == "week" {
			trunc = append(trunc, bson.E{Key: "startOfWeek", Value: "monday"})
		}
		return bson.D{{Key: "$dateToString", Value: bson.D{
			{Key: "format", Value: "%Y-%m-%d"}, {Key: "timezone", Value: VN.String()},
			{Key: "date", Value: bson.D{{Key: "$dateTrunc", Value: trunc}}},
		}}}, true
	}
	return nil, false
}

// statsDimExpr: chiều có sẵn trên stats_link_daily.
func statsDimExpr(dim string) (any, bool) {
	switch dim {
	case "account":
		return "$owner", true
	case "campaign":
		return bson.D{{Key: "$ifNull", Value: bson.A{"$campaign_code", ""}}}, true
	case "ctv":
		return bson.D{{Key: "$ifNull", Value: bson.A{"$ctv_id", ""}}}, true
	case "link_prefix":
		return "$prefix", true
	case "day", "week", "month":
		g := map[string]Granularity{"day": Day, "week": Week, "month": Month}[dim]
		return dateKey(bucketExpr("$date", g, 0)), true
	}
	return nil, false
}

// StatsDims: chiều Explorer đọc được từ stats_link_daily (≤ 12 tháng).
var StatsDims = map[string]bool{"account": true, "campaign": true, "ctv": true, "link": true, "link_prefix": true, "day": true, "week": true, "month": true}

func groupID(dims []string, expr func(string) (any, bool), linkField string) (bson.D, error) {
	id := bson.D{}
	for _, d := range dims {
		if d == "link" || strings.HasPrefix(d, "param.") {
			continue // suy từ link ở bước 2
		}
		e, ok := expr(d)
		if !ok {
			return nil, fmt.Errorf("chiều %q không hỗ trợ ở nguồn dữ liệu này", d)
		}
		id = append(id, bson.E{Key: d, Value: e})
	}
	id = append(id, bson.E{Key: "_l", Value: linkField})
	return id, nil
}

type stage1Doc struct {
	ID     bson.M    `bson:"_id"`
	Owner  string    `bson:"o"`
	CTV    string    `bson:"c"`
	Clicks int64     `bson:"clicks"`
	Unique int64     `bson:"unique"`
	Bot    int64     `bson:"bot"`
	Susp   int64     `bson:"susp"`
	First  time.Time `bson:"first"`
	Last   time.Time `bson:"last"`
}

func decodeStage1(docs []stage1Doc) []Stage1Row {
	out := make([]Stage1Row, len(docs))
	for i, d := range docs {
		r := Stage1Row{Keys: map[string]string{}, Owner: d.Owner, CTV: d.CTV, First: d.First, Last: d.Last,
			Sums: Sums{Clicks: d.Clicks, Unique: d.Unique, Bot: d.Bot, Susp: d.Susp}}
		for k, v := range d.ID {
			if k == "_l" {
				switch x := v.(type) {
				case int64:
					r.LinkID = x
				case int32:
					r.LinkID = int64(x)
				}
				continue
			}
			r.Keys[k] = fmt.Sprint(v)
		}
		out[i] = r
	}
	return out
}

// ExplorerClicks: bước 1 trên click thô (≤ 3 tháng).
func (s *Store) ExplorerClicks(ctx context.Context, q ClickQuery, dims []string, maxRows int) ([]Stage1Row, error) {
	id, err := groupID(dims, clickDimExpr, "$meta.link_id")
	if err != nil {
		return nil, err
	}
	notBot := bson.D{{Key: "$eq", Value: bson.A{"$is_bot", false}}}
	cond := func(c any) bson.D {
		return bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{c, 1, 0}}}}}
	}
	group := bson.D{
		{Key: "_id", Value: id},
		{Key: "o", Value: bson.D{{Key: "$first", Value: "$meta.owner"}}},
		{Key: "c", Value: bson.D{{Key: "$first", Value: "$meta.ctv_id"}}},
		{Key: "clicks", Value: cond(notBot)},
		{Key: "unique", Value: cond(bson.D{{Key: "$and", Value: bson.A{notBot, bson.D{{Key: "$eq", Value: bson.A{"$is_repeat", false}}}}}})},
		{Key: "bot", Value: cond(bson.D{{Key: "$eq", Value: bson.A{"$is_bot", true}}})},
		{Key: "susp", Value: cond(bson.D{{Key: "$and", Value: bson.A{notBot, bson.D{{Key: "$eq", Value: bson.A{"$is_suspicious", true}}}}}})},
		{Key: "first", Value: bson.D{{Key: "$min", Value: "$ts"}}},
		{Key: "last", Value: bson.D{{Key: "$max", Value: "$ts"}}},
	}
	return s.stage1(ctx, s.core(CollClicks).Name(), true, []bson.D{
		{{Key: "$match", Value: clickMatch(q)}},
		{{Key: "$group", Value: group}},
		{{Key: "$limit", Value: maxRows + 1}},
	}, maxRows)
}

// ExplorerStats: bước 1 trên stats_link_daily (≤ 12 tháng) — chỉ chiều có sẵn trong stats.
func (s *Store) ExplorerStats(ctx context.Context, m Match, dims []string, maxRows int) ([]Stage1Row, error) {
	id, err := groupID(dims, statsDimExpr, "$link_id")
	if err != nil {
		return nil, err
	}
	sum := func(f string) bson.D { return bson.D{{Key: "$sum", Value: f}} }
	hasClick := bson.D{{Key: "$gt", Value: bson.A{"$clicks", 0}}}
	group := bson.D{
		{Key: "_id", Value: id},
		{Key: "o", Value: bson.D{{Key: "$first", Value: "$owner"}}},
		{Key: "c", Value: bson.D{{Key: "$first", Value: "$ctv_id"}}},
		{Key: "clicks", Value: sum("$clicks")},
		{Key: "unique", Value: sum("$unique_clicks")},
		{Key: "bot", Value: sum("$bot_clicks")},
		{Key: "susp", Value: sum("$suspicious_clicks")},
		{Key: "first", Value: bson.D{{Key: "$min", Value: bson.D{{Key: "$cond", Value: bson.A{hasClick, "$date", nil}}}}}},
		{Key: "last", Value: bson.D{{Key: "$max", Value: bson.D{{Key: "$cond", Value: bson.A{hasClick, "$date", nil}}}}}},
	}
	return s.stage1(ctx, CollStatsLinkDaily, false, []bson.D{
		{{Key: "$match", Value: statsMatch(m)}},
		{{Key: "$group", Value: group}},
		{{Key: "$limit", Value: maxRows + 1}},
	}, maxRows)
}

func (s *Store) stage1(ctx context.Context, coll string, core bool, pipe []bson.D, maxRows int) ([]Stage1Row, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	c := s.report(coll)
	if core {
		c = s.core(coll)
	}
	cur, err := c.Aggregate(ctx, pipe, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return nil, err
	}
	var docs []stage1Doc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	if len(docs) > maxRows {
		return nil, ErrTooManyGroups
	}
	return decodeStage1(docs), nil
}

// LinkParamValues: link_id → giá trị của tham số key (để nhóm theo param.<key>).
func (s *Store) LinkParamValues(ctx context.Context, ids []int64, key string) (map[int64]string, error) {
	out := map[int64]string{}
	if len(ids) == 0 {
		return out, nil
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollLinkParams).Find(ctx,
		bson.D{{Key: "key", Value: key}, {Key: "link_id", Value: bson.D{{Key: "$in", Value: ids}}}},
		options.Find().SetProjection(bson.D{{Key: "link_id", Value: 1}, {Key: "value", Value: 1}, {Key: "_id", Value: 0}}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID    int64  `bson:"link_id"`
		Value string `bson:"value"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		out[d.ID] = d.Value
	}
	return out, nil
}

// FacetFields: trường có trong click_facets.
var FacetFields = map[string]bool{
	"device": true, "os": true, "browser": true, "source_group": true, "referer_host": true, "country": true,
	"province": true, "access_prefix": true, "link_api_version": true, "dest_host": true,
}

type Facet struct {
	Value  string `bson:"_id"`
	Clicks int64  `bson:"n"`
}

// ClickFacets: giá trị cho dropdown (gộp theo các tài khoản trong phạm vi).
func (s *Store) ClickFacets(ctx context.Context, m Match, field, q string, limit int64) ([]Facet, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(ownersCond("owner", m), bson.E{Key: "field", Value: field})
	if q != "" {
		match = append(match, bson.E{Key: "value", Value: containsRegex(q)})
	}
	return s.facetAgg(ctx, s.report(CollClickFacets).Name(), match, "$clicks", limit)
}

// ParamFacets: giá trị tham số URL (param_values).
func (s *Store) ParamFacets(ctx context.Context, m Match, key, q string, limit int64) ([]Facet, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	match := append(ownersCond("owner", m), bson.E{Key: "key", Value: key})
	if q != "" {
		match = append(match, bson.E{Key: "value", Value: containsRegex(q)})
	}
	return s.facetAgg(ctx, CollParamValues, match, "$clicks_total", limit)
}

func (s *Store) facetAgg(ctx context.Context, coll string, match bson.D, sumField string, limit int64) ([]Facet, error) {
	cur, err := s.report(coll).Aggregate(ctx, []bson.D{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$value"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: sumField}}}}}},
		{{Key: "$sort", Value: bson.D{{Key: "n", Value: -1}, {Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: limit}},
	})
	if err != nil {
		return nil, err
	}
	out := []Facet{}
	return out, cur.All(ctx, &out)
}
