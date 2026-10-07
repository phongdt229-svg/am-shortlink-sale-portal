package seed

// Ghi dataset vào MongoDB. Package seed là công cụ dev/test nên được dùng driver trực tiếp
// (ngoại lệ của quy tắc "chỉ store import driver").

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type Summary struct {
	Users, Links, Clicks, StatsDocs int
}

// Write ghi toàn bộ dataset vào core (am_shortlink) và report (am_shortlink_report).
// Gọi trên DB rỗng (cmd/seed lo kiểm tra + drop).
func Write(ctx context.Context, core, report *mongo.Database, ds *Dataset) (Summary, error) {
	var sum Summary
	if err := EnsureServiceSchema(ctx, core, report); err != nil {
		return sum, err
	}
	st := Aggregate(ds)

	// users
	var docs []any
	for i, u := range ds.Users {
		h, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return sum, err
		}
		hash := string(h)
		if i%2 == 1 {
			hash = "$2y$" + hash[4:] // hash kiểu PHP như dữ liệu migrate từ hệ cũ
		}
		va := u.ViewerAccounts
		if va == nil {
			va = []string{}
		}
		docs = append(docs, bson.D{
			{Key: "_id", Value: u.ID}, {Key: "username", Value: u.Username}, {Key: "email", Value: u.Email},
			{Key: "password_hash", Value: hash}, {Key: "role", Value: u.Role}, {Key: "active", Value: u.Active},
			{Key: "api_active", Value: true}, {Key: "api_quota", Value: 1000}, {Key: "prefix", Value: u.Prefix},
			{Key: "random_key_length", Value: 6}, {Key: "is_expires", Value: false}, {Key: "expires_value", Value: 0},
			{Key: "portal_access", Value: u.PortalAccess}, {Key: "viewer_accounts", Value: va},
			{Key: "created_at", Value: ds.From.AddDate(0, -6, 0)},
		})
	}
	if err := insert(ctx, core.Collection("users"), docs); err != nil {
		return sum, err
	}
	sum.Users = len(docs)

	// prefixes, campaigns, ctvs
	if err := insert(ctx, core.Collection("prefixes"), []any{
		bson.D{{Key: "_id", Value: "sale"}, {Key: "is_default", Value: true}, {Key: "is_default_v3", Value: false}, {Key: "active", Value: true}, {Key: "description", Value: "Prefix mặc định /sale"}},
		bson.D{{Key: "_id", Value: "lm"}, {Key: "is_default", Value: false}, {Key: "is_default_v3", Value: true}, {Key: "active", Value: true}, {Key: "description", Value: "Prefix /lm (mặc định API v3)"}},
	}); err != nil {
		return sum, err
	}
	campID := map[string]int64{}
	docs = docs[:0]
	for _, c := range ds.Campaigns {
		campID[c.Code] = c.ID
		docs = append(docs, bson.D{{Key: "_id", Value: c.ID}, {Key: "code", Value: c.Code}, {Key: "name", Value: c.Name},
			{Key: "created_by", Value: c.CreatedBy}, {Key: "created_at", Value: c.CreatedAt}, {Key: "updated_at", Value: c.CreatedAt}})
	}
	if err := insert(ctx, core.Collection("campaigns"), docs); err != nil {
		return sum, err
	}
	docs = docs[:0]
	for _, c := range ds.CTVs {
		d := bson.D{{Key: "_id", Value: c.ID}, {Key: "ctv_id", Value: c.ID}, {Key: "owner", Value: c.Owner}, {Key: "status", Value: "active"}}
		if c.Name != "" {
			d = append(d, bson.E{Key: "name", Value: c.Name})
		}
		docs = append(docs, d)
	}
	if err := insert(ctx, report.Collection("ctvs"), docs); err != nil {
		return sum, err
	}

	// links + link_params
	userID := map[string]int64{}
	for _, u := range ds.Users {
		userID[u.Username] = u.ID
	}
	docs = docs[:0]
	var params []any
	for _, l := range ds.Links {
		updated := l.CreatedAt
		if !l.StatusAt.IsZero() {
			updated = l.StatusAt
		}
		h := sha1.Sum([]byte(l.LongURL))
		d := bson.D{
			{Key: "_id", Value: l.ID}, {Key: "code", Value: l.Code}, {Key: "domain_id", Value: 1},
			{Key: "long_url", Value: l.LongURL}, {Key: "long_url_hash", Value: hex.EncodeToString(h[:])},
			{Key: "owner_id", Value: userID[l.Owner]}, {Key: "owner_username", Value: l.Owner},
			{Key: "campaign_code", Value: l.Campaign}, {Key: "ctv_raw", Value: l.CTVRaw}, {Key: "ctv_id", Value: l.CTV},
			{Key: "status", Value: l.Status}, {Key: "is_custom", Value: l.IsCustom}, {Key: "is_api", Value: l.APIVersion != "portal"},
			{Key: "api_version", Value: l.APIVersion}, {Key: "prefix", Value: l.Prefix}, {Key: "dest_host", Value: l.DestHost},
			{Key: "expires_at", Value: nil}, {Key: "clicks", Value: st.LinkClicks[l.ID]}, {Key: "ip", Value: "10.0.0.1"},
			{Key: "created_at", Value: l.CreatedAt.UTC()}, {Key: "updated_at", Value: updated.UTC()},
		}
		if id, ok := campID[l.Campaign]; ok {
			d = append(d, bson.E{Key: "campaign_id", Value: id})
		}
		docs = append(docs, d)
		for _, p := range l.Params {
			pd := bson.D{
				{Key: "link_id", Value: l.ID}, {Key: "owner", Value: l.Owner}, {Key: "campaign_code", Value: l.Campaign},
				{Key: "ctv_id", Value: l.CTV}, {Key: "key", Value: p.Key}, {Key: "value", Value: p.Value},
				{Key: "link_created_at", Value: l.CreatedAt.UTC()},
			}
			if p.Raw != "" {
				pd = append(pd, bson.E{Key: "value_raw", Value: p.Raw})
			}
			params = append(params, pd)
		}
	}
	if err := insert(ctx, core.Collection("links"), docs); err != nil {
		return sum, err
	}
	sum.Links = len(docs)
	if err := insert(ctx, report.Collection("link_params"), params); err != nil {
		return sum, err
	}

	// clicks (time-series)
	docs = docs[:0]
	for _, c := range ds.Clicks {
		ref := ""
		if c.RefererHost != "" {
			ref = "https://" + c.RefererHost + "/"
		}
		d := bson.D{
			{Key: "ts", Value: c.TS},
			{Key: "meta", Value: bson.D{{Key: "link_id", Value: c.LinkID}, {Key: "owner", Value: c.Owner},
				{Key: "campaign_code", Value: c.Campaign}, {Key: "ctv_id", Value: c.CTV}}},
			{Key: "ip", Value: c.IP}, {Key: "country", Value: c.Country}, {Key: "province", Value: c.Province},
			{Key: "device", Value: c.Device}, {Key: "os", Value: c.OS}, {Key: "browser", Value: c.Browser},
			{Key: "in_app", Value: c.InApp}, {Key: "referer", Value: ref}, {Key: "referer_host", Value: c.RefererHost},
			{Key: "source_group", Value: c.SourceGroup}, {Key: "access_prefix", Value: c.AccessPrefix},
			{Key: "link_prefix", Value: c.LinkPrefix}, {Key: "link_api_version", Value: c.APIVersion},
			{Key: "link_is_custom", Value: c.IsCustom}, {Key: "link_created_at", Value: c.LinkCreated},
			{Key: "dest_host", Value: c.DestHost}, {Key: "hour", Value: c.Hour}, {Key: "weekday", Value: c.Weekday},
			{Key: "is_bot", Value: c.IsBot}, {Key: "is_suspicious", Value: c.IsSuspicious}, {Key: "is_repeat", Value: c.IsRepeat},
			{Key: "user_agent", Value: c.UA}, {Key: "event_id", Value: c.EventID},
		}
		for _, k := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term"} {
			if v, ok := c.UTM[k]; ok {
				d = append(d, bson.E{Key: k, Value: v})
			}
		}
		docs = append(docs, d)
	}
	if err := insert(ctx, core.Collection("clicks"), docs); err != nil {
		return sum, err
	}
	sum.Clicks = len(docs)

	// stats_*
	n, err := writeStats(ctx, report, st)
	if err != nil {
		return sum, err
	}
	sum.StatsDocs = n

	// param_registry, param_values, click_facets
	docs = docs[:0]
	for _, r := range ds.Registry {
		d := bson.D{
			{Key: "_id", Value: r.Key}, {Key: "key", Value: r.Key}, {Key: "label", Value: r.Label},
			{Key: "tracked", Value: r.Tracked}, {Key: "pii", Value: r.PII}, {Key: "normalize", Value: nonNil(r.Normalize)},
			{Key: "max_values", Value: r.MaxValues}, {Key: "status", Value: r.Status},
			{Key: "created_by", Value: "system"}, {Key: "created_at", Value: ds.From.AddDate(0, -3, 0)},
		}
		if r.Tracked {
			d = append(d, bson.E{Key: "backfill", Value: bson.D{{Key: "status", Value: "done"}, {Key: "progress", Value: 1.0}, {Key: "from", Value: ds.From}}})
		}
		docs = append(docs, d)
	}
	if err := insert(ctx, report.Collection("param_registry"), docs); err != nil {
		return sum, err
	}
	docs = docs[:0]
	for _, k := range st.ParamValueKeys {
		pv := st.ParamValues[k]
		docs = append(docs, bson.D{
			{Key: "owner", Value: pv.Owner}, {Key: "key", Value: pv.Key}, {Key: "value", Value: pv.Value},
			{Key: "first_seen", Value: pv.FirstSeen.UTC()}, {Key: "last_seen", Value: pv.LastSeen.UTC()},
			{Key: "links", Value: len(pv.links)}, {Key: "clicks_total", Value: pv.ClicksTotal},
		})
	}
	if err := insert(ctx, report.Collection("param_values"), docs); err != nil {
		return sum, err
	}
	docs = docs[:0]
	for _, k := range st.FacetKeys {
		f := st.Facets[k]
		docs = append(docs, bson.D{{Key: "owner", Value: f.Owner}, {Key: "field", Value: f.Field}, {Key: "value", Value: f.Value},
			{Key: "clicks", Value: f.Clicks}, {Key: "last_date", Value: f.LastDate}})
	}
	if err := insert(ctx, report.Collection("click_facets"), docs); err != nil {
		return sum, err
	}

	_, err = core.Collection("_seed_info").InsertOne(ctx, bson.D{
		{Key: "_id", Value: "seed"}, {Key: "generated_at", Value: time.Now()},
		{Key: "from", Value: ds.From}, {Key: "to", Value: ds.To},
		{Key: "links", Value: sum.Links}, {Key: "clicks", Value: sum.Clicks},
	})
	return sum, err
}

func writeStats(ctx context.Context, report *mongo.Database, st *Stats) (int, error) {
	total := 0
	type spec struct {
		coll   string
		g      *group
		hasNew bool
		hasBy  bool
		snap   string // "" | owner | system
	}
	specs := []spec{
		{"stats_link_daily", st.LinkDaily, false, true, ""},
		{"stats_campaign_daily", st.CampaignDaily, true, true, ""},
		{"stats_ctv_daily", st.CTVDaily, true, false, ""},
		{"stats_owner_daily", st.OwnerDaily, true, true, "owner"},
		{"stats_system_daily", st.SystemDaily, true, true, "system"},
		{"stats_param_daily", st.ParamDaily, true, true, ""},
		{"stats_link_monthly", st.LinkMonthly, false, true, ""},
		{"stats_campaign_monthly", st.CampaignMonthly, true, true, ""},
		{"stats_ctv_monthly", st.CTVMonthly, true, false, ""},
		{"stats_owner_monthly", st.OwnerMonthly, true, true, ""},
		{"stats_system_monthly", st.SystemMonthly, true, true, ""},
		{"stats_param_monthly", st.ParamMonthly, true, true, ""},
	}
	for _, sp := range specs {
		var docs []any
		for _, k := range sp.g.keys {
			a := sp.g.m[k]
			attr := sp.g.attr[k]
			d := bson.D{}
			for _, f := range []string{"link_id", "owner", "campaign_code", "ctv_id", "key", "value", "prefix", "date", "month"} {
				if v, ok := attr[f]; ok {
					d = append(d, bson.E{Key: f, Value: v})
				}
			}
			d = append(d,
				bson.E{Key: "clicks", Value: a.Clicks}, bson.E{Key: "unique_clicks", Value: a.Unique},
				bson.E{Key: "bot_clicks", Value: a.Bot}, bson.E{Key: "suspicious_clicks", Value: a.Susp},
			)
			if sp.hasNew {
				d = append(d, bson.E{Key: "new_links", Value: a.NewLinks}, bson.E{Key: "active_links", Value: int64(len(a.active))})
			}
			if sp.hasBy {
				d = append(d,
					bson.E{Key: "by_hour", Value: a.ByHour[:]},
					bson.E{Key: "by_device", Value: a.ByDevice}, bson.E{Key: "by_referer", Value: topN(a.ByReferer, 20)},
					bson.E{Key: "by_country", Value: a.ByCountry}, bson.E{Key: "by_os", Value: a.ByOS},
					bson.E{Key: "by_browser", Value: a.ByBrowser}, bson.E{Key: "by_source_group", Value: a.BySource},
					bson.E{Key: "by_prefix", Value: a.ByPrefix},
				)
			}
			switch sp.snap {
			case "owner":
				d = append(d, bson.E{Key: "total_links_snapshot", Value: st.TotalSnap[attr["owner"].(string)+"|"+attr["date"].(time.Time).Format("2006-01-02")]})
			case "system":
				d = append(d, bson.E{Key: "total_links_snapshot", Value: st.TotalSnap["|"+attr["date"].(time.Time).Format("2006-01-02")]})
			}
			docs = append(docs, d)
		}
		if err := insert(ctx, report.Collection(sp.coll), docs); err != nil {
			return total, fmt.Errorf("%s: %w", sp.coll, err)
		}
		total += len(docs)
	}
	return total, nil
}

// topN giữ N khoá lớn nhất + "other" (giống Service ghi by_referer).
func topN(m map[string]int64, n int) map[string]int64 {
	if len(m) <= n {
		return m
	}
	type kv struct {
		k string
		v int64
	}
	var all []kv
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sortKV := func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].k < all[j].k
	}
	sortSlice(all, sortKV)
	out := map[string]int64{}
	for i, e := range all {
		if i < n {
			out[e.k] = e.v
		} else {
			out["other"] += e.v
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func insert(ctx context.Context, c *mongo.Collection, docs []any) error {
	const batch = 5000
	for i := 0; i < len(docs); i += batch {
		end := min(i+batch, len(docs))
		if _, err := c.InsertMany(ctx, docs[i:end], options.InsertMany().SetOrdered(false)); err != nil {
			return fmt.Errorf("insert %s: %w", c.Name(), err)
		}
	}
	return nil
}
