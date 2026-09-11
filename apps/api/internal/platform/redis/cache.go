package redis

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// KeyPrefix đứng đầu mọi key. Đổi cấu trúc dữ liệu cache thì tăng v1 lên v2 —
// toàn bộ cache cũ tự hết hiệu lực mà không cần quét xóa.
const KeyPrefix = "bec:v1:"

type Cache struct {
	rdb *goredis.Client
	sf  singleflight.Group
	log *slog.Logger
}

func NewCache(rdb *goredis.Client, log *slog.Logger) *Cache {
	return &Cache{rdb: rdb, log: log}
}

// GetOrLoad đọc cache, trượt thì gọi load rồi ghi lại.
//
// Hai chi tiết chống sập hệ thống:
//   - singleflight: 1000 request cùng trượt một key chỉ gọi load MỘT lần.
//     Không có nó, một sản phẩm hot hết hạn cache sẽ kéo cả nghìn truy vấn
//     xuống Postgres cùng lúc.
//   - jitter TTL: 10.000 key ghi cùng lúc sẽ hết hạn cùng lúc. Cộng ngẫu
//     nhiên 0–20% để rải ra.
//
// Mọi lỗi Redis đều bị nuốt: cache hỏng thì đọc thẳng nguồn, không bao giờ
// trả lỗi cho người dùng vì cache.
func GetOrLoad[T any](
	ctx context.Context, c *Cache, key string, ttl time.Duration,
	load func(context.Context) (T, error),
) (T, error) {
	var zero T
	full := KeyPrefix + key

	if b, err := c.rdb.Get(ctx, full).Bytes(); err == nil {
		var v T
		if json.Unmarshal(b, &v) == nil {
			return v, nil
		}
		// Dữ liệu hỏng — bỏ đi và đọc lại từ nguồn.
		c.rdb.Del(ctx, full)
	}

	res, err, _ := c.sf.Do(full, func() (any, error) {
		v, err := load(ctx)
		if err != nil {
			return nil, err
		}
		if b, mErr := json.Marshal(v); mErr == nil {
			if sErr := c.rdb.Set(ctx, full, b, jitter(ttl)).Err(); sErr != nil {
				c.log.WarnContext(ctx, "không ghi được cache", "key", full, "err", sErr)
			}
		}
		return v, nil
	})
	if err != nil {
		return zero, err
	}

	v, ok := res.(T)
	if !ok {
		return zero, nil
	}
	return v, nil
}

// Delete xóa key. Dùng DEL chứ KHÔNG ghi đè giá trị mới: ghi đè trong môi
// trường nhiều tiến trình dễ tạo tình huống giá trị cũ ghi sau giá trị mới.
func (c *Cache) Delete(ctx context.Context, keys ...string) {
	if len(keys) == 0 {
		return
	}
	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = KeyPrefix + k
	}
	if err := c.rdb.Del(ctx, full...).Err(); err != nil {
		c.log.WarnContext(ctx, "không xóa được cache", "keys", full, "err", err)
	}
}

func jitter(d time.Duration) time.Duration {
	return d + time.Duration(rand.Int63n(int64(d/5)+1))
}
