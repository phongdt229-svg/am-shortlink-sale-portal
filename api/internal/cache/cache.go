// Package cache: cache kết quả báo cáo (JSON) trên Redis.
//
//   - Fail-open: Redis lỗi / chậm → tính trực tiếp, không trả lỗi cho người dùng.
//   - Singleflight: nhiều request cùng khoá trong một pod chỉ chạy truy vấn Mongo 1 lần.
//   - Khoá gồm PHẠM VI ĐÃ ÁP QUYỀN (không phải username) → không bao giờ trả dữ liệu ngoài phạm vi.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/singleflight"
)

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration)
	// Del xoá theo khoá chính xác (vd sau khi admin đổi param_registry).
	Del(ctx context.Context, keys ...string)
}

// ---- Redis ----

type Redis struct {
	c       *goredis.Client
	prefix  string
	timeout time.Duration // mỗi lệnh; quá → coi như miss (Redis treo không kéo chậm báo cáo)
}

func NewRedis(c *goredis.Client, prefix string, timeout time.Duration) *Redis {
	return &Redis{c: c, prefix: prefix, timeout: timeout}
}

func (r *Redis) Get(ctx context.Context, key string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	b, err := r.c.Get(ctx, r.prefix+key).Bytes()
	if err != nil {
		if !errors.Is(err, goredis.Nil) {
			countErr(ctx, "get")
			slog.WarnContext(ctx, "cache get failed", "err", err)
		}
		return nil, false
	}
	return b, true
}

func (r *Redis) Set(ctx context.Context, key string, val []byte, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	// SET key val EX ttl — một lệnh, atomic (hệ PHP cũ dùng SET + EXPIRE rời nhau).
	if err := r.c.Set(ctx, r.prefix+key, val, ttl).Err(); err != nil {
		countErr(ctx, "set")
		slog.WarnContext(ctx, "cache set failed", "err", err)
	}
}

func (r *Redis) Del(ctx context.Context, keys ...string) {
	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = r.prefix + k
	}
	if err := r.c.Del(ctx, full...).Err(); err != nil {
		countErr(ctx, "del")
	}
}

// ---- Noop (không cấu hình Redis) ----

type Noop struct{}

func (Noop) Get(context.Context, string) ([]byte, bool)         { return nil, false }
func (Noop) Set(context.Context, string, []byte, time.Duration) {}
func (Noop) Del(context.Context, ...string)                     {}

// ---- helper ----

var (
	group   singleflight.Group
	meter   = otel.Meter("am-shortlink-portal/cache")
	lookups metric.Int64Counter
	errs    metric.Int64Counter
)

func init() {
	lookups, _ = meter.Int64Counter("portal_report_cache_lookups_total", metric.WithDescription("Số lần tra cache báo cáo theo kết quả (hit/miss)"))
	errs, _ = meter.Int64Counter("portal_report_cache_errors_total", metric.WithDescription("Lỗi Redis (đã bỏ qua — fail-open)"))
}

func countErr(ctx context.Context, op string) {
	errs.Add(ctx, 1, metric.WithAttributes(attribute.String("op", op)))
}

// Key: name + SHA-256 của JSON các thành phần (thứ tự cố định).
func Key(name string, parts ...any) string {
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(b)
	return name + ":" + hex.EncodeToString(sum[:16])
}

// Do trả giá trị trong cache hoặc tính bằng fn rồi lưu. ttl <= 0 → không cache.
func Do[T any](ctx context.Context, c Cache, name, key string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if c == nil || ttl <= 0 {
		return fn(ctx)
	}
	if b, ok := c.Get(ctx, key); ok {
		var v T
		if err := json.Unmarshal(b, &v); err == nil {
			lookups.Add(ctx, 1, metric.WithAttributes(attribute.String("report", name), attribute.String("result", "hit")))
			return v, nil
		}
	}
	lookups.Add(ctx, 1, metric.WithAttributes(attribute.String("report", name), attribute.String("result", "miss")))

	// Singleflight theo khoá; dùng context không huỷ để request đầu bị huỷ không làm hỏng các request đang chờ,
	// nhưng vẫn giới hạn thời gian bằng deadline của request đầu tiên (MONGODB_MAX_TIME áp ở store).
	res, err, _ := group.Do(key, func() (any, error) {
		v, err := fn(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		if b, err := json.Marshal(v); err == nil {
			c.Set(context.WithoutCancel(ctx), key, b, ttl)
		}
		return v, nil
	})
	if err != nil {
		return zero, err
	}
	return res.(T), nil
}
