package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"reflect"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// KeyPrefix đứng đầu mọi key. Đổi cấu trúc dữ liệu cache thì tăng v1 lên v2 —
// toàn bộ cache cũ tự hết hiệu lực mà không cần quét xóa.
const KeyPrefix = "bec:v1:"

// loadTimeout chặn trên thời gian nạp lại từ nguồn khi cache trượt. Cần vì
// context nạp đã tách khỏi context request (xem GetOrLoad), nên không còn ai
// hủy nó hộ nữa.
const loadTimeout = 10 * time.Second

// opTimeout là trần thời gian cho MỘT lệnh Redis, tính cả lúc Redis chết.
//
// DialTimeout/ReadTimeout của go-redis không làm được việc này: chúng giới hạn
// từng lần dial, còn pool thì tự dial lại 5 lần. Đo thật khi Redis từ chối kết
// nối: một lệnh mất ~820ms dù DialTimeout chỉ 200ms. Cache là thứ "có thì
// nhanh hơn" — nó không được phép tiêu quá chừng này thời gian của request.
const opTimeout = 60 * time.Millisecond

// withOpTimeout gắn trần opTimeout, và tách khỏi việc hủy của request cha chỉ
// khi ctx cha đã hết hạn thì thôi (không cần WithoutCancel ở đây: lệnh cache
// bị hủy theo request là đúng).
func withOpTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, opTimeout)
}

// Validator cho phép giá trị đọc từ cache tự kiểm tra bất biến của mình.
// Kiểu nào không cài đặt thì bỏ qua, nên đây là tùy chọn.
//
// Vì sao cần: json.Unmarshal chỉ báo lỗi khi cú pháp sai. Nó nhận `{}` và cho
// ra một struct toàn giá trị zero — ID toàn số 0, giá 0, status rỗng — mà
// không một lỗi nào. API sẽ trả 200 kèm sản phẩm bịa, giá 0, suốt cả TTL.
// UnmarshalJSON của Money cũng không cứu được: nó chỉ chạy khi trường Price
// CÓ MẶT trong JSON.
type Validator interface {
	Validate() error
}

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

	getCtx, cancelGet := withOpTimeout(ctx)
	b, getErr := c.rdb.Get(getCtx, full).Bytes()
	cancelGet()
	if getErr == nil {
		var v T
		if reason := decodeCached(b, &v); reason != nil {
			// Nội dung cache không dùng được — bỏ đi và đọc lại từ nguồn.
			//
			// Ghi WARN chứ không im lặng: một giá trị luôn bị từ chối sẽ khiến
			// mỗi request đều Get → Del → load → Set lại đúng giá trị hỏng đó,
			// tỉ lệ trúng cache 0% mà nhìn từ ngoài chỉ thấy database bị hỏi
			// nhiều bất thường.
			c.log.WarnContext(ctx, "bỏ qua nội dung cache không hợp lệ",
				"key", full, "lý_do", reason.Error())
			delCtx, cancelDel := withOpTimeout(ctx)
			c.rdb.Del(delCtx, full)
			cancelDel()
		} else {
			return v, nil
		}
	}

	res, err, _ := c.sf.Do(full, func() (any, error) {
		// WithoutCancel: sf.Do chỉ truyền ctx của người tới TRƯỚC vào load.
		// Người đó đóng tab thì ctx hủy, và TẤT CẢ những người đang đợi cùng
		// nhận context.Canceled — vốn không map được sang mã lỗi nào nên ra
		// 500. Đúng lúc một sản phẩm hot vừa hết hạn cache thì một request bị
		// hủy làm hỏng lượt xem của mọi người còn lại.
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), loadTimeout)
		defer cancel()

		v, err := load(loadCtx)
		if err != nil {
			return nil, err
		}
		// ttl <= 0: Set của go-redis hiểu 0 là "không đặt TTL" — khóa sống
		// vĩnh viễn. Người viết code sau này rất dễ tưởng 0 nghĩa là đừng cache.
		if ttl > 0 {
			if raw, mErr := json.Marshal(v); mErr == nil {
				setCtx, cancelSet := withOpTimeout(loadCtx)
				sErr := c.rdb.Set(setCtx, full, raw, jitter(ttl)).Err()
				cancelSet()
				if sErr != nil {
					c.log.WarnContext(ctx, "không ghi được cache", "key", full, "err", sErr)
				}
			}
		}
		return v, nil
	})
	if err != nil {
		return zero, err
	}

	v, ok := res.(T)
	if !ok {
		// Không bao giờ xảy ra trừ khi hai lời gọi cùng key dùng hai kiểu T
		// khác nhau. Trả (zero, nil) như trước là nói dối: người gọi nhận con
		// trỏ nil kèm err nil rồi deref nó.
		return zero, fmt.Errorf("cache: khóa %s trả về kiểu %T, mong đợi %T", full, res, zero)
	}
	return v, nil
}

// decodeCached giải mã và kiểm tra giá trị đọc từ cache. Trả nil nghĩa là dùng
// được; trả lỗi nghĩa là coi như cache trượt.
func decodeCached[T any](b []byte, v *T) error {
	if err := json.Unmarshal(b, v); err != nil {
		return err
	}
	// `null` giải mã thành công và cho con trỏ nil kèm err nil. Người gọi sẽ
	// deref nó và panic — cả tiến trình chết vì một khóa cache bị bơm bậy.
	if isNil(*v) {
		return errors.New("giá trị rỗng (null)")
	}
	if val, ok := any(*v).(Validator); ok {
		if err := val.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface,
		reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
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
	delCtx, cancel := withOpTimeout(ctx)
	defer cancel()
	if err := c.rdb.Del(delCtx, full...).Err(); err != nil {
		c.log.WarnContext(ctx, "không xóa được cache", "keys", full, "err", err)
	}
}

// jitter cộng ngẫu nhiên 0–20% vào TTL. d <= 0 trả về nguyên vẹn: rand.Int63n
// panic với đối số âm, và jitter(0) trả 0 — mà Set với 0 nghĩa là khóa vĩnh viễn.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	return d + time.Duration(rand.Int63n(int64(d/5)+1))
}
