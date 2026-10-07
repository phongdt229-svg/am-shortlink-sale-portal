// Package redis tạo client Redis cho cache báo cáo + rate limit của Portal.
// Redis RIÊNG của Portal (không dùng Redis nghiệp vụ của Service). REDIS_ADDR rỗng → tắt (nil client).
package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	goredis "github.com/redis/go-redis/v9"

	"am-shortlink-portal/api/internal/config"
)

// Connect trả nil khi không cấu hình. Không ping lỗi → vẫn trả client (fail-open: cache tự bỏ qua khi Redis chết).
func Connect(ctx context.Context, c config.Redis, tracing bool) (*goredis.Client, error) {
	if c.Addr == "" {
		return nil, nil
	}
	cl := goredis.NewClient(&goredis.Options{
		Addr:         c.Addr,
		Password:     c.Password,
		DB:           c.DB,
		DialTimeout:  500 * time.Millisecond,
		MaxRetries:   -1, // cache: không retry, lỗi → bỏ qua
		ReadTimeout:  c.Timeout,
		WriteTimeout: c.Timeout,
		PoolSize:     20,
		MinIdleConns: 2,
	})
	if tracing {
		if err := redisotel.InstrumentTracing(cl); err != nil {
			return nil, err
		}
	}
	pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = cl.Ping(pctx).Err() // chỉ để làm ấm kết nối; lỗi không chặn khởi động
	return cl, nil
}
