# P0.2 — Module `catalog`: Thiết kế

**Mục tiêu:** module nghiệp vụ đầu tiên, chạy xuyên đủ bốn tầng
`domain → app → adapter → HTTP`, chứng minh kiến trúc hexagonal dựng ở P0.1 thật
sự dùng được.

**Trạng thái:** đã chốt qua hỏi đáp, chờ viết kế hoạch triển khai.

---

## 1. Phạm vi

**Có trong P0.2:**
- `categories` (cây nhiều cấp), `brands`, `products`
- Thuộc tính động bằng `JSONB` + GIN index
- API đọc công khai: danh sách có lọc + phân trang, chi tiết theo slug, cây danh mục
- API ghi cho admin: tạo, sửa, đăng bán
- Cache Redis cho chi tiết sản phẩm và cây danh mục
- `api/openapi.yaml` + sinh type TypeScript

**KHÔNG có trong P0.2** — ghi rõ để tránh thiết kế hờ:
- Biến thể / SKU riêng giá — kéo theo tồn kho, thuộc P3
- Bảng media riêng — P0.2 chỉ có `images TEXT[]`
- Tìm kiếm gần đúng (`pg_trgm`) — thuộc P7 Meilisearch
- Xác thực thật (JWT + RBAC) — thuộc P2
- Outbox thật — thuộc P0.3

---

## 2. Bốn quyết định đã chốt

### 2.1. Thuộc tính sản phẩm dùng JSONB, không dùng EAV

Cột `attributes JSONB` + GIN index. Lọc bằng toán tử chứa:
`attributes @> '{"ram":"16GB"}'::jsonb`

Lý do: EAV cần `JOIN` thêm một lần cho **mỗi** thuộc tính lọc, mà mỗi ngành hàng
lại có tập thuộc tính khác nhau. JSONB giữ toàn bộ trong một dòng, đọc một lần.

Đánh đổi đã chấp nhận: không có ràng buộc kiểu ở tầng database — `{"ram": 16}` và
`{"ram": "16GB"}` đều lưu được. Việc validate phải nằm ở `domain`.

### 2.2. Cây danh mục: adjacency list + cache cây

`categories.parent_id` tự tham chiếu. Đọc toàn bộ cây bằng recursive CTE **một
lần**, cache ở Redis 6 giờ. Truy vấn "sản phẩm trong danh mục X và mọi nhánh con"
giải ra ID con cháu **trong Go từ cây đã cache**, rồi query phẳng
`WHERE category_id = ANY($1)`.

Đã loại:
- **ltree** — phải cập nhật cột `path` cho toàn bộ cây con mỗi lần di chuyển danh
  mục; sai thì sản phẩm biến mất khỏi danh mục mà không báo lỗi
- **Closure table** — ghi phức tạp, chỉ đáng cho cây rất lớn và rất sâu

Cây danh mục ở đây chỉ vài trăm node, sâu 3–4 cấp, gần như không đổi nhưng bị đọc
liên tục. Cache cây là thứ đã có trong kế hoạch; tận dụng nó biến bài toán cây
thành một truy vấn phẳng có index thường.

### 2.3. API ghi bảo vệ bằng khóa chia sẻ, bắt buộc có

Middleware đọc header `X-Admin-Key`, so với `ADMIN_API_KEY` bằng
`subtle.ConstantTimeCompare`. `config.Load()` **bắt buộc** có biến này — thiếu thì
server không khởi động, nên không thể vô tình deploy mà quên bật.

Đây là giải pháp tạm, xóa bỏ khi P2 có JWT + RBAC. Ghi rõ điều đó trong code.

### 2.4. Sự kiện: định nghĩa port ngay, cài tạm bản ghi log

`domain` entity `raise()` sự kiện; `app` khai báo port `EventPublisher`; P0.2 cài
một adapter chỉ ghi log. P0.3 thay bằng adapter outbox **mà không phải sửa `domain`
hay `app`** — đây chính là chỗ hexagonal trả công, và cũng là cách kiểm chứng kiến
trúc đang hoạt động chứ không chỉ đẹp trên giấy.

---

## 3. Schema

```sql
categories (
  id UUID PRIMARY KEY,                            -- UUIDv7 sinh ở domain
  parent_id UUID REFERENCES categories(id) ON DELETE RESTRICT,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  position INT NOT NULL DEFAULT 0,                -- thứ tự hiển thị cùng cấp
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
)

brands (
  id UUID PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
)

products (
  id UUID PRIMARY KEY,
  sku TEXT NOT NULL,
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  short_description TEXT NOT NULL DEFAULT '',
  category_id UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
  brand_id   UUID NOT NULL REFERENCES brands(id)     ON DELETE RESTRICT,
  price NUMERIC(15,2) NOT NULL CHECK (price >= 0),
  currency TEXT NOT NULL DEFAULT 'VND',
  status TEXT NOT NULL CHECK (status IN ('draft','live','archived')),
  attributes JSONB NOT NULL DEFAULT '{}',
  images TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ
)
```

Index:

```sql
CREATE UNIQUE INDEX products_slug_uq ON products (slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX products_sku_uq  ON products (sku)  WHERE deleted_at IS NULL;

CREATE INDEX products_category_live_idx ON products (category_id, price)
  WHERE deleted_at IS NULL AND status = 'live';

CREATE INDEX products_attrs_idx ON products USING gin (attributes);
CREATE INDEX categories_parent_idx ON categories (parent_id);
```

### Bốn chi tiết có chủ đích

**Partial unique index cho `slug` và `sku`**, không phải `UNIQUE` thường. Ràng buộc
`UNIQUE` thường khóa luôn slug của sản phẩm **đã xóa mềm**, nghĩa là xóa xong không
tạo lại được sản phẩm cùng slug. Đây là lỗi rất hay gặp khi ghép soft delete với
unique constraint.

**`ON DELETE RESTRICT`, không `CASCADE`.** Xóa một danh mục còn sản phẩm phải báo
lỗi, không được im lặng xóa theo hàng nghìn sản phẩm.

**`NUMERIC(15,2)` cho tiền**, không bao giờ `FLOAT`. Kèm `currency` ngay từ đầu
theo thiết kế 02 mục 1.

**`status` là `TEXT` + `CHECK`, không phải `ENUM` của Postgres.** Thêm giá trị vào
ENUM phải `ALTER TYPE`, không lùi lại được, và vướng khi chạy trong transaction.

---

## 4. Cấu trúc module

```
internal/catalog/
├── domain/                    # chỉ stdlib + platform/errs + uuid + decimal
│   ├── product.go             # entity, NewProduct, Update, Publish
│   ├── category.go            # Category + Tree.DescendantIDs
│   ├── brand.go
│   ├── money.go               # value object bọc decimal + currency
│   ├── slug.go                # chuẩn hóa tiếng Việt
│   ├── events.go              # ProductCreated / Updated / Published
│   └── errors.go              # sentinel error
├── app/
│   ├── ports.go               # 5 port
│   ├── create_product.go
│   ├── update_product.go
│   ├── publish_product.go
│   ├── get_product.go
│   ├── list_products.go
│   └── get_category_tree.go
├── adapter/
│   ├── httpapi/               # handler + DTO + routes + middleware X-Admin-Key
│   ├── pgstore/               # queries/*.sql + sqlc gen + squirrel cho List
│   ├── rediscache/
│   └── logpublisher/          # tạm, P0.3 thay
└── module.go                  # lắp ráp, expose Routes()
```

Thêm ở tầng platform: `internal/platform/redis` — client dùng chung, timeout ngắn
(`DialTimeout` 200ms, `ReadTimeout` 100ms), **mọi lỗi Redis bị nuốt** và rơi xuống
đọc thẳng Postgres. Redis chết thì hệ thống chậm đi, không được sập.

### Năm port trong `app/ports.go`

| Port | Dùng cho |
|---|---|
| `ProductRepository` | `Save`, `ByID`, `BySlug`, `List` |
| `CategoryRepository` | `All` (dựng cây), `BySlug` |
| `Cache` | `GetOrLoad`, `Delete` |
| `EventPublisher` | `Publish(ctx, ...domain.Event)` |
| `TxManager` | `Run(ctx, fn)` |

**Không có `BrandRepository`** — đây là chủ đích, không phải bỏ sót. `brand_id` được
kiểm bằng khóa ngoại của Postgres (`NOT NULL REFERENCES brands(id)`). Gửi
`brand_id` không tồn tại sẽ sinh lỗi FK, và `pgstore` map nó thành
`ErrBrandNotFound` (422). Thêm một port chỉ để `SELECT 1 FROM brands` là thừa một
vòng round-trip và vẫn có khoảng trống tranh chấp giữa lúc kiểm và lúc ghi — khóa
ngoại làm việc đó đúng hơn và nguyên tử.

Cùng lý do với `category_id`.

---

## 5. Quy tắc nghiệp vụ nằm ở domain

| Quy tắc | Sentinel error | Kind |
|---|---|---|
| SKU rỗng hoặc dài quá 64 ký tự | `ErrInvalidSKU` | Validation |
| Tên rỗng | `ErrNameRequired` | Validation |
| Giá âm | `ErrInvalidPrice` | Validation |
| Slug không hợp lệ sau khi chuẩn hóa | `ErrInvalidSlug` | Validation |
| `Publish()` khi chưa có ảnh nào | `ErrNoImage` | Validation |
| `Publish()` khi giá bằng 0 | `ErrPriceRequired` | Validation |
| `Publish()` khi đã `live` | `ErrAlreadyPublished` | Conflict |
| Không tìm thấy sản phẩm | `ErrProductNotFound` | NotFound |
| SKU đã tồn tại | `ErrDuplicateSKU` | Conflict |
| `brand_id` hoặc `category_id` không tồn tại | `ErrBrandNotFound` / `ErrCategoryNotFound` | Validation |

Mọi sentinel mang `errs.Kind` và `Code` ổn định, để `httpx.WriteError` map sang
HTTP mà không cần biết gì về catalog.

⚠️ `Message` phải là câu tiếng Việt viết sẵn, **không bao giờ nội suy từ lỗi gốc**
— nó ra thẳng `title` của response và sẽ làm lộ tên bảng, tên ràng buộc.

---

## 6. Luồng dữ liệu

**Ghi — `CreateProduct`:**

```
handler → decode DTO → use case
   → domain.NewProduct(...)              validate, sinh UUIDv7, raise event
   → tx.Run:  repo.Save + events.Publish       ← CÙNG transaction
   → sau khi commit: cache.Delete(slug)
   → 201 + Location
```

Xóa cache **sau** commit, không phải trong transaction: rollback mà đã xóa cache
thì chỉ tốn một lần đọc lại; xóa trước rồi commit lỗi thì cache có thể được nạp
lại bằng dữ liệu chưa commit.

**Đọc — `GetProductBySlug`:** cache-aside qua `GetOrLoad[T]`, kèm `singleflight`
để 1000 request cùng trượt cache chỉ gọi DB một lần, và jitter TTL để không phải
mọi key hết hạn cùng một lúc.

**Đọc — `ListProducts`:** lấy cây danh mục từ cache → `DescendantIDs` → dựng câu
SQL bằng squirrel → `WHERE category_id = ANY($1) AND ...`. Kết quả trang danh mục
**không cache** ở P0.2; chỉ cache chi tiết sản phẩm và cây danh mục.

**sqlc hay squirrel:** sqlc cho query tĩnh (`Save`, `ByID`, `BySlug`, `AllCategories`);
squirrel cho `List` vì filter động nhiều điều kiện tùy chọn là đúng giới hạn của
sqlc. Tuyệt đối không nối chuỗi SQL bằng `fmt.Sprintf` — squirrel tự tham số hóa.

---

## 7. API

```
GET   /api/v1/products?category=&brand=&price_min=&price_max=&attr.<key>=&page=&limit=&sort=
GET   /api/v1/products/{slug}
GET   /api/v1/categories

POST  /api/v1/admin/products                -> 201 + Location   | cần X-Admin-Key
PATCH /api/v1/admin/products/{id}           -> 200              | cần X-Admin-Key
POST  /api/v1/admin/products/{id}/publish   -> 200              | cần X-Admin-Key
```

- Phân trang **offset**, trả `{ data, meta: { page, limit, total, total_pages, has_next, has_prev } }`
- `limit` mặc định 24, tối đa 100. `page > 200` → 400 `PAGE_TOO_DEEP`
- `sort` là **enum đóng**: `price_asc`, `price_desc`, `newest`. Không nhận tên cột
  tự do — vừa là lỗ hổng injection, vừa là cách chắc chắn tạo ra query không dùng index
- Lọc thuộc tính qua tiền tố `attr.`: `?attr.ram=16GB&attr.socket=AM5`
- Tiền trả về dạng **chuỗi** kèm `currency`; thời gian RFC 3339 UTC
- Mọi response lỗi theo RFC 7807 với `code` ổn định và `request_id`

---

## 8. Kiểm chứng thủ công

Dự án không dùng unit test. Mười mục dưới đây thay thế, không được bỏ mục nào.

| # | Kiểm | Kỳ vọng |
|---|---|---|
| 1 | `task check` | build + vet + lint + arch đều xanh |
| 2 | Tạo sản phẩm thiếu `X-Admin-Key` | 401 `UNAUTHENTICATED` |
| 3 | Tạo sản phẩm sai khóa | 401; xác nhận code dùng `subtle.ConstantTimeCompare` |
| 4 | Tạo sản phẩm hợp lệ | 201 + `Location`; log có dòng event `product.created` |
| 5 | Tạo trùng SKU | 409 `DUPLICATE_SKU`, **không** lộ tên ràng buộc Postgres |
| 6 | `Publish` sản phẩm chưa có ảnh | 422 `NO_IMAGE` |
| 7 | **Cache có thật sự được dùng không** | Bật `log_statement=all`, gọi cùng slug hai lần → chỉ **một** truy vấn xuống DB |
| 8 | **Cache bị xóa sau khi sửa** | `PATCH` xong gọi lại ngay → thấy dữ liệu mới, không phải bản cũ |
| 9 | Lọc theo thuộc tính | `?attr.ram=16GB` trả đúng tập; `EXPLAIN ANALYZE` cho thấy dùng GIN index |
| 10 | Xóa danh mục còn sản phẩm | Báo lỗi, không xóa kèm sản phẩm |

Mục **7 và 8 quan trọng nhất** vì `curl` tuần tự không tự lộ ra: gọi một lần luôn
cho kết quả đúng, chỉ khi đếm số truy vấn mới biết cache có hoạt động hay không.

Mục 1 cũng đáng chú ý: `internal/catalog/domain` là package `domain` đầu tiên của
dự án, nên đây là lần đầu `scripts/check-arch.sh` thật sự có việc để làm.

---

## 9. Ranh giới với các kế hoạch khác

| Với | Ranh giới |
|---|---|
| **P0.3** (outbox) | P0.2 định nghĩa port `EventPublisher` + domain event. P0.3 chỉ viết adapter outbox, không sửa `domain`/`app`. Hai kế hoạch chạy song song được |
| **P2** (identity) | `X-Admin-Key` là tạm. P2 thay bằng JWT + RBAC và xóa middleware này |
| **P3** (tồn kho) | P0.2 **không** có trường tồn kho. Đừng thêm cột `stock` vào `products` — tồn kho là đa kho, thuộc bảng riêng |
| **P7** (tìm kiếm) | P0.2 lọc bằng SQL. Tìm kiếm chữ chuyển sang Meilisearch ở P7 |

### Sửa tài liệu 03 kèm theo

Tài liệu 03 mục 1 hiện ghi "tách hai instance Redis ngay từ P0.2". Sửa thành
**tách ở P4 khi giỏ hàng xuất hiện**: P0.2 chỉ dùng vai trò cache, dựng thêm một
instance chưa ai đọc ghi là thừa. Giữ nguyên cảnh báo rằng cổng 6380 sẽ **đổi ngữ
nghĩa** lúc tách — từ `noeviction` + AOF sang `allkeys-lru` + không persistence —
nên giỏ hàng không được nằm lại trên cổng đó.
