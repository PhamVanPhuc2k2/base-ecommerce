# 01 — Transaction & Outbox

Thiết kế cách `app` mở transaction mà **không biết pgx là gì**, và cách phát sự
kiện ra RabbitMQ mà không mất event.

---

## 1. Vấn đề

Use case `CreateProduct` cần làm 3 việc **nguyên tử**:

```
lưu product  +  ghi outbox event  →  cùng commit hoặc cùng rollback
```

Nhưng theo nguyên tắc 3.1, `app` không được import `pgx`. Vậy `app` mở transaction
bằng cách nào, và làm sao để hai repository khác nhau cùng dùng **đúng một** `pgx.Tx`?

Đây là chỗ Hexagonal trong Go hay vỡ trận. Ba cách giải thường gặp:

| Cách | Đánh giá |
|---|---|
| Truyền `Tx` qua tham số: `repo.Save(ctx, tx, p)` | Rò rỉ kiểu hạ tầng vào port của `app`. Loại |
| Repository có `WithTx(tx) Repository` | Vẫn phải cầm `tx` ở `app`. Loại |
| **Truyền `Tx` ngầm qua `context`** | `app` chỉ thấy interface thuần. **Chọn cách này** |

Đánh đổi của cách 3: nó **ngầm** — nhìn chữ ký hàm không biết đang trong transaction
hay không. Bù lại bằng một kỷ luật duy nhất, có thể kiểm bằng máy:
**repository luôn lấy kết nối qua `db.DB(ctx)`, không bao giờ giữ pool trực tiếp.**

---

## 2. Port khai báo trong `app`

```go
// internal/catalog/app/ports.go
package app

// TxManager cho phép use case gom nhiều thao tác vào một transaction.
// app không biết bên dưới là Postgres hay gì khác.
type TxManager interface {
    Run(ctx context.Context, fn func(ctx context.Context) error) error
}
```

Chỉ một method. Không có `Begin`/`Commit`/`Rollback` lộ ra ngoài — nếu lộ thì sớm
muộn sẽ có người quên `Rollback`.

---

## 3. Cài đặt trong `platform/postgres`

```go
// internal/platform/postgres/tx.go
package postgres

type txKey struct{}

type Manager struct{ pool *pgxpool.Pool }

func NewManager(pool *pgxpool.Pool) *Manager { return &Manager{pool: pool} }

// DB trả về tx nếu ctx đang ở trong transaction, ngược lại trả pool.
// MỌI repository phải lấy kết nối qua đây.
func (m *Manager) DB(ctx context.Context) DBTX {
    if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
        return tx
    }
    return m.pool
}

func (m *Manager) Run(ctx context.Context, fn func(context.Context) error) error {
    return m.RunWith(ctx, TxOptions{}, fn)
}

func (m *Manager) RunWith(ctx context.Context, opt TxOptions, fn func(context.Context) error) error {
    // Đã ở trong transaction → tái sử dụng, không mở transaction lồng nhau.
    if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
        return fn(ctx)
    }

    return m.retry(ctx, opt, func(ctx context.Context) error {
        tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: opt.isoLevel()})
        if err != nil {
            return fmt.Errorf("begin tx: %w", err)
        }
        // Rollback là no-op nếu đã commit. Luôn defer để không rò rỉ kết nối
        // khi fn panic.
        defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

        if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
            return err
        }
        return tx.Commit(ctx)
    })
}
```

**Chú ý `context.WithoutCancel(ctx)` ở `Rollback`.** Nếu client ngắt kết nối giữa
chừng, `ctx` đã bị hủy và `Rollback(ctx)` sẽ không chạy được — kết nối bị treo cho
tới khi pool tự thu hồi. Đây là lỗi rất khó tìm.

### 3.1. Isolation level và retry

```go
type TxOptions struct {
    Serializable bool
    MaxRetries   int   // mặc định 3
}
```

Postgres ở mức `SERIALIZABLE` (và cả `REPEATABLE READ`) có thể trả lỗi
**serialization failure** — đây là hành vi bình thường, không phải bug. Cách đúng
là **thử lại cả transaction**:

```go
func isRetryable(err error) bool {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) { return false }
    switch pgErr.Code {
    case "40001", // serialization_failure
         "40P01": // deadlock_detected
        return true
    }
    return false
}
```

Retry có backoff + jitter, tối đa `MaxRetries`. Vì transaction được thử lại nguyên
vẹn nên **`fn` phải không có tác dụng phụ bên ngoài DB** — xem quy tắc bên dưới.

---

## 4. Repository dùng như thế nào

```go
// internal/catalog/adapter/pgstore/repository.go
type ProductRepository struct{ db *postgres.Manager }

func (r *ProductRepository) Save(ctx context.Context, p *domain.Product) error {
    q := gen.New(r.db.DB(ctx))        // ← tự động dùng tx nếu đang trong tx
    return q.UpsertProduct(ctx, toRow(p))
}
```

Repository **không có** field `pool`. Đó là toàn bộ kỷ luật cần giữ, và CI kiểm được:

```bash
grep -rn "pgxpool.Pool" internal/*/adapter/ && exit 1
```

---

## 5. Use case dùng như thế nào

```go
// internal/catalog/app/create_product.go
type CreateProduct struct {
    tx     TxManager
    repo   ProductRepository
    outbox OutboxWriter
}

func (uc *CreateProduct) Execute(ctx context.Context, in CreateProductInput) (*domain.Product, error) {
    p, err := domain.NewProduct(in.SKU, in.Name, in.Price)
    if err != nil {
        return nil, err                       // lỗi nghiệp vụ, chưa chạm DB
    }

    // PullEvents gọi TRƯỚC closure, không phải trong. TxManager chạy lại closure
    // khi gặp lỗi tuần tự hóa, mà PullEvents làm rỗng danh sách sự kiện của
    // entity — gọi trong closure thì lần thử thứ hai ghi vào outbox KHÔNG sự
    // kiện nào, và event mất vĩnh viễn đúng lúc hệ thống đang tranh chấp.
    events := p.PullEvents()

    err = uc.tx.Run(ctx, func(ctx context.Context) error {
        if err := uc.repo.Save(ctx, p); err != nil { return err }
        return uc.outbox.Append(ctx, events...)   // CÙNG transaction
    })
    if err != nil {
        return nil, err
    }
    return p, nil
}
```

`app` không hề nhắc tới pgx. Đổi sang database khác chỉ cần viết `Manager` mới.

---

## 6. Bốn quy tắc bắt buộc

1. **Transaction chỉ được mở ở tầng `app`.** Không mở trong handler, không mở trong
   repository. Một use case = tối đa một transaction.
2. **Không gọi I/O bên ngoài bên trong transaction** — không HTTP, không publish
   RabbitMQ, không gửi email. Lý do: transaction giữ kết nối Postgres, mà kết nối
   là tài nguyên khan hiếm; một API bên thứ ba chậm 5 giây sẽ làm cạn pool. Đây
   chính là lý do outbox tồn tại.
3. **Transaction phải ngắn.** Validate và tính toán xong hết rồi mới `Run`.
4. **`fn` phải chạy lại được** (idempotent trong bộ nhớ), vì có thể bị retry.

---

## 7. Outbox

### 7.1. Vì sao cần

Nếu commit DB xong mới publish RabbitMQ, và publish lỗi (mạng, broker restart),
thì đơn hàng đã tạo nhưng event mất vĩnh viễn. Đây là bài toán **dual-write**,
không thể giải bằng try/catch.

Cách giải: ghi event vào bảng cùng transaction với dữ liệu nghiệp vụ. Đã commit
nghĩa là event chắc chắn tồn tại. Một tiến trình riêng đọc bảng đó và publish.

```
[api]  BEGIN → INSERT product → INSERT outbox → COMMIT
                                       │
[outboxrelay]  poll ──► publish RabbitMQ ──► UPDATE published_at
                                       │
[worker]  consume ──► kiểm tra đã xử lý chưa ──► xử lý
```

### 7.2. Schema

```sql
CREATE TABLE outbox (
    id             UUID        PRIMARY KEY,   -- UUIDv7, ĐỒNG THỜI là event_id
    aggregate_type TEXT        NOT NULL,      -- 'product'
    aggregate_id   UUID        NOT NULL,
    event_type     TEXT        NOT NULL,      -- 'product.published'
    payload        JSONB       NOT NULL,
    trace_id       TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at   TIMESTAMPTZ,
    attempts       INT         NOT NULL DEFAULT 0,
    last_error     TEXT
);

-- Chỉ index phần chưa gửi: bảng có thể rất lớn nhưng phần chưa gửi luôn nhỏ.
CREATE INDEX outbox_unpublished_idx ON outbox (id) WHERE published_at IS NULL;
```

**Khóa chính là UUIDv7 và cũng chính là `event_id`** — không cần hai cột. Consumer
khử trùng lặp bằng đúng giá trị này.

Dùng được UUID ở đây là nhờ **v7 có thứ tự theo thời gian**: 48 bit đầu là mốc
thời gian mili-giây, nên `ORDER BY id` cho ra gần đúng thứ tự sinh event, và B-tree
luôn chèn ở mép phải (không phân mảnh như UUIDv4).

⚠️ **Giới hạn cần biết:** trong cùng một mili-giây, thứ tự giữa các UUIDv7 là ngẫu
nhiên; và dù dùng UUIDv7 hay `BIGINT IDENTITY` thì thứ tự **commit** vẫn có thể
khác thứ tự **sinh ID**. Nên outbox chỉ đảm bảo thứ tự *xấp xỉ*. Khi nào cần thứ
tự tuyệt đối theo từng aggregate (ví dụ chuỗi trạng thái đơn hàng), thêm cột
`version INT` tăng dần theo `aggregate_id` và để consumer tự sắp xếp — đừng dựa
vào thứ tự của queue.

Index **partial** (`WHERE published_at IS NULL`) là điểm quan trọng: bảng outbox
tích lũy hàng triệu dòng nhưng index chỉ chứa vài chục dòng đang chờ.

**Kiểu cột phải là `UUID` (16 byte), không phải `TEXT`/`VARCHAR(36)`.** Lưu dạng
chuỗi tốn hơn gấp đôi dung lượng, so sánh chậm hơn, và index phình to.

### 7.3. Envelope của event

```json
{
  "event_id":   "01937f3e-...",
  "event_type": "product.published",
  "occurred_at":"2026-09-09T10:23:41Z",
  "aggregate":  { "type": "product", "id": "0193..." },
  "trace_id":   "4bf92f3577b34da6a3ce929d0e0e4736",
  "payload":    { "sku": "ASUS-ROG-G16", "price": "25990000" }
}
```

`event_id` chính là `outbox.id`, dùng để consumer khử trùng lặp. `trace_id` để nối
trace từ HTTP request sang worker — không có nó thì debug async gần như bất khả thi.

### 7.4. Relay

```sql
-- name: FetchUnpublished :many
SELECT * FROM outbox
WHERE published_at IS NULL
ORDER BY id
LIMIT $1
FOR UPDATE SKIP LOCKED;
```

Vòng lặp:

```
BEGIN
  rows = FetchUnpublished(100)          -- khóa các dòng này
  nếu rỗng → COMMIT, chờ tick tiếp theo
  publish từng row sang RabbitMQ (confirm mode)
  UPDATE outbox SET published_at = now() WHERE id = ANY(...)
COMMIT
```

Chi tiết quan trọng:

- **Bật publisher confirm của RabbitMQ.** Không có confirm thì `Publish()` trả về
  thành công ngay cả khi broker chưa nhận — vô hiệu hóa toàn bộ ý nghĩa của outbox.
- **Chạy đúng MỘT bản `outboxrelay`.** Nhiều bản + `SKIP LOCKED` sẽ phá vỡ thứ tự
  event. Nếu sau này cần scale, phân mảnh theo `hash(aggregate_id)`.
- **Nhịp poll**: 500ms là đủ cho P0. Muốn giảm độ trễ thì thêm `LISTEN/NOTIFY`:
  trigger `NOTIFY outbox_new` khi insert, relay `LISTEN` để tỉnh dậy ngay.
  Vẫn giữ poll làm mạng lưới an toàn — đừng bỏ poll.
- **Lỗi publish**: `attempts++`, ghi `last_error`, không set `published_at`. Vòng
  sau thử lại. Quá 10 lần → cảnh báo (Sentry), không tự xóa.
- **Dọn dẹp**: xóa dòng đã publish quá 7 ngày, chạy hằng ngày.

### 7.5. Đảm bảo giao hàng

Outbox cho **at-least-once**, không phải exactly-once. Một event có thể tới worker
nhiều lần (relay publish xong nhưng crash trước khi UPDATE). Vì vậy consumer
**bắt buộc** phải idempotent:

```sql
CREATE TABLE processed_events (
    consumer     TEXT        NOT NULL,
    event_id     UUID        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
```

```go
// Trong CÙNG transaction với việc xử lý nghiệp vụ
tag, err := q.MarkProcessed(ctx, consumer, evt.EventID)  // ON CONFLICT DO NOTHING
if err != nil { return err }
if tag.RowsAffected() == 0 {
    return nil       // đã xử lý rồi → ack và bỏ qua
}
// ... xử lý nghiệp vụ ...
```

### 7.6. Cấu hình RabbitMQ

| | |
|---|---|
| Exchange | `ecommerce.events`, kiểu `topic`, durable |
| Routing key | `product.published`, `order.created`, ... |
| Queue | Mỗi consumer một queue riêng, durable, binding theo pattern |
| Retry | Queue `*.retry` với `x-message-ttl` + `x-dead-letter-exchange` quay lại queue chính |
| DLQ | Quá 5 lần → `*.dlq`, cảnh báo Sentry, **không tự xóa message** |
| Prefetch | `qos.prefetch = 10` (đừng để mặc định không giới hạn) |
| Ack | Thủ công, ack **sau** khi xử lý xong |

---

## 8. Việc cần làm

- [x] `platform/postgres/tx.go`: `Manager`, `DB(ctx)`, `Run`, `RunWith`, retry
- [x] Kiểm chứng `Manager` bằng kịch bản `cmd/scratch` tạm: commit, rollback, lồng
      transaction, context bị hủy, và `pool.Stat().AcquiredConns() == 0`
- [x] Migration bảng `outbox` + `processed_events`
- [x] `internal/outbox`: `Append`, `FetchUnpublished`, `MarkPublished`, `MarkFailed`
- [x] `cmd/outboxrelay`: vòng lặp poll + publisher confirm + graceful shutdown
- [x] `platform/rabbitmq`: publisher (confirm), consumer (prefetch, manual ack, retry, DLQ)
- [x] Khử trùng lặp bằng `outbox.MarkProcessed` trong CÙNG transaction với việc xử lý.
      Không tách ra thành helper `worker.Idempotent` như dự định ban đầu: bọc nó
      trong một hàm nhận callback làm mờ đúng cái quan trọng nhất — rằng đánh dấu
      và xử lý phải cùng sống hoặc cùng chết trong một transaction
- [x] Job dọn outbox cũ (và `processed_events`), xóa theo lô, chạy ngay lúc khởi động
- [x] Kiểm chứng bằng tay: tạo product → `psql` thấy dòng outbox → relay publish →
      worker nhận (xem log) → `published_at` được cập nhật
- [x] Kiểm chứng bằng tay: publish lại cùng `event_id` → worker bỏ qua, `processed_events`
      không thêm dòng mới

### Thêm vào sau khi làm thật (11/09/2026)

- [x] **Chặn `FetchUnpublished` chạy ngoài transaction bằng máy.** `FOR UPDATE
      SKIP LOCKED` ngoài transaction vẫn trả dữ liệu và không lỗi gì, nhưng khóa
      nhả ngay — hai bản relay sẽ lấy trùng dòng. Không có triệu chứng nào cho
      tới lúc chạy hai bản cùng lúc trên production. `Manager.InTx(ctx)` để câu
      lệnh tự chặn mình
- [x] **Message không route được phải là LỖI**, không chỉ là dòng log. Broker ack
      chỉ có nghĩa "tôi đã nhận", không có nghĩa "có queue nào giữ nó" — thiếu
      chỗ này thì relay đánh dấu `published_at` cho message đã bị vứt
- [x] **Luật kiến trúc 4**: hạ tầng dùng chung không được phụ thuộc module nghiệp
      vụ, kể cả gián tiếp
