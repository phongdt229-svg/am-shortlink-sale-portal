package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ExportJob: portal_exports. Lưu ảnh chụp người tạo (vai trò + tài khoản được xem) để worker chạy đúng phạm vi.
type ExportJob struct {
	ID             bson.ObjectID `bson:"_id,omitempty"`
	Kind           string        `bson:"kind"`
	Format         string        `bson:"format"`
	Name           string        `bson:"name"`
	Params         bson.M        `bson:"params"`
	Status         string        `bson:"status"` // queued | running | done | failed
	CreatedBy      string        `bson:"created_by"`
	Role           string        `bson:"role"`
	ViewerAccounts []string      `bson:"viewer_accounts,omitempty"`
	Rows           int64         `bson:"rows"`
	SizeBytes      int64         `bson:"size_bytes"`
	File           string        `bson:"file,omitempty"`
	Error          string        `bson:"error,omitempty"`
	Worker         string        `bson:"worker,omitempty"`
	CreatedAt      time.Time     `bson:"created_at"`
	StartedAt      *time.Time    `bson:"started_at,omitempty"`
	FinishedAt     *time.Time    `bson:"finished_at,omitempty"`
	HeartbeatAt    *time.Time    `bson:"heartbeat_at,omitempty"`
	ExpiresAt      time.Time     `bson:"expires_at"` // TTL index → Mongo tự xoá bản ghi
}

func (s *Store) InsertExport(ctx context.Context, j *ExportJob) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	res, err := s.report(CollExports).InsertOne(ctx, j)
	if err == nil {
		j.ID = res.InsertedID.(bson.ObjectID)
	}
	return err
}

// ListExports: owner rỗng = tất cả (admin).
func (s *Store) ListExports(ctx context.Context, owner string, limit int64) ([]ExportJob, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := bson.D{}
	if owner != "" {
		filter = bson.D{{Key: "created_by", Value: owner}}
	}
	cur, err := s.report(CollExports).Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(limit))
	if err != nil {
		return nil, err
	}
	out := []ExportJob{}
	return out, cur.All(ctx, &out)
}

func (s *Store) GetExport(ctx context.Context, id string) (*ExportJob, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, nil
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	var j ExportJob
	err = s.report(CollExports).FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&j)
	if isNoDocs(err) {
		return nil, nil
	}
	return &j, err
}

// CountActiveExports: job chưa xong của 1 người (giới hạn đồng thời).
func (s *Store) CountActiveExports(ctx context.Context, owner string) (int64, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	return s.report(CollExports).CountDocuments(ctx, bson.D{{Key: "created_by", Value: owner}, {Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"queued", "running"}}}}})
}

// ClaimExport: nhận 1 job (queued, hoặc running nhưng worker chết — heartbeat cũ hơn staleAfter). Atomic.
func (s *Store) ClaimExport(ctx context.Context, worker string, now time.Time, staleAfter time.Duration) (*ExportJob, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	filter := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "status", Value: "queued"}},
		bson.D{{Key: "status", Value: "running"}, {Key: "heartbeat_at", Value: bson.D{{Key: "$lt", Value: now.Add(-staleAfter)}}}},
	}}}
	var j ExportJob
	err := s.report(CollExports).FindOneAndUpdate(ctx, filter,
		bson.D{{Key: "$set", Value: bson.D{{Key: "status", Value: "running"}, {Key: "worker", Value: worker}, {Key: "started_at", Value: now}, {Key: "heartbeat_at", Value: now}}}},
		options.FindOneAndUpdate().SetSort(bson.D{{Key: "created_at", Value: 1}}).SetReturnDocument(options.After)).Decode(&j)
	if isNoDocs(err) {
		return nil, nil
	}
	return &j, err
}

func (s *Store) HeartbeatExport(ctx context.Context, id bson.ObjectID, rows int64, now time.Time) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	_, err := s.report(CollExports).UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$set", Value: bson.D{{Key: "heartbeat_at", Value: now}, {Key: "rows", Value: rows}}}})
	return err
}

func (s *Store) FinishExport(ctx context.Context, id bson.ObjectID, status, file, errMsg string, rows, size int64, now time.Time) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	set := bson.D{{Key: "status", Value: status}, {Key: "rows", Value: rows}, {Key: "size_bytes", Value: size}, {Key: "finished_at", Value: now}}
	if file != "" {
		set = append(set, bson.E{Key: "file", Value: file})
	}
	if errMsg != "" {
		set = append(set, bson.E{Key: "error", Value: errMsg})
	}
	_, err := s.report(CollExports).UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{{Key: "$set", Value: set}})
	return err
}
