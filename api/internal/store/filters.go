package store

import (
	"context"
	"slices"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ListUsernames: tài khoản cho bộ lọc. owners rỗng + unrestricted → mọi user có link (admin).
func (s *Store) ListUsernames(ctx context.Context, q string, limit int, owners []string, unrestricted bool) ([]string, error) {
	n := clampLimit(limit, 50, 200)
	if !unrestricted {
		out := make([]string, 0, len(owners))
		for _, o := range owners {
			if q == "" || strings.Contains(strings.ToLower(o), strings.ToLower(q)) {
				out = append(out, o)
			}
		}
		slices.Sort(out)
		if int64(len(out)) > n {
			out = out[:n]
		}
		return out, nil
	}

	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := bson.D{}
	if q != "" {
		filter = append(filter, bson.E{Key: "username", Value: containsRegex(q)})
	}
	cur, err := s.core(CollUsers).Find(ctx, filter, options.Find().
		SetProjection(bson.D{{Key: "username", Value: 1}}).
		SetSort(bson.D{{Key: "username", Value: 1}}).
		SetLimit(n))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		Username string `bson:"username"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = d.Username
	}
	return out, nil
}

type CampaignOption struct {
	Code string `bson:"code"`
	Name string `bson:"name"`
}

// ListCampaigns: chiến dịch có số liệu thuộc phạm vi (ownerFilter đã do scope dựng trên trường "owner").
func (s *Store) ListCampaigns(ctx context.Context, q string, limit int, ownerFilter bson.D) ([]CampaignOption, error) {
	n := clampLimit(limit, 50, 200)
	ctx, cancel := s.ctx(ctx)
	defer cancel()

	if len(ownerFilter) == 0 {
		// Admin, không lọc tài khoản: đọc thẳng danh mục campaigns.
		filter := bson.D{}
		if q != "" {
			filter = bson.D{{Key: "$or", Value: bson.A{
				bson.D{{Key: "code", Value: containsRegex(q)}},
				bson.D{{Key: "name", Value: containsRegex(q)}},
			}}}
		}
		cur, err := s.core(CollCampaigns).Find(ctx, filter, options.Find().
			SetProjection(bson.D{{Key: "code", Value: 1}, {Key: "name", Value: 1}}).
			SetSort(bson.D{{Key: "code", Value: 1}}).SetLimit(n))
		if err != nil {
			return nil, err
		}
		out := []CampaignOption{}
		return out, cur.All(ctx, &out)
	}

	// Có phạm vi: lấy mã chiến dịch từ stats_campaign_daily (index (owner, campaign_code, date)),
	// rồi ghép tên từ campaigns (khác database nên không $lookup được).
	codeCond := bson.D{{Key: "$nin", Value: bson.A{"", nil}}}
	if q != "" {
		codeCond = append(codeCond, bson.E{Key: "$regex", Value: containsRegex(q)})
	}
	pipe := []bson.D{
		{{Key: "$match", Value: and(ownerFilter, bson.D{{Key: "campaign_code", Value: codeCond}})}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$campaign_code"}}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: n}},
	}
	cur, err := s.report(CollStatsCampaignDaily).Aggregate(ctx, pipe)
	if err != nil {
		return nil, err
	}
	var codes []struct {
		Code string `bson:"_id"`
	}
	if err := cur.All(ctx, &codes); err != nil {
		return nil, err
	}
	list := make([]string, len(codes))
	for i, c := range codes {
		list[i] = c.Code
	}
	names, err := s.campaignNames(ctx, list)
	if err != nil {
		return nil, err
	}
	out := make([]CampaignOption, len(list))
	for i, c := range list {
		out[i] = CampaignOption{Code: c, Name: names[c]}
	}
	return out, nil
}

// campaignNames tra tên chiến dịch theo mã (thiếu tên → dùng mã).
func (s *Store) campaignNames(ctx context.Context, codes []string) (map[string]string, error) {
	out := make(map[string]string, len(codes))
	if len(codes) == 0 {
		return out, nil
	}
	cur, err := s.core(CollCampaigns).Find(ctx, bson.D{{Key: "code", Value: bson.D{{Key: "$in", Value: codes}}}},
		options.Find().SetProjection(bson.D{{Key: "code", Value: 1}, {Key: "name", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var docs []CampaignOption
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, d := range docs {
		out[d.Code] = d.Name
	}
	for _, c := range codes {
		if out[c] == "" {
			out[c] = c
		}
	}
	return out, nil
}

type PrefixOption struct {
	ID          string `bson:"_id"`
	IsDefault   bool   `bson:"is_default"`
	Description string `bson:"description"`
}

func (s *Store) ListPrefixes(ctx context.Context) ([]PrefixOption, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.core(CollPrefixes).Find(ctx, bson.D{{Key: "active", Value: bson.D{{Key: "$ne", Value: false}}}},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []PrefixOption{}
	return out, cur.All(ctx, &out)
}
