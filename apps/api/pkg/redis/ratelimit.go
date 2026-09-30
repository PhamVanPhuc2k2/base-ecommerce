package redis

import (
	"context"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// fixedWindow: INCR rồi đặt hạn ở lần đầu, trong MỘT lệnh Lua — nguyên tử và
// một vòng mạng. Tách INCR và EXPIRE thành hai lệnh thì tiến trình chết giữa
// chừng để lại khóa KHÔNG hạn: bộ đếm tăng mãi, người đó bị chặn vĩnh viễn.
// Trả về: số lần đã đếm và số mili-giây còn lại của cửa sổ.
var fixedWindow = goredis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return {n, redis.call('PTTL', KEYS[1])}
`)

// RateLimiter đếm theo cửa sổ cố định (thiết kế 03 mục 7).
type RateLimiter struct {
	rdb *goredis.Client
	log *slog.Logger
}

func NewRateLimiter(rdb *goredis.Client, log *slog.Logger) *RateLimiter {
	return &RateLimiter{rdb: rdb, log: log}
}

// Allow trả ok=false khi đã vượt limit trong cửa sổ, kèm thời gian phải chờ.
//
// Redis lỗi → CHO QUA (fail-open, thiết kế 03): chặn hết khách vì bộ đếm hỏng
// tệ hơn để lọt vài request. Ghi WARN để thấy lớp phòng thủ đang tắt.
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration) {
	ctx, cancel := withOpTimeout(ctx)
	defer cancel()
	res, err := fixedWindow.Run(ctx, r.rdb, []string{KeyPrefix + "rl:" + key}, window.Milliseconds()).Int64Slice()
	if err != nil || len(res) != 2 {
		r.log.WarnContext(ctx, "rate limit không đếm được, cho qua", "err", err)
		return true, 0
	}
	if res[0] > int64(limit) {
		wait := time.Duration(res[1]) * time.Millisecond
		if wait <= 0 {
			wait = window
		}
		return false, wait
	}
	return true, 0
}
