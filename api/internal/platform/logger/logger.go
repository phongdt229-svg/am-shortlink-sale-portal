// Package logger dựng slog JSON → stdout (+ Kafka nếu cấu hình).
//
// Không bao giờ ghi mật khẩu, token, cookie, Authorization; SĐT và IP được che.
// Ghi Kafka không chặn request: buffer đầy → bỏ bản ghi, đếm vào Dropped().
package logger

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"am-shortlink-portal/api/internal/mask"
)

type Options struct {
	Level       string
	Service     string
	Environment string
	Brokers     []string
	Topic       string
}

// Logger kèm hàm Close để flush Kafka khi tắt.
type Logger struct {
	*slog.Logger
	kafka *kafkaWriter
}

func New(o Options) (*Logger, error) {
	level := parseLevel(o.Level)
	hopts := &slog.HandlerOptions{Level: level, ReplaceAttr: redact}

	var w io.Writer = os.Stdout
	var kw *kafkaWriter
	if len(o.Brokers) > 0 {
		var err error
		kw, err = newKafkaWriter(o)
		if err != nil {
			return nil, err
		}
		w = io.MultiWriter(os.Stdout, kw)
	}
	l := slog.New(slog.NewJSONHandler(w, hopts)).With(
		slog.String("service", o.Service),
		slog.String("env", o.Environment),
	)
	return &Logger{Logger: l, kafka: kw}, nil
}

// Close flush Kafka tối đa 2s.
func (l *Logger) Close() {
	if l.kafka != nil {
		l.kafka.close()
	}
}

// Dropped: số bản ghi log Kafka bị bỏ do buffer đầy / lỗi gửi.
func (l *Logger) Dropped() int64 {
	if l.kafka == nil {
		return 0
	}
	return l.kafka.dropped.Load()
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

var secretKeys = map[string]bool{
	"password": true, "password_hash": true, "token": true, "access_token": true,
	"refresh_token": true, "authorization": true, "cookie": true, "set-cookie": true,
	"key": true, "api_key": true, "secret": true,
}

func redact(_ []string, a slog.Attr) slog.Attr {
	k := strings.ToLower(a.Key)
	switch {
	case secretKeys[k]:
		return slog.String(a.Key, "[REDACTED]")
	case k == "ip" || k == "client_ip" || k == "remote_ip":
		return slog.String(a.Key, mask.IP(a.Value.String()))
	case k == "phone" || k == "ctv_id":
		return slog.String(a.Key, mask.Phone(a.Value.String()))
	}
	return a
}

// kafkaWriter nhận từng dòng JSON từ slog và đẩy async theo định dạng chung của shortlink:
// {log_type, environment, service, data}.
type kafkaWriter struct {
	client  *kgo.Client
	topic   string
	env     string
	service string
	dropped atomic.Int64
}

func newKafkaWriter(o Options) (*kafkaWriter, error) {
	c, err := kgo.NewClient(
		kgo.SeedBrokers(o.Brokers...),
		kgo.DefaultProduceTopic(o.Topic),
		kgo.MaxBufferedRecords(10_000),
		kgo.ProducerLinger(200*time.Millisecond),
		kgo.RequiredAcks(kgo.LeaderAck()),
		kgo.DisableIdempotentWrite(),
	)
	if err != nil {
		return nil, err
	}
	return &kafkaWriter{client: c, topic: o.Topic, env: o.Environment, service: o.Service}, nil
}

func (k *kafkaWriter) Write(p []byte) (int, error) {
	data := make([]byte, len(p))
	copy(data, p)
	msg, err := json.Marshal(map[string]any{
		"log_type":    "logs",
		"environment": k.env,
		"service":     k.service,
		"data":        json.RawMessage(trimNewline(data)),
	})
	if err != nil {
		k.dropped.Add(1)
		return len(p), nil
	}
	k.client.TryProduce(context.Background(), &kgo.Record{Value: msg}, func(_ *kgo.Record, err error) {
		if err != nil {
			k.dropped.Add(1)
		}
	})
	return len(p), nil
}

func (k *kafkaWriter) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = k.client.Flush(ctx)
	k.client.Close()
}

func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
