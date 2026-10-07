// Command migrate tạo collection + index của Portal trong am_shortlink_report.
// Chạy như K8s Job trước mỗi lần deploy; idempotent.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"am-shortlink-portal/api/internal/config"
	"am-shortlink-portal/api/internal/platform/mongodb"
	"am-shortlink-portal/api/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Migrate ghi vào primary.
	cfg.Mongo.ReadPreference = "primary"
	db, err := mongodb.Connect(ctx, cfg.Mongo, false)
	if err != nil {
		log.Error("mongo", "err", err)
		os.Exit(1)
	}
	defer db.Close(context.Background())

	if err := store.New(db).Migrate(ctx, log); err != nil {
		log.Error("migrate failed", "err", err)
		os.Exit(1)
	}
	log.Info("migrate done")
}
