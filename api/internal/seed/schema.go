package seed

import (
	"context"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureServiceSchema tạo collection + index THEO HỢP ĐỒNG của Service (docs/PLAN_SERVICE_API.md §4, §5.4, §5.4b).
// Trên môi trường thật Service tạo các index này (migrations/ của Service); seed dựng lại để dev / test
// có cùng kế hoạch truy vấn (explain) như production.
func EnsureServiceSchema(ctx context.Context, core, report *mongo.Database) error {
	// clicks: time-series
	err := core.CreateCollection(ctx, "clicks", options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().SetTimeField("ts").SetMetaField("meta").SetGranularity("minutes")))
	if err != nil {
		return fmt.Errorf("tạo clicks time-series: %w", err)
	}

	idx := map[string]map[string][]mongo.IndexModel{
		core.Name(): {
			"users":     {uniq("username")},
			"campaigns": {uniq("code")},
			"links": {
				uniq("code"),
				ix("owner_username", "created_at"),
				ix("owner_username", "campaign_code", "created_at"),
				ix("owner_username", "long_url_hash"),
				ix("campaign_code", "created_at"),
				ix("ctv_id", "created_at"),
			},
			"clicks": {
				ix("meta.owner", "ts"), ix("meta.campaign_code", "ts"), ix("meta.ctv_id", "ts"), ix("meta.link_id", "ts"),
			},
		},
		report.Name(): {
			// (date), (prefix, date): Portal đề xuất thêm vào hợp đồng (lọc prefix / toàn hệ thống trên mức link).
			"stats_link_daily":       {uniq("link_id", "date"), ix("owner", "date"), ix("campaign_code", "date"), ix("ctv_id", "date"), ix("prefix", "date"), ix("date")},
			"stats_campaign_daily":   {uniq("owner", "campaign_code", "date"), ix("campaign_code", "date")},
			"stats_ctv_daily":        {uniq("ctv_id", "owner", "campaign_code", "date"), ix("owner", "date"), ix("campaign_code", "date")},
			"stats_owner_daily":      {uniq("owner", "date")},
			"stats_system_daily":     {uniq("date")},
			"stats_param_daily":      {uniq("owner", "campaign_code", "key", "value", "date"), ix("key", "date")},
			"stats_link_monthly":     {uniq("link_id", "month"), ix("owner", "month")},
			"stats_campaign_monthly": {uniq("owner", "campaign_code", "month"), ix("campaign_code", "month")},
			"stats_ctv_monthly":      {uniq("ctv_id", "owner", "campaign_code", "month"), ix("owner", "month")},
			"stats_owner_monthly":    {uniq("owner", "month")},
			"stats_system_monthly":   {uniq("month")},
			"stats_param_monthly":    {uniq("owner", "campaign_code", "key", "value", "month"), ix("key", "month")},
			"link_params":            {uniq("link_id", "key"), ix("owner", "key", "value"), ix("key", "value"), ix("campaign_code", "key", "value")},
			"param_values":           {uniq("owner", "key", "value")},
			"click_facets":           {uniq("owner", "field", "value"), ix("field", "clicks")},
			"ctvs":                   {ix("owner")},
		},
	}
	dbs := map[string]*mongo.Database{core.Name(): core, report.Name(): report}
	for dbName, colls := range idx {
		names := make([]string, 0, len(colls))
		for c := range colls {
			names = append(names, c)
		}
		sort.Strings(names)
		for _, c := range names {
			if _, err := dbs[dbName].Collection(c).Indexes().CreateMany(ctx, colls[c]); err != nil {
				return fmt.Errorf("index %s.%s: %w", dbName, c, err)
			}
		}
	}
	return nil
}

func keys(fields ...string) bson.D {
	d := bson.D{}
	for _, f := range fields {
		d = append(d, bson.E{Key: f, Value: 1})
	}
	return d
}

func ix(fields ...string) mongo.IndexModel { return mongo.IndexModel{Keys: keys(fields...)} }

func uniq(fields ...string) mongo.IndexModel {
	return mongo.IndexModel{Keys: keys(fields...), Options: options.Index().SetUnique(true)}
}

func sortSlice[T any](s []T, less func(i, j int) bool) { sort.Slice(s, less) }
