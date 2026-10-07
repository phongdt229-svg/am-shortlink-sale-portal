// Package config nạp cấu hình portal-api từ biến môi trường.
//
// Secret có thể truyền qua file (Vault Agent): đặt <TÊN>_FILE=/đường/dẫn — có cả TÊN và TÊN_FILE thì ưu tiên file.
// Thiếu biến bắt buộc → dừng khởi động và báo TÊN biến (không bao giờ in giá trị).
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Env      string `envconfig:"APP_ENV" default:"local"` // local | dev | staging | production
	HTTPAddr string `envconfig:"HTTP_ADDR" default:":8080"`
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`

	Mongo Mongo
	Redis Redis
	Auth  Auth
	Kafka KafkaLog
	OTel  OTel

	// PIIHashSalt: salt SHA-256 cho tham số pii=hash và khoá mã hoá tham chiếu CTV (ctv_ref).
	PIIHashSalt string `envconfig:"PII_HASH_SALT" required:"true"`

	// ShortURLBase: domain dựng short URL <base>/<prefix>/<code> (hiển thị, QR).
	ShortURLBase string `envconfig:"SHORT_URL_BASE" default:"http://localhost"`

	// Export
	ExportDir    string        `envconfig:"EXPORT_DIR" default:"./var/exports"`
	ExportWorker bool          `envconfig:"EXPORT_WORKER" default:"true"` // false = pod chỉ phục vụ API (worker chạy deployment riêng)
	ExportTTL    time.Duration `envconfig:"EXPORT_TTL" default:"168h"`
}

type Mongo struct {
	URI            string        `envconfig:"MONGODB_URI" required:"true"`
	CoreDB         string        `envconfig:"MONGODB_DB_CORE" default:"am_shortlink"`
	ReportDB       string        `envconfig:"MONGODB_DB_REPORT" default:"am_shortlink_report"`
	ReadPreference string        `envconfig:"MONGODB_READ_PREFERENCE" default:"secondaryPreferred"`
	MaxTime        time.Duration `envconfig:"MONGODB_MAX_TIME" default:"10s"`
	ConnectTimeout time.Duration `envconfig:"MONGODB_CONNECT_TIMEOUT" default:"10s"`
}

// Redis: cache báo cáo + rate limit đăng nhập. Redis RIÊNG của Portal; REDIS_ADDR rỗng = tắt cache.
type Redis struct {
	Addr      string        `envconfig:"REDIS_ADDR"`
	Password  string        `envconfig:"REDIS_PASSWORD"`
	DB        int           `envconfig:"REDIS_DB" default:"0"`
	KeyPrefix string        `envconfig:"REDIS_KEY_PREFIX" default:"am-shortlink-portal:"`
	Timeout   time.Duration `envconfig:"REDIS_TIMEOUT" default:"150ms"`
	// ReportTTL: kỳ có hôm nay (số liệu còn chạy, trễ ≤ 1 phút); ReportTTLPast: kỳ đã kết thúc.
	ReportTTL     time.Duration `envconfig:"REPORT_CACHE_TTL" default:"60s"`
	ReportTTLPast time.Duration `envconfig:"REPORT_CACHE_TTL_PAST" default:"15m"`
}

type Auth struct {
	JWTSigningKey   string        `envconfig:"PORTAL_JWT_SIGNING_KEY" required:"true"`
	JWTIssuer       string        `envconfig:"PORTAL_JWT_ISSUER" default:"am-shortlink-portal-api"`
	AccessTTL       time.Duration `envconfig:"PORTAL_ACCESS_TTL" default:"15m"`
	RefreshTTL      time.Duration `envconfig:"PORTAL_REFRESH_TTL" default:"8h"`
	MaxFailedLogins int           `envconfig:"PORTAL_MAX_FAILED_LOGINS" default:"5"`
	LockDuration    time.Duration `envconfig:"PORTAL_LOCK_DURATION" default:"15m"`
	// LoginRatePerMin: số lần gọi /auth/login tối đa mỗi IP mỗi phút.
	LoginRatePerMin int `envconfig:"PORTAL_LOGIN_RATE_PER_MIN" default:"20"`
}

type KafkaLog struct {
	Brokers []string `envconfig:"KAFKA_LOG_BROKERS"`
	Topic   string   `envconfig:"KAFKA_LOG_TOPIC"`
}

// OTel giữ đúng bộ biến quy ước của fptvn-web (docs/ENV_OBSERVABILITY_PLAN.md §4).
type OTel struct {
	Enabled          bool    `envconfig:"OTEL_ENABLED" default:"false"`
	ServiceNamespace string  `envconfig:"OTEL_SERVICE_NAMESPACE" default:"am-shortlink"`
	ServiceName      string  `envconfig:"OTEL_SERVICE_NAME" default:"am-shortlink-portal-api"`
	EndpointTraces   string  `envconfig:"OTEL_ENDPOINT_TRACES"`
	Sampler          string  `envconfig:"OTEL_SAMPLER" default:"always"` // always | ratio
	SamplerRatio     float64 `envconfig:"OTEL_SAMPLER_RATIO" default:"0.05"`
}

// secretVars có thể đọc từ <TÊN>_FILE.
var secretVars = []string{"MONGODB_URI", "PORTAL_JWT_SIGNING_KEY", "PII_HASH_SALT", "REDIS_PASSWORD"}

// Load đọc cấu hình; trả lỗi chỉ chứa tên biến.
func Load() (*Config, error) {
	if err := loadFileSecrets(os.Getenv, os.ReadFile, os.Setenv); err != nil {
		return nil, err
	}
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, sanitize(err)
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) validate() error {
	var errs []error
	// envconfig coi biến đặt rỗng là "đã có" → tự kiểm.
	for name, v := range map[string]string{
		"MONGODB_URI": c.Mongo.URI, "PORTAL_JWT_SIGNING_KEY": c.Auth.JWTSigningKey, "PII_HASH_SALT": c.PIIHashSalt,
	} {
		if strings.TrimSpace(v) == "" {
			errs = append(errs, errors.New("thiếu biến bắt buộc "+name))
		}
	}
	if errs != nil {
		return errors.Join(errs...)
	}
	if len(c.Auth.JWTSigningKey) < 32 {
		errs = append(errs, errors.New("PORTAL_JWT_SIGNING_KEY phải dài ≥ 32 ký tự"))
	}
	if len(c.PIIHashSalt) < 16 {
		errs = append(errs, errors.New("PII_HASH_SALT phải dài ≥ 16 ký tự"))
	}
	if c.Auth.AccessTTL <= 0 || c.Auth.RefreshTTL <= c.Auth.AccessTTL {
		errs = append(errs, errors.New("PORTAL_REFRESH_TTL phải lớn hơn PORTAL_ACCESS_TTL > 0"))
	}
	switch c.OTel.Sampler {
	case "always", "ratio":
	default:
		errs = append(errs, errors.New("OTEL_SAMPLER chỉ nhận always | ratio"))
	}
	if c.OTel.Enabled && c.OTel.EndpointTraces == "" {
		errs = append(errs, errors.New("OTEL_ENDPOINT_TRACES bắt buộc khi OTEL_ENABLED=true"))
	}
	if len(c.Kafka.Brokers) > 0 && c.Kafka.Topic == "" {
		errs = append(errs, errors.New("KAFKA_LOG_TOPIC bắt buộc khi có KAFKA_LOG_BROKERS"))
	}
	return errors.Join(errs...)
}

// IsProduction: dùng để tắt các tiện ích dev (seed, chi tiết lỗi).
func (c *Config) IsProduction() bool { return c.Env == "production" }

// LoadedSecrets trả về tên các secret đã có giá trị (để log khi khởi động, không in giá trị).
func LoadedSecrets() []string {
	var names []string
	for _, n := range secretVars {
		if os.Getenv(n) != "" {
			names = append(names, n)
		}
	}
	return names
}

func loadFileSecrets(getenv func(string) string, readFile func(string) ([]byte, error), setenv func(string, string) error) error {
	for _, name := range secretVars {
		path := getenv(name + "_FILE")
		if path == "" {
			continue
		}
		b, err := readFile(path)
		if err != nil {
			return fmt.Errorf("không đọc được %s_FILE: %w", name, err)
		}
		if err := setenv(name, strings.TrimSpace(string(b))); err != nil {
			return err
		}
	}
	return nil
}

// sanitize bỏ giá trị khỏi lỗi envconfig (lỗi parse có thể chứa giá trị biến).
func sanitize(err error) error {
	var pe *envconfig.ParseError
	if errors.As(err, &pe) {
		return fmt.Errorf("biến %s sai định dạng (%s)", pe.KeyName, pe.TypeName)
	}
	return err
}
