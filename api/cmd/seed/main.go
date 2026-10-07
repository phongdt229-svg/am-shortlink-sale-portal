// Command seed (CHỈ DEV): sinh am_shortlink + am_shortlink_report giả lập đúng schema Service.
//
//	go run ./cmd/seed              # DB rỗng → sinh dữ liệu
//	go run ./cmd/seed -reset       # xoá 2 DB (chỉ khi do seed tạo) rồi sinh lại
//
// An toàn: từ chối khi APP_ENV=production; -reset chỉ xoá DB có dấu _seed_info (không xoá dữ liệu Service thật).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"am-shortlink-portal/api/internal/config"
	"am-shortlink-portal/api/internal/platform/mongodb"
	"am-shortlink-portal/api/internal/seed"
	"am-shortlink-portal/api/internal/store"
)

func main() {
	reset := flag.Bool("reset", false, "xoá DB do seed tạo trước đó rồi sinh lại")
	days := flag.Int("days", 120, "số ngày có click (tính cả hôm nay)")
	links := flag.Int("links", 3000, "số link")
	perDay := flag.Int("clicks-per-day", 1500, "trung bình click / ngày")
	seedN := flag.Uint64("seed", 20261007, "seed ngẫu nhiên (cùng seed → cùng dữ liệu)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log, *reset, seed.Options{Seed: *seedN, Days: *days, Links: *links, ClicksPerDay: *perDay}); err != nil {
		log.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, reset bool, o seed.Options) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProduction() || cfg.Env == "staging" {
		return fmt.Errorf("seed chỉ dùng cho local/dev (APP_ENV=%s)", cfg.Env)
	}
	o.PIIHashSalt = cfg.PIIHashSalt

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.Mongo.URI))
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background())
	core := client.Database(cfg.Mongo.CoreDB)
	report := client.Database(cfg.Mongo.ReportDB)

	for _, db := range []*mongo.Database{core, report} {
		names, err := db.ListCollectionNames(ctx, bson.D{})
		if err != nil {
			return err
		}
		if len(names) == 0 {
			continue
		}
		if !reset {
			return fmt.Errorf("database %s đã có dữ liệu — chạy lại với -reset", db.Name())
		}
	}
	if reset {
		n, err := core.Collection("_seed_info").CountDocuments(ctx, bson.D{})
		if err != nil {
			return err
		}
		coreNames, _ := core.ListCollectionNames(ctx, bson.D{})
		if len(coreNames) > 0 && n == 0 {
			return fmt.Errorf("database %s không do seed tạo (thiếu _seed_info) — từ chối xoá", core.Name())
		}
		log.Info("dropping", "core", core.Name(), "report", report.Name())
		if err := core.Drop(ctx); err != nil {
			return err
		}
		if err := report.Drop(ctx); err != nil {
			return err
		}
	}

	start := time.Now()
	ds := seed.Generate(o)
	log.Info("generated", "links", len(ds.Links), "clicks", len(ds.Clicks), "from", ds.From.Format("2006-01-02"), "to", ds.To.Format("2006-01-02"), "took", time.Since(start).Round(time.Millisecond))

	sum, err := seed.Write(ctx, core, report, ds)
	if err != nil {
		return err
	}
	// -reset xoá cả portal_* → tạo lại collection + index của Portal.
	db := &mongodb.DB{Client: client, Core: core, Report: report, MaxTime: time.Minute}
	if err := store.New(db).Migrate(ctx, log); err != nil {
		return err
	}
	log.Info("seed done", "users", sum.Users, "links", sum.Links, "clicks", sum.Clicks, "stats_docs", sum.StatsDocs, "took", time.Since(start).Round(time.Millisecond))
	fmt.Println()
	fmt.Println("Tài khoản dev:")
	for _, u := range ds.Users {
		fmt.Printf("  %-10s / %-11s role=%-6s portal_access=%-5v active=%v\n", u.Username, u.Password, u.Role, u.PortalAccess, u.Active)
	}
	return nil
}
