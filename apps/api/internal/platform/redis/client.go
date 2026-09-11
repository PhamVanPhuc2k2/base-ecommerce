// Package redis bọc client Redis dùng chung.
//
// Nguyên tắc: Redis chết thì hệ thống CHẬM ĐI, không được SẬP. Mọi lỗi Redis
// đều bị nuốt ở tầng cache và rơi xuống đọc thẳng Postgres. Timeout đặt rất
// ngắn để Redis treo không kéo theo cả API.
package redis

import (
	"context"
	"log/slog"
	"time"

	"base-ecommerce/api/internal/platform/config"

	goredis "github.com/redis/go-redis/v9"
)

// NewClient dựng client Redis. KHÔNG trả lỗi khi Redis không kết nối được —
// chỉ ghi WARN.
//
// Bản trước fail hẳn lúc khởi động, và điều đó mâu thuẫn với chính nguyên tắc
// ghi ở đầu package này. Hậu quả thật: Redis sập kéo theo `/readyz` trả 503,
// load balancer rút hết pod ra, rồi không pod nào khởi động lại được nữa vì
// Redis vẫn chưa lên. Một sự cố cache biến thành sự cố toàn hệ thống.
//
// Mọi đường đi request đều đã chịu được Redis chết (lỗi bị nuốt ở tầng cache),
// nên khởi động được cũng phải chịu được.
func NewClient(ctx context.Context, cfg config.Redis, log *slog.Logger) *goredis.Client {
	c := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,

		// MaxRetries: -1 nghĩa là TẮT retry, không phải retry vô hạn.
		//
		// Với MaxRetries: 1, một lệnh khi Redis từ chối kết nối mất ~824ms:
		// pool tự dial lại 5 lần rồi go-redis nhân đôi lên vì retry. DialTimeout
		// 200ms không giới hạn tổng thời gian — nó chỉ giới hạn MỘT lần dial.
		// Một request GET sản phẩm vì thế đi từ 2ms lên 1.66s, tức chậm 700 lần,
		// và ở vài chục RPS là goroutine tồn đọng đủ làm sập API vì tắc nghẽn
		// chứ không phải vì lỗi.
		MaxRetries: -1,
	})

	if err := c.Ping(ctx).Err(); err != nil {
		log.Warn("không ping được redis lúc khởi động — chạy tiếp không có cache",
			"addr", cfg.Addr, "err", err)
	}
	return c
}

// HealthChecker cài đặt health.Checker cho Redis.
type HealthChecker struct{ c *goredis.Client }

func NewHealthChecker(c *goredis.Client) *HealthChecker { return &HealthChecker{c: c} }

func (h *HealthChecker) Name() string { return "redis" }

// Optional: Redis chết thì API vẫn phục vụ đúng, chỉ chậm hơn. Xem health.Optional.
func (h *HealthChecker) Optional() bool { return true }

func (h *HealthChecker) Check(ctx context.Context) error { return h.c.Ping(ctx).Err() }
