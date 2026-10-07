package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"am-shortlink-portal/api/migrations"
)

// Migrate áp các migration chưa chạy vào am_shortlink_report. Idempotent: chạy lại không lỗi.
func (s *Store) Migrate(ctx context.Context, log *slog.Logger) error {
	all, err := migrations.All()
	if err != nil {
		return err
	}
	applied := map[int]bool{}
	cur, err := s.report(CollMigrations).Find(ctx, bson.D{})
	if err != nil {
		return err
	}
	var done []struct {
		Version int `bson:"_id"`
	}
	if err := cur.All(ctx, &done); err != nil {
		return err
	}
	for _, d := range done {
		applied[d.Version] = true
	}

	existing, err := s.db.Report.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return err
	}
	has := map[string]bool{}
	for _, n := range existing {
		has[n] = true
	}

	for _, m := range all {
		if applied[m.Version] {
			continue
		}
		for _, c := range m.Collections {
			if has[c] {
				continue
			}
			if err := s.db.Report.CreateCollection(ctx, c); err != nil && !isNamespaceExists(err) {
				return fmt.Errorf("migration %d: tạo %s: %w", m.Version, c, err)
			}
			has[c] = true
		}
		for _, ix := range m.Indexes {
			keys := bson.D{}
			for _, k := range ix.Keys {
				keys = append(keys, bson.E{Key: fmt.Sprint(k[0]), Value: k[1]})
			}
			o := options.Index().SetName(ix.Name)
			if ix.Unique {
				o.SetUnique(true)
			}
			if ix.ExpireAfterSeconds != nil {
				o.SetExpireAfterSeconds(*ix.ExpireAfterSeconds)
			}
			if ix.Partial != nil {
				o.SetPartialFilterExpression(ix.Partial)
			}
			if _, err := s.report(ix.Collection).Indexes().CreateOne(ctx, mongo.IndexModel{Keys: keys, Options: o}); err != nil {
				return fmt.Errorf("migration %d: index %s.%s: %w", m.Version, ix.Collection, ix.Name, err)
			}
		}
		if _, err := s.report(CollMigrations).InsertOne(ctx, bson.D{
			{Key: "_id", Value: m.Version}, {Key: "description", Value: m.Description}, {Key: "applied_at", Value: time.Now()},
		}); err != nil {
			return err
		}
		log.Info("migration applied", "version", m.Version, "description", m.Description)
	}
	return nil
}

func isNamespaceExists(err error) bool {
	var ce mongo.CommandError
	return errors.As(err, &ce) && ce.Code == 48 // NamespaceExists
}
