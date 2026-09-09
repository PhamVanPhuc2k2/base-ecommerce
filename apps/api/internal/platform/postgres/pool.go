package postgres

import (
	"context"
	"fmt"
	"time"

	"base-ecommerce/api/internal/platform/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool tạo connection pool và ping ngay để lỗi cấu hình lộ ra lúc khởi động
// chứ không phải lúc có request đầu tiên.
//
// Lưu ý về kích thước pool: pool to hơn KHÔNG nhanh hơn. Mỗi kết nối Postgres là
// một process riêng; vượt quá số core của máy database thì thông lượng giảm.
// Tổng (pool × số bản chạy) của mọi tiến trình phải nhỏ hơn max_connections.
func NewPool(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("phân tích DSN: %w", err)
	}

	pc.MaxConns = cfg.MaxConns
	pc.MinConns = cfg.MinConns
	pc.MaxConnLifetime = cfg.MaxConnLifetime
	pc.MaxConnIdleTime = 30 * time.Minute
	pc.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("tạo pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
