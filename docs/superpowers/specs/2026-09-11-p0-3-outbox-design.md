# P0.3 — Outbox, relay và worker: đặc tả

**Mục tiêu:** sự kiện nghiệp vụ đi từ transaction database ra RabbitMQ tới worker
mà **không mất và không xử lý trùng**, và làm được điều đó bằng cách thay đúng
một adapter — không sửa `domain` hay `app` của module `catalog`.

**Nền:** [thiết kế 01 — Transaction & Outbox](../../design/01-transaction-outbox.md)
đã chốt schema, envelope, vòng lặp relay và cấu hình RabbitMQ. Tài liệu này chỉ
giải quyết những chỗ thiết kế 01 để ngỏ, và ghi lại các quyết định đã chốt.

---

## 1. Quyết định đã chốt

| | Chọn | Vì sao |
|---|---|---|
| Phạm vi | **Đầy đủ** — outbox, relay, worker, retry + DLQ, job dọn | Retry/DLQ là phần dễ bỏ qua nhất, mà thiếu nó thì message lỗi quay vòng vô hạn hoặc mất — đúng thứ outbox sinh ra để tránh |
| Đánh thức relay | **Chỉ poll 500 ms** | Độ trễ trung bình ~250 ms, thừa cho mọi việc P0. `LISTEN/NOTIFY` thêm trigger, thêm kết nối riêng, thêm một đường hỏng im lặng phải kiểm |
| `domain.Event` | **Thêm `AggregateType()` và `Payload()`** | Sửa `domain` đúng một lần; sau đó mỗi event tự khai báo. Type-switch trong adapter thì mỗi event mới lại phải sửa adapter, và quên thì payload rỗng mà không có lỗi nào |

---

## 2. Chiều phụ thuộc — chỗ dễ làm sai nhất

`app.EventPublisher` nhận `catalog/domain.Event`. Nhưng `internal/outbox` là hạ
tầng **dùng chung cho mọi module**, nên nó **không được** import `catalog/domain`.

```
catalog/app  ──(port EventPublisher)──►  catalog/adapter/outboxpub
                                                  │ dịch domain.Event → outbox.Record
                                                  ▼
                                          internal/outbox   (không biết catalog là gì)
                                                  ▲
                                          cmd/outboxrelay
```

`outbox.Record` là struct hạ tầng thuần: `ID`, `AggregateType`, `AggregateID`,
`EventType`, `Payload []byte`, `TraceID`. Không có kiểu nào của `domain` trong đó.

⚠️ Nếu ai đó cho `internal/outbox` import `catalog/domain` thì module `orders` ở
P4 sẽ kéo theo `catalog` — và đó là lúc modular monolith biến thành big ball of
mud. `scripts/check-arch.sh` phải có luật chặn việc này (xem §7).

---

## 3. `domain.Event` sau khi mở rộng

```go
type Event interface {
	EventID() uuid.UUID
	EventType() string
	AggregateID() uuid.UUID
	AggregateType() string   // MỚI: "product"
	OccurredAt() time.Time
	Payload() any            // MỚI: được marshal thành JSONB
}
```

`baseEvent` **không** cài `AggregateType()` và `Payload()` — cố ý. Để nó cài mặc
định thì thêm một event mới mà quên khai báo sẽ lọt qua trình biên dịch và ra
production với payload rỗng. Bắt buộc từng event tự khai thì quên là **không
biên dịch được**.

### 3.1. Event mỏng, không phải event dày

Payload chỉ mang định danh và vài trường ổn định nhất:

```go
func (e ProductPublished) Payload() any {
	return map[string]any{"sku": e.sku, "slug": e.slug}
}
```

Consumer cần gì hơn thì **tự đọc lại từ database**. Lý do: payload dày nghĩa là
mỗi lần đổi hình dạng sản phẩm là đổi hợp đồng của mọi consumer, và những event
cũ nằm trong outbox mang hình dạng cũ mãi mãi. Event mỏng thì hợp đồng gần như
không bao giờ đổi.

Đánh đổi đã biết: consumer đọc lại DB thấy **trạng thái mới nhất**, không phải
trạng thái lúc event xảy ra. Với indexer (P7) thì đó là điều mong muốn. Với
nghiệp vụ cần ảnh chụp tại thời điểm (ví dụ giá lúc đặt hàng ở P4) thì **phải**
đưa giá trị đó vào payload — ghi rõ ở đây để P4 không mặc định làm theo.

---

## 4. Envelope gửi sang RabbitMQ

Đúng như thiết kế 01 §7.3. `event_id` chính là `outbox.id`.

```json
{
  "event_id":   "01937f3e-...",
  "event_type": "product.published",
  "occurred_at":"2026-09-11T10:23:41Z",
  "aggregate":  { "type": "product", "id": "0193..." },
  "trace_id":   "dothanhdat/abc-000007",
  "payload":    { "sku": "ASUS-ROG-G16", "slug": "laptop-asus-rog-strix-g16" }
}
```

`trace_id` lấy từ `middleware.GetReqID(ctx)` lúc ghi outbox. Không có nó thì gỡ
lỗi bất đồng bộ gần như bất khả thi: không nối được dòng log của worker với
request HTTP đã sinh ra nó.

---

## 5. Cấu trúc RabbitMQ

| | |
|---|---|
| Exchange | `ecommerce.events`, `topic`, durable |
| Routing key | chính là `event_type`: `product.created`, `product.published` |
| Queue chính | `catalog.indexer`, durable, binding `product.*` |
| Queue retry | `catalog.indexer.retry`, `x-message-ttl: 30000`, `x-dead-letter-exchange: ""`, `x-dead-letter-routing-key: catalog.indexer` |
| DLQ | `catalog.indexer.dlq`, durable. **Không tự xóa message** |
| Prefetch | 10 |
| Ack | thủ công, sau khi xử lý xong |

### 5.1. Retry bằng TTL + DLX — và cái bẫy của nó

Message lỗi được publish sang `catalog.indexer.retry`. Queue đó không có
consumer; sau 30 giây TTL hết hạn và RabbitMQ tự đẩy message về
`catalog.indexer`.

⚠️ **`x-message-ttl` của queue hết hạn theo thứ tự ĐẦU HÀNG, không theo từng
message.** Message ở giữa hàng có TTL hết trước vẫn phải chờ message đầu hàng.
Với TTL cố định 30 s cho mọi message thì không sao — mọi message vào cùng thứ tự
và hết hạn cùng thứ tự. Nhưng **đừng** đặt TTL riêng cho từng message (backoff
lũy tiến) trên cùng một queue: một message TTL 10 phút nằm đầu hàng sẽ chặn mọi
message TTL 30 giây phía sau. Muốn backoff lũy tiến thì cần nhiều queue retry,
mỗi queue một TTL — để P1.

Đếm số lần thử bằng header `x-death` do RabbitMQ tự gắn. Quá **5** lần → publish
thẳng sang DLQ và ghi log mức ERROR.

### 5.2. Hai loại lỗi phải phân biệt

| Loại | Ví dụ | Xử lý |
|---|---|---|
| **Tạm thời** | mất kết nối DB, timeout | nack → retry queue |
| **Vĩnh viễn** | JSON không parse được, `event_type` lạ | **thẳng vào DLQ**, đừng retry |

Retry một message hỏng cú pháp là lãng phí 5 lần × 30 giây rồi vẫn vào DLQ, và
trong lúc đó nó chiếm chỗ của message tốt.

---

## 6. Idempotency

Bảng `processed_events` như thiết kế 01 §7.5. Điểm bắt buộc:

`MarkProcessed` phải nằm **trong cùng transaction** với việc xử lý nghiệp vụ.
Tách ra thì có khe cửa: đánh dấu xong, crash trước khi xử lý → event mất vĩnh
viễn mà bảng nói là đã xử lý.

```go
// ON CONFLICT DO NOTHING
if n == 0 { return nil }   // đã xử lý rồi → ack, bỏ qua
```

**`consumer` là một phần của khóa chính.** Hai consumer khác nhau phải xử lý được
cùng một event một cách độc lập. Quên cột này thì consumer thứ hai thêm vào ở P7
sẽ thấy mọi event đều "đã xử lý".

---

## 7. Luật kiến trúc mới cho `check-arch.sh`

**Luật 4: `internal/outbox` và `internal/platform/**` không được import bất kỳ
module nghiệp vụ nào** (`internal/catalog`, và sau này `internal/orders`...).

Hạ tầng biết về nghiệp vụ là con đường một chiều dẫn tới big ball of mud, và nó
xảy ra rất tự nhiên: chỉ cần một lần "tiện tay" import `catalog/domain` để lấy
một hằng số.

Luật này phải được chứng minh là **bắt được vi phạm** trước khi tin.

---

## 8. Chạy đúng một bản relay

`FOR UPDATE SKIP LOCKED` cho phép nhiều bản relay chạy song song mà không đụng
nhau — nhưng **phá vỡ thứ tự event**, vì bản A có thể publish event #2 trước khi
bản B publish event #1.

P0.3 chạy **một** bản. `compose.prod.yml` không được đặt `replicas` cho
`outboxrelay`. Ghi rõ trong file đó.

Muốn scale ở P7 thì phân mảnh theo `hash(aggregate_id) % N` để mọi event của cùng
một aggregate luôn về cùng một bản.

---

## 9. Job dọn outbox

Xóa dòng đã `published_at` quá **7 ngày**, chạy hằng ngày trong chính tiến trình
`outboxrelay` (không cần cron riêng ở P0).

```sql
DELETE FROM outbox WHERE published_at < now() - interval '7 days';
```

⚠️ Xóa hàng loạt trên bảng lớn khóa lâu và làm phình WAL. Xóa theo lô
`LIMIT 10000` trong vòng lặp, nghỉ giữa các lô.

`processed_events` cũng phải dọn, cùng ngưỡng — nó lớn nhanh hơn outbox vì mỗi
consumer thêm một dòng cho mỗi event.

---

## 10. Kiểm chứng thủ công

Không có unit test. Mười hai mục dưới đây thay thế.

| # | Kiểm | Kỳ vọng |
|---|---|---|
| 1 | `task check` | build + vet + lint + arch + api-codes + tree đều xanh |
| 2 | Tạo sản phẩm | `psql` thấy đúng **một** dòng `outbox`, `published_at IS NULL`, `payload` có `sku` |
| 3 | Dòng outbox và dòng product cùng transaction | Ép `repo.Save` lỗi → **không** có dòng outbox nào; ép `outbox.Append` lỗi → **không** có sản phẩm nào |
| 4 | Chạy relay | `published_at` được set; RabbitMQ UI thấy message |
| 5 | Chạy worker | Log worker có `event_id` **và** `trace_id` khớp với request HTTP đã tạo sản phẩm |
| 6 | **Gửi lại cùng `event_id`** | Worker bỏ qua, `processed_events` **không** thêm dòng |
| 7 | **Tắt RabbitMQ rồi tạo sản phẩm** | API vẫn 201; dòng outbox vẫn được ghi; bật lại → relay tự đẩy đi |
| 8 | **Giết relay giữa lúc publish** | Bật lại → event được publish **lại**, worker khử trùng lặp. Không mất event |
| 9 | Message lỗi tạm thời | Vào `catalog.indexer.retry`, 30 s sau quay lại queue chính |
| 10 | Message lỗi vĩnh viễn (JSON hỏng) | Vào **thẳng** DLQ, không quay vòng |
| 11 | Quá 5 lần thử | Vào DLQ, log ERROR, message **vẫn còn** trong DLQ |
| 12 | `docker stop outboxrelay` giữa lúc chạy | Exit code **0**, log có dòng dừng êm, không có dòng outbox nào kẹt ở trạng thái nửa vời |

**Mục 3, 7 và 8 quan trọng nhất** — đó chính là ba tình huống outbox sinh ra để
giải quyết, và cả ba đều **không** lộ ra khi mọi thứ chạy bình thường.

---

## 11. Giới hạn chấp nhận ở P0.3

- **At-least-once, không phải exactly-once.** Consumer bắt buộc idempotent. Không
  có cách nào bỏ yêu cầu này.
- **Thứ tự chỉ xấp xỉ.** UUIDv7 cùng mili-giây thì thứ tự ngẫu nhiên, và thứ tự
  commit có thể khác thứ tự sinh ID. Nghiệp vụ cần thứ tự tuyệt đối theo
  aggregate (chuỗi trạng thái đơn hàng ở P6) phải thêm cột `version`.
- **Một bản relay.** Xem §8.
- **Backoff cố định 30 giây.** Xem §5.1.
- **Worker chỉ ghi log.** Nó là khung để P7 thay bằng indexer Meilisearch. Giá
  trị của nó ở P0.3 là chứng minh đường đi hoạt động, không phải làm việc gì.
