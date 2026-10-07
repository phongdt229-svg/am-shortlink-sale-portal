package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// LinkInfo: thông tin link để hiển thị (KHÔNG có ctv_raw / ip của người tạo).
type LinkInfo struct {
	ID         int64     `bson:"_id"`
	Code       string    `bson:"code"`
	LongURL    string    `bson:"long_url"`
	DestHost   string    `bson:"dest_host"`
	Owner      string    `bson:"owner_username"`
	Campaign   string    `bson:"campaign_code"`
	CTV        string    `bson:"ctv_id"`
	Status     string    `bson:"status"`
	Prefix     string    `bson:"prefix"`
	IsCustom   bool      `bson:"is_custom"`
	APIVersion string    `bson:"api_version"`
	CreatedAt  time.Time `bson:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

var linkInfoProjection = bson.D{
	{Key: "code", Value: 1}, {Key: "long_url", Value: 1}, {Key: "dest_host", Value: 1}, {Key: "owner_username", Value: 1},
	{Key: "campaign_code", Value: 1}, {Key: "ctv_id", Value: 1}, {Key: "status", Value: 1}, {Key: "prefix", Value: 1},
	{Key: "is_custom", Value: 1}, {Key: "api_version", Value: 1}, {Key: "created_at", Value: 1}, {Key: "updated_at", Value: 1},
}

// LinksByIDs tra thông tin link (giữ phạm vi: owner phải thuộc phạm vi m).
func (s *Store) LinksByIDs(ctx context.Context, m Match, ids []int64) (map[int64]LinkInfo, error) {
	out := map[int64]LinkInfo{}
	if len(ids) == 0 {
		return out, nil
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := append(ownersCond("owner_username", m), bson.E{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}})
	cur, err := s.core(CollLinks).Find(ctx, filter, options.Find().SetProjection(linkInfoProjection))
	if err != nil {
		return nil, err
	}
	var docs []LinkInfo
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		out[d.ID] = d
	}
	return out, nil
}

// CampaignNames: mã → tên (thiếu → mã).
func (s *Store) CampaignNames(ctx context.Context, codes []string) (map[string]string, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.campaignNames(ctx, codes)
}

// CTVNames: ctv_id → tên trong danh mục ctvs (nếu có).
func (s *Store) CTVNames(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollCTVs).Find(ctx, bson.D{{Key: "ctv_id", Value: bson.D{{Key: "$in", Value: ids}}}},
		options.Find().SetProjection(bson.D{{Key: "ctv_id", Value: 1}, {Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID   string `bson:"ctv_id"`
		Name string `bson:"name"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		if d.Name != "" {
			out[d.ID] = d.Name
		}
	}
	return out, nil
}

// PIIKeys: tham số PII trong param_registry → chế độ ("hash" | "drop").
func (s *Store) PIIKeys(ctx context.Context) (map[string]string, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollParamRegistry).Find(ctx, bson.D{{Key: "pii", Value: bson.D{{Key: "$in", Value: bson.A{"hash", "drop"}}}}},
		options.Find().SetProjection(bson.D{{Key: "key", Value: 1}, {Key: "pii", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		Key string `bson:"key"`
		PII string `bson:"pii"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(docs))
	for _, d := range docs {
		out[d.Key] = d.PII
	}
	return out, nil
}
