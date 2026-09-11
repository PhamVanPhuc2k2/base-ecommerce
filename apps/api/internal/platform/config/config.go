// Package config đọc cấu hình từ biến môi trường và validate ngay lúc khởi động.
//
// Nguyên tắc: thiếu hoặc sai biến thì tiến trình thoát ngay với thông báo rõ ràng,
// thay vì chạy được rồi mới hỏng lúc có request đầu tiên chạm tới.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string
	Version  string
	LogLevel string
	AdminKey string
	HTTP     HTTP
	DB       DB
	Redis    Redis
	RabbitMQ RabbitMQ
}

type HTTP struct {
	Addr string
	// HandlerTimeout là trần thời gian cho một handler nghiệp vụ.
	HandlerTimeout time.Duration
	// WriteTimeout là trần thời gian net/http cho phép ghi response. PHẢI lớn
	// hơn HandlerTimeout — xem phần kiểm tra trong Load.
	WriteTimeout    time.Duration
	ReadTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type DB struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

type Redis struct {
	Addr     string
	PoolSize int
}

type RabbitMQ struct {
	// URL là AMQP URI đầy đủ, KÈM mật khẩu. Mọi chỗ in nó ra phải đi qua
	// redactDSN — xem Config.String.
	URL string
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// String che các giá trị nhạy cảm để an toàn khi ghi log toàn bộ config.
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{Env:%s Version:%s HTTP.Addr:%s DB.DSN:%s DB.MaxConns:%d Redis.Addr:%s "+
			"RabbitMQ.URL:%s AdminKey:%s}",
		c.Env, c.Version, c.HTTP.Addr, redactDSN(c.DB.DSN), c.DB.MaxConns,
		c.Redis.Addr, redactDSN(c.RabbitMQ.URL), redactSecret(c.AdminKey),
	)
}

// redactDSN thay mật khẩu bằng *** trong mọi URI dạng scheme://user:pass@host,
// nên dùng được cho cả DATABASE_URL lẫn RABBITMQ_URL.
func redactDSN(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	scheme := strings.Index(dsn, "://")
	if at < 0 || scheme < 0 || at < scheme {
		return dsn
	}
	creds := dsn[scheme+3 : at]
	if colon := strings.Index(creds, ":"); colon >= 0 {
		return dsn[:scheme+3] + creds[:colon] + ":***" + dsn[at:]
	}
	return dsn
}

// redactSecret chỉ để lại dấu vết đủ để biết đã nạp đúng biến hay chưa.
func redactSecret(s string) string {
	if s == "" {
		return "(rỗng)"
	}
	return fmt.Sprintf("(đã đặt, %d ký tự)", len(s))
}

func Load() (*Config, error) {
	l := &loader{}

	c := &Config{
		Env:      l.str("APP_ENV", "development"),
		Version:  l.str("APP_VERSION", "dev"),
		LogLevel: l.str("LOG_LEVEL", "info"),
		HTTP: HTTP{
			Addr:            l.str("HTTP_ADDR", ":8080"),
			ReadTimeout:     l.dur("HTTP_READ_TIMEOUT", 15*time.Second),
			HandlerTimeout:  l.dur("HTTP_HANDLER_TIMEOUT", 30*time.Second),
			WriteTimeout:    l.dur("HTTP_WRITE_TIMEOUT", 35*time.Second),
			ShutdownTimeout: l.dur("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		DB: DB{
			DSN:             l.required("DATABASE_URL"),
			MaxConns:        int32(l.num("DB_MAX_CONNS", 20)),
			MinConns:        int32(l.num("DB_MIN_CONNS", 2)),
			MaxConnLifetime: l.dur("DB_MAX_CONN_LIFETIME", time.Hour),
		},
		// Khóa tạm bảo vệ API ghi cho tới khi P2 có JWT + RBAC.
		// BẮT BUỘC: thiếu thì server không khởi động, nên không thể vô tình
		// deploy một API ghi không ai bảo vệ.
		AdminKey: l.required("ADMIN_API_KEY"),
		Redis: Redis{
			Addr:     l.str("REDIS_ADDR", "localhost:6380"),
			PoolSize: l.num("REDIS_POOL_SIZE", 20),
		},
		RabbitMQ: RabbitMQ{
			// Không dùng required: cmd/api chạy được mà không cần broker (event
			// chỉ được ghi xuống bảng outbox, relay đẩy sau), nên bắt buộc biến
			// này sẽ chặn khởi động vì một phụ thuộc không chặn đường request.
			// Dấu "/" cuối là vhost mặc định, bỏ đi là URI không hợp lệ.
			URL: l.str("RABBITMQ_URL", "amqp://app:app@localhost:5672/"),
		},
	}

	// WriteTimeout phải lớn hơn HandlerTimeout, nếu không response lỗi không
	// bao giờ tới được client.
	//
	// Đây là lỗi đã đo được: hai mốc cùng là 30s, handler hết giờ, httpx ghi
	// problem+json — nhưng write deadline của net/http hết đúng khoảnh khắc đó
	// nên kết nối bị đóng trước khi flush. Client nhận "Empty reply from
	// server" thay vì mã lỗi, đúng lúc hệ thống quá tải và cần chẩn đoán nhất.
	if c.HTTP.WriteTimeout <= c.HTTP.HandlerTimeout {
		l.errs = append(l.errs, fmt.Errorf(
			"HTTP_WRITE_TIMEOUT (%s) phải lớn hơn HTTP_HANDLER_TIMEOUT (%s), "+
				"nếu không response lỗi lúc hết giờ không kịp ghi ra cho client",
			c.HTTP.WriteTimeout, c.HTTP.HandlerTimeout))
	}

	if err := l.err(); err != nil {
		return nil, err
	}
	return c, nil
}

// loader gom lỗi lại thay vì dừng ở lỗi đầu tiên, để một lần chạy báo hết
// mọi biến sai — sửa một lượt thay vì sửa từng cái.
type loader struct{ errs []error }

func (l *loader) str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (l *loader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		l.errs = append(l.errs, fmt.Errorf("thiếu biến môi trường bắt buộc %s", key))
	}
	return v
}

func (l *loader) num(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s phải là số nguyên, nhận được %q", key, raw))
		return def
	}
	return v
}

func (l *loader) dur(key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s phải là khoảng thời gian (ví dụ 15s, 1h), nhận được %q", key, raw))
		return def
	}
	return v
}

func (l *loader) err() error {
	if len(l.errs) == 0 {
		return nil
	}
	return fmt.Errorf("cấu hình không hợp lệ: %w", errors.Join(l.errs...))
}
