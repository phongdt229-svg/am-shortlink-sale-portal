package store

import (
	"context"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// In: một tiêu chí lọc — OR giữa Values; Exclude = NOT.
type In struct {
	Values  []string
	Exclude bool
}

func (c In) empty() bool { return len(c.Values) == 0 }

// IDSet: tập link_id (từ mã link hoặc tham số URL) — gồm hoặc loại trừ.
type IDSet struct {
	IDs     []int64
	Exclude bool
}

// ClickQuery: tiêu chí lọc click thô (§4.1a). Luôn có phạm vi owner (M) + khoảng ngày.
type ClickQuery struct {
	M                                                                    Match // owners, campaigns, ctvs, prefixes (link_prefix)
	LinkSets                                                             []IDSet
	AccessPrefix, APIVersion, DestHost, Device, OS, Browser, SourceGroup In
	RefererHost, Country, Province                                       In
	IPExact                                                              In
	IPPrefixes                                                           []string // "113.161." (CIDR theo octet)
	IPPrefixExclude                                                      bool
	Weekdays                                                             []int
	HourFrom, HourTo                                                     *int
	IsCustom                                                             *bool
	LinkCreatedFrom, LinkCreatedTo                                       *time.Time // date-only
	Quality                                                              string     // valid | bot | suspicious | all
	Visit                                                                string     // all | first | repeat
	CampaignPresence                                                     string     // any | with | without
	CTVPresence                                                          string     // any | identified | unidentified
}

func inCond(field string, c In) bson.E {
	op := "$in"
	if c.Exclude {
		op = "$nin"
	}
	return bson.E{Key: field, Value: bson.D{{Key: op, Value: c.Values}}}
}

// clickMatch dựng điều kiện trên collection clicks (time-series). Điều kiện owner + ts đứng đầu để dùng index (meta.owner, ts).
func clickMatch(q ClickQuery) bson.D {
	m := q.M
	and := bson.A{}
	d := ownersCond("meta.owner", m)
	d = append(d, bson.E{Key: "ts", Value: bson.D{{Key: "$gte", Value: DayStart(m.From)}, {Key: "$lt", Value: DayStart(m.To).AddDate(0, 0, 1)}}})
	if len(m.Campaigns) > 0 {
		d = append(d, in("meta.campaign_code", m.Campaigns))
	}
	if len(m.CTVs) > 0 {
		d = append(d, in("meta.ctv_id", m.CTVs))
	}
	if len(m.Prefixes) > 0 {
		d = append(d, in("link_prefix", m.Prefixes))
	}
	if len(m.LinkIDs) > 0 {
		d = append(d, in("meta.link_id", m.LinkIDs))
	}
	for _, s := range q.LinkSets {
		op := "$in"
		if s.Exclude {
			op = "$nin"
		}
		ids := s.IDs
		if ids == nil {
			ids = []int64{}
		}
		and = append(and, bson.D{{Key: "meta.link_id", Value: bson.D{{Key: op, Value: ids}}}})
	}
	for field, c := range map[string]In{
		"access_prefix": q.AccessPrefix, "link_api_version": q.APIVersion, "dest_host": q.DestHost,
		"device": q.Device, "os": q.OS, "browser": q.Browser, "source_group": q.SourceGroup,
		"referer_host": q.RefererHost, "country": q.Country, "province": q.Province, "ip": q.IPExact,
	} {
		if !c.empty() {
			and = append(and, bson.D{inCond(field, c)})
		}
	}
	if len(q.IPPrefixes) > 0 {
		or := bson.A{}
		for _, p := range q.IPPrefixes {
			or = append(or, bson.D{{Key: "ip", Value: bson.Regex{Pattern: "^" + regexp.QuoteMeta(p)}}})
		}
		cond := bson.D{{Key: "$or", Value: or}}
		if q.IPPrefixExclude {
			cond = bson.D{{Key: "$nor", Value: or}}
		}
		and = append(and, cond)
	}
	if len(q.Weekdays) > 0 {
		and = append(and, bson.D{{Key: "weekday", Value: bson.D{{Key: "$in", Value: q.Weekdays}}}})
	}
	if q.HourFrom != nil && q.HourTo != nil {
		f, t := *q.HourFrom, *q.HourTo
		if f <= t {
			and = append(and, bson.D{{Key: "hour", Value: bson.D{{Key: "$gte", Value: f}, {Key: "$lte", Value: t}}}})
		} else { // qua nửa đêm, vd 22h–2h
			and = append(and, bson.D{{Key: "$or", Value: bson.A{
				bson.D{{Key: "hour", Value: bson.D{{Key: "$gte", Value: f}}}},
				bson.D{{Key: "hour", Value: bson.D{{Key: "$lte", Value: t}}}},
			}}})
		}
	}
	if q.IsCustom != nil {
		and = append(and, bson.D{{Key: "link_is_custom", Value: *q.IsCustom}})
	}
	if q.LinkCreatedFrom != nil || q.LinkCreatedTo != nil {
		r := bson.D{}
		if q.LinkCreatedFrom != nil {
			r = append(r, bson.E{Key: "$gte", Value: DayStart(*q.LinkCreatedFrom)})
		}
		if q.LinkCreatedTo != nil {
			r = append(r, bson.E{Key: "$lt", Value: DayStart(*q.LinkCreatedTo).AddDate(0, 0, 1)})
		}
		and = append(and, bson.D{{Key: "link_created_at", Value: r}})
	}
	switch q.Quality {
	case "valid":
		and = append(and, bson.D{{Key: "is_bot", Value: false}})
	case "bot":
		and = append(and, bson.D{{Key: "is_bot", Value: true}})
	case "suspicious":
		and = append(and, bson.D{{Key: "is_suspicious", Value: true}})
	}
	switch q.Visit {
	case "first":
		and = append(and, bson.D{{Key: "is_repeat", Value: false}})
	case "repeat":
		and = append(and, bson.D{{Key: "is_repeat", Value: true}})
	}
	switch q.CampaignPresence {
	case "with":
		and = append(and, bson.D{{Key: "meta.campaign_code", Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}}})
	case "without":
		and = append(and, bson.D{{Key: "meta.campaign_code", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}}})
	}
	switch q.CTVPresence {
	case "identified":
		and = append(and, bson.D{{Key: "meta.ctv_id", Value: bson.D{{Key: "$nin", Value: bson.A{"", nil}}}}})
	case "unidentified":
		and = append(and, bson.D{{Key: "meta.ctv_id", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}}})
	}
	if len(and) > 0 {
		d = append(d, bson.E{Key: "$and", Value: and})
	}
	return d
}

// ClickDoc: một click thô (chỉ các trường hiển thị; KHÔNG trả user_agent đầy đủ).
type ClickDoc struct {
	ID   bson.ObjectID `bson:"_id"`
	TS   time.Time     `bson:"ts"`
	Meta struct {
		LinkID   int64  `bson:"link_id"`
		Owner    string `bson:"owner"`
		Campaign string `bson:"campaign_code"`
		CTV      string `bson:"ctv_id"`
	} `bson:"meta"`
	IP           string `bson:"ip"`
	Country      string `bson:"country"`
	Province     string `bson:"province"`
	Device       string `bson:"device"`
	OS           string `bson:"os"`
	Browser      string `bson:"browser"`
	SourceGroup  string `bson:"source_group"`
	RefererHost  string `bson:"referer_host"`
	AccessPrefix string `bson:"access_prefix"`
	LinkPrefix   string `bson:"link_prefix"`
	IsBot        bool   `bson:"is_bot"`
	IsSuspicious bool   `bson:"is_suspicious"`
	IsRepeat     bool   `bson:"is_repeat"`
}

type ClickCursor struct {
	TS time.Time
	ID bson.ObjectID
}

// ListClicks: click mới nhất trước, phân trang con trỏ (ts, _id) — không dùng skip.
func (s *Store) ListClicks(ctx context.Context, q ClickQuery, after *ClickCursor, limit int64) ([]ClickDoc, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := clickMatch(q)
	if after != nil {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "ts", Value: bson.D{{Key: "$lt", Value: after.TS}}}},
			bson.D{{Key: "ts", Value: after.TS}, {Key: "_id", Value: bson.D{{Key: "$lt", Value: after.ID}}}},
		}})
	}
	cur, err := s.core(CollClicks).Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "ts", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(limit).
		SetProjection(bson.D{{Key: "user_agent", Value: 0}, {Key: "event_id", Value: 0}, {Key: "referer", Value: 0}}))
	if err != nil {
		return nil, err
	}
	out := []ClickDoc{}
	return out, cur.All(ctx, &out)
}

// CountClicks: đếm nhanh (dùng cho export ước lượng).
func (s *Store) CountClicks(ctx context.Context, q ClickQuery) (int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.core(CollClicks).CountDocuments(ctx, clickMatch(q))
}
