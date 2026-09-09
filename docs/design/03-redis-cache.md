# 03 — Redis: cache, giỏ hàng, rate limit, khóa

---

## 1. Redis giữ mấy vai trò khác nhau — đừng trộn lẫn

Đây là điểm quan trọng nhất của tài liệu này. Redis trong dự án làm **hai loại việc
có yêu cầu độ bền hoàn toàn trái ngược nhau**:

| Loại | Dùng cho | Mất dữ liệu thì sao |
|---|---|---|
| **Cache** — dữ liệu phái sinh | Chi tiết sản phẩm, cây danh mục, số đếm | Không sao, đọc lại từ Postgres |
| **Dữ liệu gốc** — không có ở nơi khác | Giỏ hàng khách vãng lai, idempotency key | **Mất là mất thật** |

Trộn hai loại vào một instance với `maxmemory-policy allkeys-lru` thì khi đầy bộ
nhớ, Redis sẽ xóa giỏ hàng của khách để lấy chỗ cache sản phẩm.

⚠️ **Không tách được bằng database logic.** `maxmemory` và `maxmemory-policy` là
tham số **toàn server** — các database logic (DB 0, DB 1...) dùng chung một ngưỡng
bộ nhớ và một chính sách xóa. Không có cách nào đặt `allkeys-lru` cho DB 0 và
`noeviction` cho DB 1 trên cùng một instance. Đây là hiểu nhầm phổ biến.

**Quyết định: hai instance Redis riêng biệt**, ngay từ P0.2 khi bắt đầu dùng Redis.

| Instance | Cổng dev | Chính sách | Persistence |
|---|---|---|---|
| `redis-cache` | 6380 | `maxmemory 512mb` + `maxmemory-policy allkeys-lru` | Tắt — mất là đọc lại từ Postgres |
| `redis-data` | 6381 | `maxmemory-policy noeviction` | **Bật AOF**, `appendfsync everysec` |

Hai container trong `compose.dev.yml`, hai service trong production. Chi phí gần
như bằng không so với rủi ro xóa nhầm giỏ hàng của khách.

Với `noeviction`, khi đầy bộ nhớ Redis trả lỗi ghi thay vì âm thầm xóa dữ liệu —
lỗi ồn ào tốt hơn mất giỏ hàng im lặng. Còn `redis-cache` thì ngược lại: xóa key cũ
là hành vi đúng, vì mọi thứ trong đó đều dựng lại được.

*(Ở P0.1 compose chỉ có một Redis chạy mặc định `noeviction` + AOF — an toàn cho cả
hai vai trò vì chưa có code nào dùng tới. P0.2 tách thành hai instance.)*

**Việc P0.2 phải làm khi tách:** biến `REDIS_ADDR` hiện có trong `.env.example`
không diễn tả được hai instance, phải đổi thành `REDIS_CACHE_ADDR` (6380) và
`REDIS_DATA_ADDR` (6381). Lưu ý cổng 6380 sẽ **đổi ngữ nghĩa**: từ `noeviction` +
AOF sang `allkeys-lru` + không persistence. Đừng để giỏ hàng nằm lại trên cổng đó.

---

## 2. Quy ước đặt key

```
{app}:{ver}:{loại}:{định danh}[:{biến thể}]
```

```
bec:v1:product:slug:asus-rog-strix-g16
bec:v1:product:id:01937f3e-8a2c-7c1e-9f3b-2d4e5a6b7c8d
bec:v1:category:tree
bec:v1:catalog:count:laptop-gaming:brand=asus
bec:v1:cart:01937f40-...
bec:v1:rl:login:203.0.113.7
bec:v1:idem:01937f41-...
```

Quy tắc:

- **Luôn có `{ver}`.** Đổi cấu trúc dữ liệu cache → tăng `v1` thành `v2`, toàn bộ
  cache cũ tự hết hiệu lực. Đây là cách vô hiệu hóa hàng loạt an toàn nhất, thay
  cho `KEYS`/`SCAN` rồi xóa.
- **Không bao giờ dùng lệnh `KEYS`** trong code chạy thật — nó chặn toàn bộ Redis.
- Key chứa tham số của người dùng phải được chuẩn hóa và **sắp xếp** trước khi
  nối chuỗi, nếu không `?brand=asus&max=30` và `?max=30&brand=asus` tạo hai key
  khác nhau cho cùng một kết quả.
- Giá trị lưu **JSON**, không dùng msgpack/gob. Chậm hơn không đáng kể nhưng
  `redis-cli GET` đọc được bằng mắt lúc gỡ lỗi — đổi lại rất đáng.

---

## 3. Chính sách TTL

Nguyên tắc: **TTL tỉ lệ nghịch với hậu quả khi dữ liệu cũ.**

| Dữ liệu | TTL | Ghi chú |
|---|---|---|
| Cây danh mục, thương hiệu | 6 giờ | Gần như không đổi |
| Chi tiết sản phẩm (mô tả, thông số, ảnh) | 30 phút | + xóa key ngay khi cập nhật |
| Số đếm kết quả lọc (`COUNT`) | 5 phút | Sai vài chục sản phẩm không ai chết |
| Kết quả trang danh mục | 60 giây | Chỉ cache trang 1–3, phần còn lại bot cào là chính |
| **Giá** | **Không cache riêng** | Nằm trong payload sản phẩm, hết hạn cùng nhau |
| **Tồn kho** | **Không cache** | Đọc thẳng Postgres. Sai tồn kho = bán vượt = mất tiền thật |
| Kết quả 404 (negative cache) | 60 giây | Chặn bot dò slug làm ngập database |
| Session / giỏ khách vãng lai | 7 ngày, gia hạn mỗi lần chạm | instance `redis-data` |
| Idempotency key | 24 giờ | instance `redis-data` |

**Tồn kho tuyệt đối không cache.** Cám dỗ rất lớn vì nó bị đọc nhiều nhất, nhưng
hiển thị "còn hàng" khi đã hết là con đường ngắn nhất tới bán vượt và hủy đơn.
Nếu cần giảm tải, dùng read replica của Postgres, đừng dùng Redis.

---

## 4. Cache-aside

Chỉ một pattern duy nhất trong toàn dự án: đọc thì thử cache trước, trượt thì đọc
DB rồi ghi lại cache. **Không** dùng write-through, không dùng write-behind.

```go
// internal/platform/redis/cache.go
func GetOrLoad[T any](
    ctx context.Context, c *Cache, key string, ttl time.Duration,
    load func(context.Context) (T, error),
) (T, error) {
    var zero T

    if b, err := c.rdb.Get(ctx, key).Bytes(); err == nil {
        var v T
        if json.Unmarshal(b, &v) == nil {
            return v, nil
        }
        c.rdb.Del(ctx, key)      // dữ liệu hỏng → bỏ, đọc lại từ nguồn
    }

    // singleflight: 1000 request cùng trượt cache chỉ gọi DB một lần
    res, err, _ := c.sf.Do(key, func() (any, error) {
        v, err := load(ctx)
        if err != nil { return nil, err }
        if b, err := json.Marshal(v); err == nil {
            c.rdb.Set(ctx, key, b, jitter(ttl))
        }
        return v, nil
    })
    if err != nil { return zero, err }
    return res.(T), nil
}

// jitter tránh cache stampede: mọi key hết hạn cùng lúc → DB bị đấm một phát
func jitter(d time.Duration) time.Duration {
    return d + time.Duration(rand.Int63n(int64(d/5)))   // +0..20%
}
```

Hai chi tiết chống sập hệ thống:

1. **`singleflight`** (`golang.org/x/sync/singleflight`) — khi một sản phẩm hot
   hết hạn cache, hàng nghìn request đồng thời sẽ cùng trượt và cùng đấm vào
   Postgres. singleflight gộp chúng lại thành một query.
2. **Jitter TTL** — nếu 10.000 key được ghi cùng lúc với TTL 30 phút, chúng cũng
   hết hạn cùng lúc. Cộng ngẫu nhiên 0–20% để rải ra.

**Redis chết thì hệ thống phải vẫn chạy.** Mọi lỗi Redis đều bị nuốt và rơi xuống
đọc thẳng Postgres — không bao giờ trả lỗi cho người dùng vì cache trượt. Đặt
timeout ngắn (`DialTimeout` 200ms, `ReadTimeout` 100ms) để Redis treo không kéo
theo cả API.

---

## 5. Vô hiệu hóa cache

Ba tầng, theo thứ tự ưu tiên:

**1. Xóa key ngay khi ghi** — trong use case, **sau khi transaction commit thành công**:

```go
if err := uc.tx.Run(ctx, ...); err != nil { return err }
uc.cache.Delete(ctx, keyProductID(p.ID), keyProductSlug(p.Slug))
```

Xóa (`DEL`) chứ **không** ghi đè giá trị mới — ghi đè trong môi trường nhiều
tiến trình dễ tạo ra tình huống giá trị cũ ghi sau giá trị mới.

Xóa **sau** commit, không phải trong transaction: nếu transaction rollback mà đã
xóa cache thì chỉ tốn một lần đọc lại, còn xóa trước rồi commit lỗi thì cache có
thể được nạp lại bằng dữ liệu chưa commit.

**2. Sự kiện qua RabbitMQ** — worker nghe `product.updated` để xóa cache ở những
nơi use case không biết tới (ví dụ cache của trang danh mục chứa sản phẩm đó), và
gọi Next.js `revalidateTag` để làm mới trang ISR.

**3. Tăng version key** — khi đổi cấu trúc dữ liệu cache: `bec:v1:` → `bec:v2:`.
Không cần xóa gì, key cũ tự hết hạn theo TTL của nó.

---

## 6. Giỏ hàng

| | Khách vãng lai | Đã đăng nhập |
|---|---|---|
| Lưu ở | Redis (instance `redis-data`) | **PostgreSQL** |
| Key / bảng | `bec:v1:cart:{cart_id}` | `carts` + `cart_items` |
| Định danh | `cart_id` (UUIDv7) trong cookie HttpOnly, `SameSite=Lax` | `user_id` |
| TTL | 7 ngày, gia hạn mỗi lần chạm | Không hết hạn |

Giỏ của người đã đăng nhập nằm ở Postgres vì nó phải đồng bộ giữa điện thoại và
máy tính, và cần cho phân tích tỉ lệ bỏ giỏ.

**Khi đăng nhập:** gộp giỏ Redis vào giỏ Postgres — cùng SKU thì lấy số lượng lớn
hơn (không cộng dồn: khách thêm 1 cái ở hai thiết bị không có nghĩa là muốn 2 cái),
rồi xóa key Redis và cookie.

**Giỏ hàng chỉ lưu `product_id` + số lượng, không lưu giá.** Giá luôn tính lại lúc
hiển thị và lúc thanh toán. Cache giá trong giỏ là cách chắc chắn để bán sai giá
sau một đợt điều chỉnh.

Cấu trúc dùng Redis Hash để sửa từng dòng không phải đọc/ghi cả giỏ:

```
HSET bec:v1:cart:{id} {product_id} {quantity}
EXPIRE bec:v1:cart:{id} 604800
```

---

## 7. Rate limit

Thuật toán **sliding window** cài bằng Lua script (nguyên tử, một vòng mạng):

```lua
-- KEYS[1] = key, ARGV[1] = giới hạn, ARGV[2] = cửa sổ (giây)
local current = redis.call('INCR', KEYS[1])
if current == 1 then redis.call('EXPIRE', KEYS[1], ARGV[2]) end
return current <= tonumber(ARGV[1]) and 1 or 0
```

| Nhóm | Key | Giới hạn |
|---|---|---|
| API đọc công khai | `rl:pub:{ip}` | 120/phút |
| Đăng nhập, OTP, quên mật khẩu | `rl:auth:{ip}` **và** `rl:auth:{email}` | 5/phút |
| Tạo đơn, thanh toán | `rl:order:{user_id}` | 10/phút |
| Tìm kiếm | `rl:search:{ip}` | 30/phút |

Giới hạn theo **cả IP và định danh tài khoản** cho các endpoint xác thực. Chỉ theo
IP thì một mạng công ty dùng chung NAT sẽ bị chặn oan; chỉ theo tài khoản thì kẻ
tấn công đổi email là thoát.

Vượt giới hạn → 429 kèm `Retry-After`. Redis chết → **cho qua** (fail-open), vì
chặn hết khách hàng còn tệ hơn là để lọt vài request.

**Counter rate limit đặt ở `redis-cache`, không phải `redis-data`.** Đây là ngoại
lệ so với bảng phân vai ở mục 1, và có lý do: counter sinh ra rất nhiều key ngắn
hạn. Để chúng trên instance `noeviction` thì bộ nhớ chỉ tăng không giảm, tới lúc
đầy Redis sẽ từ chối mọi lệnh ghi — nghĩa là **giỏ hàng và idempotency key chết
theo**, đúng hai thứ quan trọng nhất. Đổi lại, khi `redis-cache` khởi động lại thì
counter mất và kẻ tấn công được một nhịp burst. Đó là đánh đổi chấp nhận được:
rate limit là lớp phòng thủ, không phải lớp bảo đảm tính đúng đắn.

---

## 8. Khóa phân tán — và khi nào KHÔNG được dùng

```go
ok, _ := rdb.SetNX(ctx, "bec:v1:lock:"+name, token, 30*time.Second).Result()
// nhả khóa bằng Lua: chỉ DEL nếu value đúng token của mình
```

**Chỉ dùng cho việc lặp lại không gây hại**: đảm bảo một job định kỳ chỉ chạy ở
một bản, tránh gửi trùng email hàng loạt, chống double-submit ở tầng UI.

⚠️ **Tuyệt đối không dùng khóa Redis để đảm bảo tính đúng đắn của tồn kho hay
tiền.** Khóa Redis có thể hết hạn giữa chừng trong khi tiến trình giữ khóa vẫn
đang chạy (GC pause, mạng chậm) — hai tiến trình cùng tưởng mình giữ khóa và cùng
trừ kho. Với những chỗ sai một lần là mất tiền, dùng cơ chế của Postgres:

```sql
-- Cách 1: để chính câu UPDATE làm trọng tài
UPDATE inventory SET available = available - $1
WHERE sku_id = $2 AND warehouse_id = $3 AND available >= $1;
-- RowsAffected() == 0  →  không đủ hàng

-- Cách 2: khóa tư vấn trong transaction, tự nhả khi commit/rollback
SELECT pg_advisory_xact_lock(hashtext($1));
```

---

## 9. Idempotency key

Endpoint ghi nhận header `Idempotency-Key` (UUIDv7 do client sinh):

```
bec:v1:idem:{key}  →  {"status":201,"body":"...","completed":true}   TTL 24h
```

Luồng: `SETNX` key với trạng thái `processing`.
- Đặt được → xử lý, xong thì ghi đè bằng response thật
- Đã tồn tại và `completed` → trả lại response cũ nguyên vẹn
- Đã tồn tại và `processing` → **409**, yêu cầu client thử lại sau

Bắt buộc với tạo đơn và thanh toán. Người dùng bấm hai lần vào nút "Đặt hàng" là
chuyện xảy ra hằng ngày.

---

## 10. Vận hành

**Cấu hình Redis (production):**
```
maxmemory 2gb
maxmemory-policy allkeys-lru      # instance cache
# instance data: noeviction + appendonly yes + appendfsync everysec
timeout 300
tcp-keepalive 60
```

**Cấu hình client Go** (`redis/go-redis/v9`):
`PoolSize` 20, `DialTimeout` 200ms, `ReadTimeout` 100ms, `WriteTimeout` 100ms,
`MaxRetries` 1. Timeout ngắn là có chủ đích — cache chậm thì thà bỏ qua.

**Chỉ số phải theo dõi:**

| Chỉ số | Ngưỡng cảnh báo |
|---|---|
| Tỉ lệ hit | < 80% với cache sản phẩm |
| `evicted_keys` | > 0 trên instance `data` — nghĩa là sắp mất dữ liệu |
| `used_memory` / `maxmemory` | > 85% |
| Độ trễ p99 | > 5ms |
| `rejected_connections` | > 0 |

---

## 11. Việc cần làm

- [ ] `platform/redis`: client, hai kết nối (`cache`/`data`), timeout, health check
- [ ] `GetOrLoad[T]` với singleflight + jitter TTL
- [ ] Helper key: `keyProductID`, `keyProductSlug`, `keyCategoryTree`... tập trung một file
- [ ] Nuốt lỗi Redis, luôn rơi xuống Postgres, có metric đếm số lần rơi
- [ ] Negative cache cho 404
- [ ] `CartStore`: Redis cho khách vãng lai, Postgres cho user, hàm gộp khi đăng nhập
- [ ] Middleware rate limit bằng Lua script, cấu hình theo nhóm
- [ ] Middleware `Idempotency-Key`
- [ ] Worker nghe `product.updated` → xóa cache + gọi `revalidateTag` của Next.js
- [ ] Test: Redis chết thì API vẫn trả đúng dữ liệu (dừng container giữa test)
- [ ] Test: 100 goroutine cùng trượt cache → chỉ 1 query xuống DB
