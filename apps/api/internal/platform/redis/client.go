// Package redis bọc client Redis dùng chung.
//
// Nguyên tắc: Redis chết thì hệ thống CHẬM ĐI, không được SẬP. Mọi lỗi Redis
// đều bị nuốt ở tầng cache và rơi xuống đọc thẳng Postgres. Timeout đặt rất
// ngắn để Redis treo không kéo theo cả API.
package redis

import (
	"context"
	"fmt"
	"time"

	"base-ecommerce/api/internal/platform/config"

	goredis "github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, cfg config.Redis) (*goredis.Client, error) {
	c := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
		MaxRetries:   1,
	})

	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return c, nil
}

// HealthChecker cài đặt health.Checker cho Redis.
type HealthChecker struct{ c *goredis.Client }

func NewHealthChecker(c *goredis.Client) *HealthChecker { return &HealthChecker{c: c} }

func (h *HealthChecker) Name() string { return "redis" }

func (h *HealthChecker) Check(ctx context.Context) error { return h.c.Ping(ctx).Err() }
