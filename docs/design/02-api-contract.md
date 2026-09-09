# 02 — Hợp đồng API

Chốt: mô hình lỗi, phân trang, quy ước JSON, quy trình OpenAPI.

---

## 1. Quy ước JSON

| Hạng mục | Quyết định | Lý do |
|---|---|---|
| Tên trường | **`snake_case`** | Khớp với Postgres và sqlc, không phải đổi tên ở tầng nào |
| ID | **UUID v7** dạng chuỗi | Có thứ tự theo thời gian → index Postgres không bị phân mảnh như UUID v4 |
| Thời gian | **RFC 3339, UTC, hậu tố `Z`** | `"2026-09-09T10:23:41Z"` |
| Tiền tệ | **Chuỗi decimal**: `"25990000"` | `number` của JS mất chính xác với số lớn |
| Đơn vị tiền | Luôn kèm `currency: "VND"` | Chuẩn bị cho đa tiền tệ, không phải sửa sau |
| Enum | Chuỗi `snake_case`: `"out_of_stock"` | Đọc được trong log, không vỡ khi thêm giá trị |
| Trường rỗng | Trả `null`, **không bỏ trường** | Client không phải phân biệt "không có" và "chưa gửi" |
| Boolean | Tên khẳng định: `is_active`, không `is_not_active` | |

Prefix đường dẫn: **`/api/v1/`** ngay từ đầu. Thêm version sau khi đã có client
chạy thật là việc rất tốn kém.

### 1.1. Quy ước ID — UUID v7 cho toàn bộ hệ thống

**Mọi khóa chính đều là UUID v7.** Không có bảng nào dùng `SERIAL`/`BIGINT IDENTITY`,
kể cả bảng nội bộ như `outbox`.

**Vì sao v7 chứ không phải v4:** 48 bit đầu của v7 là mốc thời gian mili-giây, nên
ID có thứ tự tăng dần theo thời gian. Hệ quả với Postgres:

- B-tree luôn chèn ở **mép phải** → không phân mảnh trang, không phải `REINDEX` định kỳ
- Dữ liệu mới nằm gần nhau trên đĩa → truy vấn "đơn hàng gần đây" đọc ít trang hơn
- `ORDER BY id` xấp xỉ `ORDER BY created_at` — dùng được cho phân trang cursor

UUIDv4 thì ngược lại: mỗi lần chèn rơi vào một vị trí ngẫu nhiên trong index, gây
phân mảnh nặng và làm chậm dần theo thời gian. Đây là lý do UUID mang tiếng xấu, và
v7 giải quyết đúng vấn đề đó.

**Vì sao không dùng số tăng dần:** ID tuần tự lộ thông tin kinh doanh ra ngoài.
`/api/v1/orders/1042` cho biết cửa hàng mới có 1042 đơn; đối thủ đặt hai đơn cách
nhau một ngày là đo được sản lượng. Với đơn hàng còn là lỗ hổng liệt kê — đoán ID
để xem đơn của người khác. UUID chặn cả hai.

**Quy tắc bắt buộc:**

| | |
|---|---|
| Kiểu cột Postgres | `UUID` (16 byte). **Không** `TEXT`/`VARCHAR(36)` — tốn hơn gấp đôi, index phình, so sánh chậm |
| Nơi sinh ID | **Ở Go, tầng `domain`**, trong constructor entity — không phải `DEFAULT` của Postgres |
| Thư viện | `github.com/google/uuid` (`uuid.NewV7()`) — nằm trong danh sách trắng của quy tắc 3.1 |
| Dạng trong JSON | Chuỗi chuẩn có dấu gạch: `"01937f3e-8a2c-7c1e-9f3b-2d4e5a6b7c8d"` |

Sinh ID ở tầng domain chứ không để Postgres `DEFAULT gen_random_uuid()`, vì entity
phải có ID **trước khi** insert — domain event tham chiếu tới ID đó và được ghi vào
outbox trong cùng transaction.

*(Postgres 18 có hàm `uuidv7()` sẵn, nhưng vẫn nên sinh ở Go vì lý do trên. Dùng
`DEFAULT` chỉ như lưới an toàn khi chèn dữ liệu bằng tay.)*

### 1.2. Mã hiển thị cho người dùng — tách khỏi khóa chính

Không ai đọc UUID qua điện thoại được. Những thực thể khách hàng phải nhắc tới cần
thêm **một cột mã riêng**, ngắn và đọc được, bên cạnh khóa chính UUID:

| Thực thể | Cột `code` | Ví dụ |
|---|---|---|
| Đơn hàng | `orders.code` | `DH26090042` |
| Phiếu bảo hành | `warranties.code` | `BH26090017` |
| Hóa đơn | `invoices.code` | theo quy định hóa đơn điện tử |
| Sản phẩm | `products.sku` | `ASUS-ROG-G16-2026` |

`code` là `TEXT UNIQUE NOT NULL`, sinh theo định dạng `{tiền tố}{yymm}{số thứ tự}`.
Số thứ tự lấy từ một `SEQUENCE` riêng của Postgres — đây là ngoại lệ duy nhất được
dùng sequence, và nó **không phải khóa chính**, chỉ là nhãn hiển thị.

API tra cứu công khai (tra đơn, tra bảo hành) dùng `code`; API nội bộ dùng UUID.

---

## 2. Mô hình lỗi

### 2.1. Định dạng — RFC 7807 rút gọn

`Content-Type: application/problem+json`

```json
{
  "type":       "/errors/product-not-found",
  "title":      "Không tìm thấy sản phẩm",
  "status":     404,
  "code":       "PRODUCT_NOT_FOUND",
  "request_id": "01937f3e-8a2c-7c1e-9f3b-2d4e5a6b7c8d"
}
```

Lỗi validate có thêm `errors`:

```json
{
  "type": "/errors/validation-failed",
  "title": "Dữ liệu không hợp lệ",
  "status": 422,
  "code": "VALIDATION_FAILED",
  "request_id": "0193...",
  "errors": [
    { "field": "price", "code": "REQUIRED", "message": "Giá là bắt buộc" },
    { "field": "sku",   "code": "TOO_LONG", "message": "SKU tối đa 64 ký tự" }
  ]
}
```

**`code` là hợp đồng, `title` là để người đọc.** Frontend map `code` sang
thông điệp tiếng Việt của nó; không bao giờ so sánh chuỗi `title`. `code` một khi
đã công bố thì không đổi.

`request_id` bắt buộc có ở mọi response lỗi — người dùng báo lỗi thì chỉ cần đọc
mã này là tra được log.

### 2.2. Bảng mã HTTP

| Tình huống | Mã | `code` ví dụ |
|---|---|---|
| JSON hỏng, sai kiểu, thiếu tham số bắt buộc | 400 | `MALFORMED_REQUEST` |
| Chưa đăng nhập / token hết hạn | 401 | `UNAUTHENTICATED` |
| Đã đăng nhập nhưng không đủ quyền | 403 | `FORBIDDEN` |
| Không tìm thấy tài nguyên | 404 | `PRODUCT_NOT_FOUND` |
| Xung đột trạng thái (SKU trùng, đơn đã hủy) | 409 | `DUPLICATE_SKU` |
| Cú pháp đúng nhưng vi phạm quy tắc nghiệp vụ | 422 | `VALIDATION_FAILED`, `INSUFFICIENT_STOCK` |
| Vượt rate limit | 429 | `RATE_LIMITED` (kèm header `Retry-After`) |
| Lỗi không lường trước | 500 | `INTERNAL_ERROR` |
| Phụ thuộc bên ngoài chết (DB, cổng thanh toán) | 503 | `SERVICE_UNAVAILABLE` |

Phân biệt 400 và 422: **400 = tôi không hiểu request, 422 = tôi hiểu nhưng nó sai.**

### 2.3. Cài đặt trong Go

Package kernel `platform/errs`, **không phụ thuộc gì ngoài stdlib**, nên `domain`
được phép import (xem nới lỏng quy tắc 3.1 ở README).

```go
// internal/platform/errs/errs.go
package errs

type Kind uint8

const (
    KindInternal Kind = iota   // giá trị 0 = an toàn nhất khi quên gán
    KindInvalid
    KindUnauthenticated
    KindForbidden
    KindNotFound
    KindConflict
    KindValidation
    KindRateLimited
    KindUnavailable
)

type Error struct {
    Kind    Kind `json:"-"`    // không bao giờ serialize giá trị iota này
    Code    string             // ổn định, dùng cho client: "PRODUCT_NOT_FOUND"
    Message string             // tiếng Việt, hiển thị được cho người dùng
    Fields  []FieldError       // chỉ dùng cho KindValidation
    cause   error              // lỗi gốc, không xuất khẩu → không thể marshal
}

func (e *Error) Unwrap() error { return e.cause }

// Error() CÓ kèm cause — chuỗi này dành cho log, không dành cho response.
func (e *Error) Error() string {
    if e.cause != nil {
        return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Is so cả Code VÀ Kind. Chỉ so Code là không đủ: một lỗi Wrap nhầm Kind sẽ
// vẫn khớp sentinel của domain, khiến errors.Is báo đúng trong khi client
// nhận sai mã HTTP.
func (e *Error) Is(target error) bool {
    t, ok := target.(*Error)
    return ok && e.Code == t.Code && e.Kind == t.Kind
}
```

🚫 **Quy tắc cho người viết handler: không bao giờ đưa `err.Error()` vào response body.**

`Error()` cố ý kèm `cause` để `slog` ghi được nguyên nhân gốc. Chuỗi đó trông như:

```
SKU_DUPLICATE: SKU đã tồn tại: pq: duplicate key value violates unique constraint "products_sku_key"
```

Lộ ra ngoài là lộ tên bảng, tên cột, tên ràng buộc. Hai lớp bảo vệ:

1. `cause` **không xuất khẩu**, nên `json.Marshal(*Error)` không thể chạm tới nó.
2. `WriteError` dựng `Problem` bằng từng trường cụ thể, không marshal thẳng `*Error`.

Cả hai đều là bảo vệ về mặt cấu trúc. Còn lại hai đường hở mà code không thể tự
chặn, vì chúng nằm ở phía người gọi:

1. Tự viết `http.Error(w, err.Error(), 500)` — đừng làm vậy, luôn `return err` để
   `httpx.Wrap` xử lý.
2. **Nhét lỗi gốc vào `Message` hoặc `FieldError.Message`.** `Message` được thiết
   kế để hiển thị cho người dùng, nên nó ra thẳng `title` trong response. Viết
   `errs.Wrap(err, ..., fmt.Sprintf("Không lưu được: %v", err))` là tự tay dán tên
   bảng và tên ràng buộc vào body. Message phải là câu tiếng Việt viết sẵn, không
   bao giờ nội suy từ lỗi.

`KindInternal` để ở vị trí 0 là có chủ đích: quên gán `Kind` thì mặc định thành
500, không phải 200 hay 404.

Domain khai báo lỗi của mình:

```go
// internal/catalog/domain/errors.go
var ErrProductNotFound = &errs.Error{
    Kind: errs.KindNotFound, Code: "PRODUCT_NOT_FOUND",
    Message: "Không tìm thấy sản phẩm",
}
```

Tầng HTTP map một chỗ duy nhất:

```go
// internal/platform/httpx/error.go
func writeError(w http.ResponseWriter, r *http.Request, err error) {
    var e *errs.Error
    if !errors.As(err, &e) {
        e = &errs.Error{Kind: errs.KindInternal, Code: "INTERNAL_ERROR",
                        Message: "Đã có lỗi xảy ra"}
    }

    status := statusOf(e.Kind)
    if status >= 500 {
        slog.ErrorContext(r.Context(), "request failed", "err", err)  // log lỗi gốc
        sentry.CaptureException(err)
    }
    // KHÔNG bao giờ đưa e.cause hay err.Error() vào response
    writeProblem(w, r, status, e)
}
```

**Không bao giờ để lỗi gốc lọt ra response.** `pq: duplicate key value violates
unique constraint "products_sku_key"` làm lộ tên bảng, tên cột, cấu trúc DB.

### 2.4. Danh sách mã lỗi

Mọi `code` phải được liệt kê trong `api/openapi.yaml` (dạng enum) và có bản dịch
tương ứng ở `apps/web/lib/errors.ts`. CI kiểm hai bên khớp nhau.

---

## 3. Phân trang

Hai kiểu, dùng cho hai mục đích khác nhau. **Không dùng một kiểu cho tất cả.**

### 3.1. Offset — cho danh sách công khai

Dùng ở: trang danh mục, kết quả tìm kiếm, danh sách admin.

Lý do: người mua hàng nhảy trang (`?page=5`), và Google cần URL trang 2, 3 để
index. Cursor không làm được hai việc này.

```
GET /api/v1/products?category=laptop-gaming&page=2&limit=24&sort=price_asc
```

```json
{
  "data": [ ... ],
  "meta": {
    "page": 2, "limit": 24,
    "total": 1523, "total_pages": 64,
    "has_next": true, "has_prev": true
  }
}
```

Quy tắc:
- `limit` mặc định 24, **tối đa 100**. Vượt → 400.
- **Chặn offset sâu**: `page > 200` → 400 `PAGE_TOO_DEEP`. Postgres phải quét và
  vứt bỏ `offset` dòng, nên trang 5000 sẽ giết database. Google cũng không index
  sâu vậy. Bot cào giá thì có.
- `COUNT(*)` với bộ lọc là phép đếm đắt. Với danh mục lớn: cache số đếm ở Redis
  (TTL 5 phút), hoặc dùng ước lượng từ `EXPLAIN` khi > 10.000 dòng và hiển thị
  "khoảng 1.500 sản phẩm".
- `sort` là enum đóng (`price_asc`, `price_desc`, `newest`, `bestseller`), **không**
  nhận tên cột tự do — đó là lỗ hổng injection và là cách dễ nhất để tạo query
  không có index.

### 3.2. Cursor — cho danh sách nội bộ và cuộn vô hạn

Dùng ở: đơn hàng của tôi, lịch sử giao dịch, export admin, danh sách trong app.

```
GET /api/v1/orders?limit=20&after=eyJpZCI6IjAxOTM...
```

```json
{
  "data": [ ... ],
  "meta": { "next_cursor": "eyJpZCI6...", "has_more": true }
}
```

- Cursor là base64 của `{"created_at": "...", "id": "..."}` — **phải có tie-breaker**
  là khóa chính, nếu không sẽ nhảy/lặp bản ghi khi trùng `created_at`.
- Query dạng keyset, chạy được với index:
  ```sql
  WHERE (created_at, id) < ($1, $2) ORDER BY created_at DESC, id DESC LIMIT $3
  ```
- Không có tổng số. Đó là chủ đích — đếm là phần đắt nhất.
- Cursor **không** được mã hóa thông tin nhạy cảm; nó chỉ là base64, không phải mã hóa.

---

## 4. Quy ước response chung

- Danh sách **luôn** trả `{ "data": [...], "meta": {...} }`, không trả mảng trần.
  Trả mảng trần thì sau này thêm `meta` là breaking change.
- Chi tiết một tài nguyên trả thẳng object, không bọc `data`.
- Mọi response (kể cả lỗi) có header `X-Request-Id`.
- Endpoint ghi (POST/PUT/PATCH/DELETE) nhận header **`Idempotency-Key`**; lưu
  key + response ở Redis 24h, gọi lại cùng key trả về đúng response cũ. Bắt buộc
  với thanh toán và tạo đơn.
- `POST` tạo thành công → **201** kèm header `Location`.
- `DELETE` thành công → **204**, không body.
- Thao tác dài (xuất báo cáo) → **202** kèm `job_id` để hỏi trạng thái.

---

## 5. Quy trình OpenAPI

```
sửa api/openapi.yaml  →  make openapi  →  sinh lại type TS  →  code cả hai đầu
```

`api/openapi.yaml` là **nguồn sự thật**. Backend viết handler tay (để học), nhưng
được kiểm bằng hai lớp:

**Lớp 1 — CI kiểm drift.** Sinh lại client TS, nếu `git diff` khác rỗng thì fail.
Nghĩa là quên chạy `make openapi` sau khi sửa spec sẽ bị chặn.

**Lớp 2 — Contract test.** Dùng `kin-openapi` nạp spec, mỗi test handler sẽ
validate response thật có khớp schema không:

```go
func assertMatchesSpec(t *testing.T, req *http.Request, res *http.Response) {
    route, params, err := router.FindRoute(req)
    require.NoError(t, err)
    err = openapi3filter.ValidateResponse(ctx, &openapi3filter.ResponseValidationInput{
        RequestValidationInput: &openapi3filter.RequestValidationInput{
            Request: req, PathParams: params, Route: route,
        },
        Status: res.StatusCode, Header: res.Header, Body: res.Body,
    })
    require.NoError(t, err)
}
```

Nhờ hai lớp này, viết handler tay vẫn an toàn như sinh code, mà không mất phần
đang muốn học.

### 5.1. Tổ chức file spec

Một file `openapi.yaml` cho 15 module sẽ thành vài nghìn dòng. Tách:

```
api/
├── openapi.yaml              # gốc, chỉ chứa $ref
├── paths/
│   ├── products.yaml
│   └── orders.yaml
└── components/
    ├── schemas/
    ├── responses/            # problem+json, các lỗi chuẩn
    └── parameters/           # page, limit, sort, cursor
```

`make openapi` chạy `redocly bundle` gộp lại rồi mới sinh code.

---

## 6. CORS, rate limit, bảo mật

- **CORS**: danh sách origin lấy từ env, **không** dùng `*` khi có cookie.
- **Rate limit** (token bucket ở Redis, xem tài liệu 03):

  | Nhóm | Giới hạn |
  |---|---|
  | API đọc công khai | 120 req/phút/IP |
  | Đăng nhập, quên mật khẩu, OTP | 5 req/phút/IP **và** /tài khoản |
  | Tạo đơn, thanh toán | 10 req/phút/user |
  | Webhook từ đối tác | không giới hạn theo IP, nhưng **bắt buộc verify chữ ký** |

- Header bảo mật: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`,
  `Strict-Transport-Security`, CSP (đặt ở Caddy/Nginx).
- Giới hạn kích thước body: `http.MaxBytesReader`, mặc định 1 MB (upload đi
  đường riêng qua presigned URL của MinIO).
- Timeout: `ReadHeaderTimeout` 5s, `ReadTimeout` 15s, `WriteTimeout` 30s,
  `IdleTimeout` 60s. Bỏ trống là mở cửa cho Slowloris.

---

## 7. Việc cần làm

- [ ] `platform/errs`: `Kind`, `Error`, constructor, `statusOf`
- [ ] `platform/httpx`: `writeProblem`, `writeError`, `decodeJSON` có giới hạn kích thước
- [ ] `platform/httpx/paging`: parse + validate `page`/`limit`/`sort`/`after`
- [ ] Middleware: RequestID, Logger, Recoverer, Timeout, CORS, RateLimit, BodyLimit
- [ ] Middleware `Idempotency-Key` (lưu Redis 24h)
- [ ] `api/openapi.yaml` + cấu trúc `paths/` `components/`
- [ ] `make openapi`: redocly bundle + openapi-typescript
- [ ] CI job `openapi-drift`
- [ ] Helper contract test bằng `kin-openapi`
- [ ] `apps/web/lib/errors.ts`: map `code` → thông điệp tiếng Việt
- [ ] CI kiểm enum `code` trong spec khớp với `errors.ts`
