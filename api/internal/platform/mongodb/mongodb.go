// Package mongodb tạo client MongoDB dùng chung cho portal-api.
//
// Đọc ưu tiên secondary / node analytics (không ảnh hưởng redirect, API của Service).
// Mọi truy vấn phải đặt maxTimeMS (MaxTime) — store dùng Opts.MaxTime khi dựng option.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"

	"am-shortlink-portal/api/internal/config"
)

// DB gom 2 database: Core (am_shortlink, chỉ đọc) và Report (am_shortlink_report).
type DB struct {
	Client  *mongo.Client
	Core    *mongo.Database
	Report  *mongo.Database
	MaxTime time.Duration
}

func Connect(ctx context.Context, c config.Mongo, tracing bool) (*DB, error) {
	mode, err := readpref.ModeFromString(c.ReadPreference)
	if err != nil {
		return nil, fmt.Errorf("MONGODB_READ_PREFERENCE không hợp lệ: %w", err)
	}
	rp, err := readpref.New(mode)
	if err != nil {
		return nil, err
	}

	opts := options.Client().
		ApplyURI(c.URI).
		SetReadPreference(rp).
		SetConnectTimeout(c.ConnectTimeout).
		SetServerSelectionTimeout(c.ConnectTimeout).
		SetAppName("am-shortlink-portal-api")
	if tracing {
		// Không ghi nội dung câu lệnh (có thể chứa SĐT / IP) vào span.
		opts.SetMonitor(otelmongo.NewMonitor(otelmongo.WithCommandAttributeDisabled(true)))
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("kết nối MongoDB: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, c.ConnectTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, rp); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}

	return &DB{
		Client:  client,
		Core:    client.Database(c.CoreDB),
		Report:  client.Database(c.ReportDB),
		MaxTime: c.MaxTime,
	}, nil
}

// Ping dùng cho /readyz.
func (d *DB) Ping(ctx context.Context) error {
	return d.Client.Ping(ctx, readpref.Nearest())
}

func (d *DB) Close(ctx context.Context) error { return d.Client.Disconnect(ctx) }
