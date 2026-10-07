package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// SavedReport: portal_saved_reports. Query chỉ là trạng thái màn hình — khi chạy luôn áp phạm vi của người mở.
type SavedReport struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Owner       string        `bson:"owner"`
	Name        string        `bson:"name"`
	Description string        `bson:"description,omitempty"`
	Kind        string        `bson:"kind"`
	Query       bson.M        `bson:"query"`
	ShareToken  string        `bson:"share_token,omitempty"`
	CreatedAt   time.Time     `bson:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at"`
}

const MaxSavedPerUser = 200

func (s *Store) ListSaved(ctx context.Context, owner string) ([]SavedReport, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	cur, err := s.report(CollSavedReports).Find(ctx, bson.D{{Key: "owner", Value: owner}},
		options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetLimit(MaxSavedPerUser))
	if err != nil {
		return nil, err
	}
	out := []SavedReport{}
	return out, cur.All(ctx, &out)
}

func (s *Store) CountSaved(ctx context.Context, owner string) (int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.report(CollSavedReports).CountDocuments(ctx, bson.D{{Key: "owner", Value: owner}})
}

func (s *Store) InsertSaved(ctx context.Context, r *SavedReport) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	res, err := s.report(CollSavedReports).InsertOne(ctx, r)
	if err == nil {
		r.ID = res.InsertedID.(bson.ObjectID)
	}
	return err
}

// GetSaved: theo id, chỉ của owner. Không có → nil.
func (s *Store) GetSaved(ctx context.Context, owner, id string) (*SavedReport, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, nil
	}
	return s.findSaved(ctx, bson.D{{Key: "_id", Value: oid}, {Key: "owner", Value: owner}})
}

func (s *Store) GetSavedByToken(ctx context.Context, token string) (*SavedReport, error) {
	if token == "" {
		return nil, nil
	}
	return s.findSaved(ctx, bson.D{{Key: "share_token", Value: token}})
}

func (s *Store) findSaved(ctx context.Context, filter bson.D) (*SavedReport, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	var r SavedReport
	err := s.report(CollSavedReports).FindOne(ctx, filter).Decode(&r)
	if isNoDocs(err) {
		return nil, nil
	}
	return &r, err
}

// UpdateSaved: ghi đè name / description / kind / query / share_token (của owner).
func (s *Store) UpdateSaved(ctx context.Context, r *SavedReport) (bool, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	set := bson.D{
		{Key: "name", Value: r.Name}, {Key: "description", Value: r.Description}, {Key: "kind", Value: r.Kind},
		{Key: "query", Value: r.Query}, {Key: "updated_at", Value: r.UpdatedAt},
	}
	upd := bson.D{{Key: "$set", Value: set}}
	if r.ShareToken == "" {
		upd = append(upd, bson.E{Key: "$unset", Value: bson.D{{Key: "share_token", Value: ""}}})
	} else {
		set = append(set, bson.E{Key: "share_token", Value: r.ShareToken})
		upd[0].Value = set
	}
	res, err := s.report(CollSavedReports).UpdateOne(ctx, bson.D{{Key: "_id", Value: r.ID}, {Key: "owner", Value: r.Owner}}, upd)
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

func (s *Store) DeleteSaved(ctx context.Context, owner, id string) (bool, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return false, nil
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	res, err := s.report(CollSavedReports).DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}, {Key: "owner", Value: owner}})
	if err != nil {
		return false, err
	}
	return res.DeletedCount == 1, nil
}
