// Package testutil dựng MongoDB có dữ liệu seed cho test tích hợp.
//
// Cần MONGODB_TEST_URI (vd mongodb://localhost:27017 hoặc service container trong CI); không có → test bị skip.
// Mỗi lần chạy dùng 2 database tên ngẫu nhiên và xoá khi xong.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"am-shortlink-portal/api/internal/platform/mongodb"
	"am-shortlink-portal/api/internal/seed"
	"am-shortlink-portal/api/internal/store"
)

const Salt = "integration-test-pii-salt-0123456789"

type Env struct {
	DB      *mongodb.DB
	Store   *store.Store
	Dataset *seed.Dataset
}

var (
	once    sync.Once
	shared  *Env
	initErr error
	cleanup func()
)

// Seeded trả môi trường dùng chung cho cả package test (seed 1 lần).
func Seeded(t *testing.T) *Env {
	t.Helper()
	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		t.Skip("MONGODB_TEST_URI chưa đặt — bỏ qua test tích hợp")
	}
	once.Do(func() { shared, initErr = setup(uri) })
	if initErr != nil {
		t.Fatalf("seed test DB: %v", initErr)
	}
	return shared
}

// Teardown gọi trong TestMain sau m.Run().
func Teardown() {
	if cleanup != nil {
		cleanup()
	}
}

func setup(uri string) (*Env, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	suffix := hex.EncodeToString(b)
	core := client.Database("test_core_" + suffix)
	report := client.Database("test_report_" + suffix)
	cleanup = func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = core.Drop(c)
		_ = report.Drop(c)
		_ = client.Disconnect(c)
	}

	// "Bây giờ" cố định → kết quả xác định.
	now := time.Date(2026, 10, 7, 15, 0, 0, 0, seed.VN)
	ds := seed.Generate(seed.Options{Seed: 42, Days: 45, Links: 500, ClicksPerDay: 400, Now: now, PIIHashSalt: Salt})
	if _, err := seed.Write(ctx, core, report, ds); err != nil {
		cleanup()
		return nil, err
	}
	db := &mongodb.DB{Client: client, Core: core, Report: report, MaxTime: 30 * time.Second}
	st := store.New(db)
	if err := st.Migrate(ctx, slog.New(slog.DiscardHandler)); err != nil {
		cleanup()
		return nil, err
	}
	return &Env{DB: db, Store: st, Dataset: ds}, nil
}
