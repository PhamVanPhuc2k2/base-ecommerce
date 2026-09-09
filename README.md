# base-ecommerce

Base dự án thương mại điện tử fullstack (mô hình tham chiếu: hacom.vn) — Next.js + Go.

Tài liệu này là **bản ghi quyết định thiết kế + quy ước code + danh sách việc cần làm**.
Đọc hết mục 3 (Nguyên tắc kiến trúc) trước khi viết dòng code đầu tiên.

**Ký hiệu trong tài liệu:**
- ✅ **Đã chốt** — quyết định đã thống nhất, không tranh luận lại
- ⬜ **Đề xuất** — khuyến nghị nhưng chưa chốt, cần quyết định trước khi code phần đó

---

## Mục lục

1. [Mục tiêu](#1-mục-tiêu)
2. [Stack](#2-stack)
3. [Nguyên tắc kiến trúc](#3-nguyên-tắc-kiến-trúc-luật-bất-di-bất-dịch)
4. [Cấu trúc thư mục](#4-cấu-trúc-thư-mục)
5. [Backend — quy ước theo tầng](#5-backend--quy-ước-theo-tầng)
6. [Backend — tầng dữ liệu](#6-backend--tầng-dữ-liệu-pgx-transaction-outbox)
7. [Frontend — quy ước Next.js](#7-frontend--quy-ước-nextjs)
8. [Hợp đồng API](#8-hợp-đồng-api)
9. [Khuôn mẫu thêm một module mới (fullstack)](#9-khuôn-mẫu-thêm-một-module-mới-fullstack)
10. [Hạ tầng & vận hành](#10-hạ-tầng--vận-hành)
11. [Checklist P0](#11-checklist-p0--nền-móng--lát-cắt-dọc)
12. [Roadmap các dự án con](#12-roadmap-các-dự-án-con)
13. [Công nghệ còn thiếu](#13-công-nghệ-còn-thiếu-sẽ-thêm-theo-giai-đoạn)
14. [Kiến thức PostgreSQL cần nắm](#14-kiến-thức-postgresql-cần-nắm)
15. [Cạm bẫy đã biết trước](#15-cạm-bẫy-đã-biết-trước)
16. [Tài liệu thiết kế chi tiết](#16-tài-liệu-thiết-kế-chi-tiết)

---

## 1. Mục tiêu

Hai mục tiêu song song, khi xung đột thì ưu tiên theo thứ tự này:

1. **Học Go và PostgreSQL cho vững** — không ORM che giấu SQL, không framework che giấu `net/http`.
2. **Base đủ tốt để lớn thành hệ thống thật** — 15+ module nghiệp vụ cắm vào được
   mà không phải đập đi làm lại.

Trình độ Go hiện tại: **mới bắt đầu**. Mọi lớp trừu tượng trong thiết kế này phải
trả lời được câu hỏi "nó giải quyết vấn đề gì" — nếu không, bỏ.

---

## 2. Stack

### 2.1. Backend ✅

| Hạng mục | Công nghệ | Ghi chú |
|---|---|---|
| Ngôn ngữ | **Go** (1.22+) | `http.ServeMux` chuẩn đã hỗ trợ method + path param |
| Router | **Chi** | Router mỏng trên `net/http`, không che giấu stdlib |
| Database | **PostgreSQL** | Source of truth |
| Driver | **pgx/v5** (`pgxpool`) | Không dùng chế độ `database/sql` |
| Sinh code SQL | **sqlc** | Viết SQL thật, sinh Go type-safe. **Không GORM** |
| Query động | **squirrel** | Chỉ cho bộ lọc nhiều điều kiện tùy chọn |
| Migration | **goose** | Hỗ trợ cả migration viết bằng Go (cần cho backfill) |
| Cache / session | **Redis** | Cache catalog, giỏ hàng khách vãng lai, rate limit, lock |
| Message queue | **RabbitMQ** | Async job, **luôn** đi kèm outbox pattern |
| Log | **`log/slog`** (stdlib) | JSON ra stdout |
| Validate | ⬜ `go-playground/validator` | Validate DTO ở tầng handler |
| Test | **testify** + **testcontainers-go** | Test với Postgres/Redis/Rabbit thật |
| Lint | ⬜ `golangci-lint` | |

### 2.2. Frontend ✅

Chi tiết: [`docs/design/06-frontend.md`](docs/design/06-frontend.md)

| Hạng mục | Công nghệ | Ghi chú |
|---|---|---|
| Framework | **Next.js App Router** | SSR/ISR cho SEO — nguồn traffic chính của e-commerce |
| Ngôn ngữ | **TypeScript** `strict: true` | |
| CSS | **Tailwind CSS** | |
| Component | **shadcn/ui** | Copy vào repo, sở hữu code, không bị khóa version |
| Icon | **lucide-react** | |
| Data phía server | Server Component + `fetch` | Mặc định cho mọi thứ cần SEO |
| Data phía client | **TanStack Query** | Chỉ giỏ hàng, tài khoản, admin |
| State trên URL | **nuqs** | Bộ lọc/phân trang phải ở URL để chia sẻ link + SEO |
| State toàn cục | **Zustand** | Chỉ giỏ hàng + UI state, không giữ dữ liệu server |
| Form | **react-hook-form + zod** | |
| API client | **openapi-typescript** | Sinh type từ `api/openapi.yaml` |
| Ảnh | `next/image` + custom loader → imgproxy | |
| Format/lint | **Biome** | Một công cụ thay ESLint + Prettier |
| Test | **Vitest** (unit) + **Playwright** (E2E) | |

**Không dùng:** Redux · axios · CSS-in-JS runtime · MUI/Antd.

### 2.3. Hạ tầng ✅

| Hạng mục | Công nghệ |
|---|---|
| Đóng gói | Docker (multi-stage build) |
| Chạy dev | Docker Compose |
| Triển khai | VPS + Docker Compose, code viết theo 12-factor để lên K8s sau không phải sửa |
| CI | ⬜ GitHub Actions |

### 2.4. Đã cân nhắc và loại bỏ

- **Fiber** — chạy trên `fasthttp`, không tương thích `net/http`. Dự án phải tích hợp
  nhiều SDK bên thứ ba (VNPay, MoMo, GHN, OpenTelemetry) nên đây là rủi ro thật,
  không phải lý thuyết. Thêm nữa: không có HTTP/2, và `c.Params()`/`c.Body()` trả về
  bộ nhớ tái sử dụng → hỏng dữ liệu nếu đưa vào goroutine.
- **GORM** — giấu SQL, đi ngược mục tiêu học.
- **lib/pq** — đã ở chế độ bảo trì, không dùng cho dự án mới.
- **Microservice ngay từ đầu** — lỗi phổ biến nhất của base e-commerce.
- **oapi-codegen sinh server Go** — che mất tầng routing/binding, đúng cái đang muốn học.

---

## 3. Nguyên tắc kiến trúc (luật bất di bất dịch)

**Kiến trúc:** Modular monolith ở tầng ngoài + **Hexagonal (Ports & Adapters)** bên
trong mỗi module. Một binary, nhiều module có ranh giới rõ ràng.

### 3.1. Chiều phụ thuộc luôn hướng vào trong

```
adapter ──► app ──► domain ──► (chỉ stdlib)
```

- `app/` chỉ import `domain/`.
- `adapter/` import cả hai. Không bao giờ có chiều ngược lại.
- `domain/` chỉ được import **thư viện chuẩn Go và đúng ba package trong danh
  sách trắng** dưới đây. Không `chi`, không `pgx`, không `net/http`, không `redis`.

**Danh sách trắng của `domain/`:**

| Package | Vì sao được phép |
|---|---|
| Thư viện chuẩn Go | |
| `internal/platform/errs` | Package kernel, tự nó không phụ thuộc gì ngoài stdlib |
| `github.com/google/uuid` | Sinh UUIDv7 cho khóa chính, thuần tính toán |
| `github.com/shopspring/decimal` | Kiểu tiền tệ. Dùng `float64` là sai về nghiệp vụ |

Ba package này đều **thuần túy tính toán, không chạm I/O** — đó là tiêu chí duy
nhất để một package được vào danh sách. Muốn thêm gì nữa phải sửa tài liệu này trước.

**Kiểm chứng bằng máy, không bằng tự giác** — đưa vào CI:

```bash
go list -deps ./internal/*/domain | grep -E 'chi|pgx|net/http|redis|amqp' && exit 1
grep -rn "pgxpool.Pool" internal/*/adapter/ && exit 1   # repository phải dùng DBTX
```

### 3.2. Ports đặt trong `app/`, không tạo package `port/` riêng

`app` chính là bên tiêu thụ các port, mà Go quy ước đặt interface ở nơi tiêu thụ.
Vừa đúng hexagonal (core định nghĩa hợp đồng) vừa đúng văn hóa Go.

### 3.3. Một port cho một hệ thống bên ngoài — không phải một port cho mỗi hàm

Mỗi module thường chỉ cần 2–3 port: `Repository`, `Cache`, `EventPublisher`.
Interface có đúng một method và chỉ một cài đặt duy nhất → nhiều khả năng là thừa.

### 3.4. Đúng HAI lần mapping, không phải ba

```
sqlc struct ──[pgstore]──► domain entity ──[httpapi]──► DTO (khớp OpenAPI)
```

Không tạo thêm "persistence model" tách rời domain. DTO ở tầng HTTP là bắt buộc
vì hợp đồng OpenAPI phải ổn định độc lập với domain.

### 3.5. Domain phải có thịt — chống "anemic domain"

Nếu entity chỉ có getter/setter và mọi logic dồn vào service thì toàn bộ kiến trúc
này vô nghĩa. Quy tắc nghiệp vụ **phải** nằm trong domain.

```go
func (p *Product) Publish() error {
    if len(p.Images) == 0     { return ErrNoImage }
    if p.Price.IsZero()       { return ErrPriceRequired }
    if p.Status == StatusLive { return ErrAlreadyPublished }
    p.Status = StatusLive
    p.raise(ProductPublished{ID: p.ID})
    return nil
}
```

### 3.6. Module chỉ nói chuyện với nhau qua hàm public của `app`

Module `order` **không được** đọc bảng của `catalog`, không import `catalog/adapter/pgstore`.
Chỉ gọi `catalog/app`. Đây là điều kiện để sau này tách microservice nếu cần.

### 3.7. Không nối chuỗi SQL

Luôn dùng tham số `$1, $2`. Query động dùng squirrel (tự tham số hóa).
`fmt.Sprintf` vào câu SQL là lỗi nghiêm trọng, không có ngoại lệ.

### 3.8. 12-factor ngay từ đầu

Config qua biến môi trường · log JSON ra stdout · không ghi state vào đĩa local ·
`/healthz` + `/readyz` · graceful shutdown. Làm đúng thì chuyển sang K8s sau này
không phải sửa code.

---

## 4. Cấu trúc thư mục

```
base-ecommerce/
├── api/
│   └── openapi.yaml                  # NGUỒN SỰ THẬT của hợp đồng API
├── apps/
│   ├── api/                                  # ===== Go backend =====
│   │   ├── cmd/
│   │   │   ├── api/main.go                   # HTTP server
│   │   │   ├── worker/main.go                # RabbitMQ consumer
│   │   │   └── outboxrelay/main.go           # đẩy outbox → RabbitMQ
│   │   ├── internal/
│   │   │   ├── platform/                     # hạ tầng dùng chung, KHÔNG chứa nghiệp vụ
│   │   │   │   ├── config/                   # đọc env, validate lúc khởi động
│   │   │   │   ├── httpx/                    # Wrap(), decode, respond, writeError, paging
│   │   │   │   ├── postgres/                 # pgxpool, DBTX, txmanager
│   │   │   │   ├── redis/
│   │   │   │   ├── rabbitmq/                 # publisher, consumer, retry, DLQ
│   │   │   │   ├── observability/            # slog JSON, OTel, /healthz /readyz
│   │   │   │   └── validate/
│   │   │   │
│   │   │   ├── catalog/                      # MODULE = một hexagon hoàn chỉnh
│   │   │   │   ├── domain/
│   │   │   │   │   ├── product.go            # entity + quy tắc nghiệp vụ
│   │   │   │   │   ├── money.go              # value object (bọc NUMERIC)
│   │   │   │   │   ├── slug.go
│   │   │   │   │   ├── events.go             # ProductPublished, ProductUpdated...
│   │   │   │   │   └── errors.go             # ErrProductNotFound, ErrDuplicateSKU...
│   │   │   │   ├── app/
│   │   │   │   │   ├── ports.go              # Repository, Cache, EventPublisher
│   │   │   │   │   ├── create_product.go     # use case
│   │   │   │   │   ├── get_product.go
│   │   │   │   │   └── list_products.go
│   │   │   │   ├── adapter/
│   │   │   │   │   ├── httpapi/              # driving: Chi handler + DTO
│   │   │   │   │   ├── pgstore/              # driven: sqlc + mapping → domain
│   │   │   │   │   │   ├── queries/*.sql
│   │   │   │   │   │   ├── gen/              # sqlc sinh ra — KHÔNG sửa tay
│   │   │   │   │   │   └── repository.go     # cài đặt app.ProductRepository
│   │   │   │   │   └── rediscache/
│   │   │   │   └── module.go                 # lắp ráp module, expose Routes()
│   │   │   │
│   │   │   ├── outbox/                       # module hạ tầng dùng chung
│   │   │   └── server/router.go              # nơi DUY NHẤT ráp module vào Chi
│   │   ├── db/migrations/                    # goose
│   │   ├── sqlc.yaml
│   │   └── Dockerfile
│   │
│   └── web/                                  # ===== Next.js frontend =====
│       ├── app/
│       │   ├── (shop)/                       # route group: storefront
│       │   │   ├── page.tsx                  # trang chủ
│       │   │   ├── [category]/page.tsx       # danh mục + bộ lọc
│       │   │   ├── san-pham/[slug]/page.tsx  # chi tiết sản phẩm
│       │   │   ├── gio-hang/page.tsx
│       │   │   └── thanh-toan/page.tsx
│       │   ├── (account)/                    # tài khoản, đơn hàng
│       │   ├── admin/                        # khu vực quản trị
│       │   ├── sitemap.ts  robots.ts
│       │   └── layout.tsx
│       ├── components/
│       │   ├── ui/                           # shadcn/ui primitives
│       │   └── product/  cart/  layout/      # component theo nghiệp vụ
│       ├── lib/
│       │   ├── api/
│       │   │   ├── generated/                # sinh từ openapi.yaml — KHÔNG sửa tay
│       │   │   └── client.ts                 # wrapper: base URL, auth, xử lý lỗi
│       │   ├── format.ts                     # tiền tệ VND, ngày giờ
│       │   └── seo.ts                        # JSON-LD helper
│       ├── e2e/                              # Playwright
│       └── Dockerfile
├── deploy/
│   ├── compose.dev.yml
│   ├── compose.prod.yml
│   └── nginx/ hoặc caddy/
├── docs/
├── .github/workflows/
├── .env.example
└── Taskfile.yml                      # go-task, thay Makefile (chạy được trên Windows)
```

---

## 5. Backend — quy ước theo tầng

### 5.1. `domain/`

- Chỉ import stdlib + 3 package trong danh sách trắng ở mục 3.1.
- Entity tự bảo vệ tính toàn vẹn. Constructor `NewProduct(...)` trả `(*Product, error)`
  — không cho tạo entity ở trạng thái sai.
- **ID sinh ngay trong constructor** bằng `uuid.NewV7()`, không để Postgres `DEFAULT`.
  Lý do: entity phải có ID trước khi insert, vì domain event tham chiếu tới ID đó
  và được ghi vào outbox trong cùng transaction.
- Tiền tệ: **value object bọc `decimal`/`NUMERIC`**, tuyệt đối không `float64`.
- Thời gian: `time.Time` lưu UTC; DB dùng `timestamptz`, **không** `timestamp`.
- Lỗi nghiệp vụ định nghĩa ở đây bằng sentinel error, để tầng trên dùng `errors.Is`.
- Domain event được entity `raise()`, `app` đọc ra và ghi vào outbox.

### 5.2. `app/` (use case)

- Mỗi use case một file, một struct, một method `Execute`.
- Là nơi **duy nhất** mở transaction. Domain không biết transaction là gì.
- Không import `net/http`, không nhận `*http.Request`.
- Nhận input là struct thuần Go đã validate xong, trả domain entity hoặc lỗi.

### 5.3. `adapter/httpapi/` (driving adapter)

- Handler chỉ làm 3 việc: parse request → gọi use case → ghi response.
- Không logic nghiệp vụ, không truy cập DB.
- DTO định nghĩa ở đây và **phải khớp `api/openapi.yaml`**.
- Pattern handler trả `error` để dồn việc map lỗi về một chỗ:

```go
type Handler func(w http.ResponseWriter, r *http.Request) error

func Wrap(h Handler) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := h(w, r); err != nil {
            writeError(w, r, err)   // map domain error → HTTP status ở MỘT chỗ
        }
    }
}
```

### 5.4. `adapter/pgstore/` (driven adapter)

- SQL tĩnh → `queries/*.sql` → `sqlc generate`.
- SQL động (bộ lọc catalog) → squirrel, viết tay trong `repository.go`.
- Nơi **duy nhất** biết đến sqlc struct. Map sang domain entity trước khi trả ra.

### 5.5. Middleware bắt buộc (thứ tự trong `server/router.go`)

`RequestID` → `RealIP` → `Logger` (slog) → `Recoverer` → `Timeout` → `CORS` → `RateLimit`

---

## 6. Backend — tầng dữ liệu: pgx, transaction, outbox

### 6.1. Luôn dùng `pgxpool.Pool`, không dùng `pgx.Conn`

| | Dùng khi nào |
|---|---|
| `pgx.Conn` | Một kết nối, **không an toàn goroutine**. Chỉ cho CLI/script |
| `pgxpool.Pool` | Server. An toàn goroutine, tự quản lý vòng đời kết nối |

Pool là chi tiết hạ tầng → chỉ tồn tại ở `platform/postgres`, inject vào adapter.
`domain` và `app` không bao giờ nhìn thấy nó.

```go
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
    pc, err := pgxpool.ParseConfig(cfg.DSN)
    if err != nil { return nil, err }

    pc.MaxConns          = cfg.MaxConns
    pc.MinConns          = cfg.MinConns
    pc.MaxConnLifetime   = time.Hour          // ép xoay vòng, tránh conn "thiu"
    pc.MaxConnIdleTime   = 30 * time.Minute
    pc.HealthCheckPeriod = time.Minute

    pool, err := pgxpool.NewWithConfig(ctx, pc)
    if err != nil { return nil, err }
    if err := pool.Ping(ctx); err != nil {    // fail nhanh lúc khởi động
        pool.Close()
        return nil, err
    }
    return pool, nil
}
```

**Kích thước pool — pool to hơn KHÔNG nhanh hơn.** Postgres mỗi kết nối là một
process riêng; vượt quá số core thì thông lượng giảm. Ba binary, ba pool riêng:

```
api          MaxConns = 20
worker       MaxConns = 5
outboxrelay  MaxConns = 3
```

Kiểm tra: `tổng pool × số bản chạy < max_connections` (mặc định 100, trừ 3 cho superuser).

**Khi nào cần `pool.Acquire()`** (ghim một kết nối vì gắn với session):
`LISTEN`/`NOTIFY` · `pg_advisory_lock` cấp session · `SET LOCAL`.
Nhớ `defer conn.Release()` — quên là rò rỉ kết nối, pool cạn dần, mọi request treo.

*(`pg_advisory_xact_lock` dùng chống oversell thì không cần — nó tự nhả khi transaction kết thúc.)*

### 6.2. `DBTX` — chạy được cả trong lẫn ngoài transaction

```go
type DBTX interface {
    Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
    Query(context.Context, string, ...any) (pgx.Rows, error)
    QueryRow(context.Context, string, ...any) pgx.Row
}
// *pgxpool.Pool và pgx.Tx đều thỏa mãn interface này
```

Repository nhận `DBTX`, **không** nhận `*pgxpool.Pool`. Nhờ vậy cùng một repository
dùng được cả hai ngữ cảnh mà không phải viết hai bản.

### 6.3. Ghi sự kiện — luôn qua outbox, không publish trực tiếp

Publish thẳng vào RabbitMQ sau khi commit DB sẽ mất event nếu publish lỗi
(bài toán dual-write). Bắt buộc ghi bảng `outbox` **trong cùng transaction**,
rồi để `outboxrelay` đẩy sang RabbitMQ.

```go
err := tx.Run(ctx, func(ctx context.Context) error {
    if err := repo.Save(ctx, product); err != nil { return err }
    return outbox.Append(ctx, product.Events()...)   // CÙNG transaction
})
```

Consumer phải **idempotent** — RabbitMQ đảm bảo at-least-once, không phải exactly-once.

### 6.4. Cạm bẫy PgBouncer

Nếu đặt PgBouncer chế độ `transaction`, pgx phải chuyển sang:

```go
pc.ConnConfig.DefaultQueryExecMode    = pgx.QueryExecModeExec
pc.ConnConfig.StatementCacheCapacity  = 0
```
Không làm sẽ dính lỗi `prepared statement "lrupsc_1_0" already exists`, rất khó đoán nguyên nhân.

### 6.5. Không dùng chế độ `database/sql` của pgx

`pgx/v5/stdlib` làm mất kiểu native (JSONB, array, `numeric`, `timestamptz`), mất
`CollectRows`, `SendBatch`, `CopyFrom`. Chỉ dùng nếu một thư viện bắt buộc.
Goose chạy tiến trình riêng, không dùng chung pool với server.

---

## 7. Frontend — quy ước Next.js

### 7.1. Server Component là mặc định

`"use client"` chỉ khi thật sự cần (state, event handler, browser API). Đẩy ranh
giới client xuống càng sâu càng tốt — một nút "Thêm vào giỏ" là client component,
cả trang sản phẩm thì không.

### 7.2. Chiến lược render theo loại trang

| Trang | Cách render | Lý do |
|---|---|---|
| Trang chủ, danh mục | ISR (`revalidate`) | SEO + tốc độ, dữ liệu đổi chậm |
| Chi tiết sản phẩm | ISR + on-demand revalidate khi có event `product.updated` | Giá/tồn kho phải mới |
| Giỏ hàng, thanh toán, tài khoản | Client, dynamic | Riêng tư, không cần SEO |
| Admin | Client, dynamic, `noindex` | |

### 7.3. State nằm ở URL, không nằm trong React

Bộ lọc, sắp xếp, phân trang **phải** ở query string. Lý do: chia sẻ được link,
back/forward hoạt động đúng, và Google index được trang danh mục đã lọc.
Chỉ giỏ hàng và UI state (mở/đóng modal) mới dùng store.

### 7.4. SEO — không phải việc làm sau

Đây là nguồn traffic chính của e-commerce, phải làm ngay từ P0:

- `generateMetadata` cho mọi trang: title, description, canonical, OG image
- **JSON-LD** schema.org: `Product` + `Offer` + `AggregateRating`, `BreadcrumbList`,
  `Organization`, `WebSite` (kèm SearchAction)
- `sitemap.ts` sinh động từ API, `robots.ts`
- URL tiếng Việt không dấu, có ý nghĩa: `/laptop-gaming/asus-rog-strix-g16`
- Đúng một `<h1>` mỗi trang

### 7.5. Định dạng dữ liệu

- **Tiền tệ**: backend trả chuỗi decimal (`"25990000"`), frontend format bằng
  `Intl.NumberFormat('vi-VN')`. **Không** truyền số tiền dưới dạng `number` của JS —
  mất chính xác với số lớn.
- **Thời gian**: backend trả ISO 8601 UTC, frontend đổi sang giờ VN khi hiển thị.

### 7.6. Ảnh

`next/image` với custom loader trỏ tới imgproxy. Bắt buộc có `sizes` và `priority`
cho ảnh above-the-fold. Đây là yếu tố ảnh hưởng LCP nhiều nhất trên trang sản phẩm.

### 7.7. Xử lý lỗi

Mỗi route group có `error.tsx` và `loading.tsx`. Lỗi API phải hiện thông báo có
nghĩa cho người dùng, không được im lặng. Mã lỗi từ backend map sang thông điệp
tiếng Việt ở một chỗ duy nhất.

---

## 8. Hợp đồng API

### 8.1. Quy trình ✅

```
Sửa api/openapi.yaml  →  task openapi  →  type TS được sinh lại  →  code cả hai đầu
```

`api/openapi.yaml` là **nguồn sự thật**. Sửa backend mà quên cập nhật spec thì web
sẽ không compile — đó là chủ đích.

Backend viết handler tay (để học), nhưng có test kiểm tra response khớp spec.

### 8.2. Đã chốt ✅

Chi tiết đầy đủ: [`docs/design/02-api-contract.md`](docs/design/02-api-contract.md)

| Hạng mục | Quyết định |
|---|---|
| Version | `/api/v1/` ngay từ đầu |
| Tên trường JSON | `snake_case` (khớp Postgres và sqlc) |
| **Khóa chính** | **UUID v7 cho mọi bảng**, kiểu cột `UUID`, sinh ở tầng `domain` |
| Mã hiển thị cho khách | Cột `code` riêng (`DH26090042`), tách khỏi khóa chính |
| Tiền tệ | Chuỗi decimal + `currency: "VND"`. Không dùng `number` của JS |
| Thời gian | RFC 3339, UTC, hậu tố `Z` |
| Mô hình lỗi | RFC 7807 rút gọn + trường `code` ổn định + `request_id` |
| Phân trang danh sách công khai | **Offset** (`?page=&limit=`) — cần số trang thật cho người dùng và cho SEO. Chặn `page > 200` |
| Phân trang danh sách nội bộ | **Cursor** (`?after=`) — đơn hàng của tôi, export, cuộn vô hạn |

### 8.3. Quy ước cố định

- Danh sách **luôn** trả `{ "data": [...], "meta": {...} }`, không trả mảng trần
- Chi tiết một tài nguyên trả thẳng object, không bọc `data`
- Mọi endpoint ghi (POST/PUT/PATCH/DELETE) nhận header `Idempotency-Key`
- Mọi response có header `X-Request-Id` để tra log
- `POST` tạo thành công → 201 + `Location`; `DELETE` → 204 không body

---

## 9. Khuôn mẫu thêm một module mới (fullstack)

Làm đúng thứ tự này cho mọi module từ P1 trở đi:

**Backend**
1. Viết migration goose trong `db/migrations/`
2. Viết `domain/` trước — entity, value object, quy tắc, lỗi. **Unit test ngay**, không cần DB
3. Khai báo port trong `app/ports.go` — chỉ những gì use case thật sự cần
4. Viết use case trong `app/`, test bằng fake repository (struct đơn giản, không mock framework)
5. Viết `queries/*.sql` → `sqlc generate` → cài đặt repository trong `adapter/pgstore/`.
   Test bằng testcontainers với Postgres thật
6. Cập nhật `api/openapi.yaml` **trước** khi viết handler
7. Viết handler trong `adapter/httpapi/`
8. Lắp ráp trong `module.go`, đăng ký route ở `server/router.go`

**Frontend**
9. `task openapi` sinh lại type TS
10. Viết page/component, ưu tiên Server Component
11. Bổ sung `generateMetadata` + JSON-LD nếu là trang công khai
12. Viết E2E Playwright cho luồng chính

---

## 10. Hạ tầng & vận hành

### 10.1. Dịch vụ trong `compose.dev.yml`

Bảng cổng host đã cấp phát. **Kiểm tra bảng này trước khi thêm dịch vụ mới** —
mọi cổng chỉ bind vào `127.0.0.1`, không mở ra mạng LAN.

| Dịch vụ | Cổng host | Thêm ở giai đoạn |
|---|---|---|
| API (Go) | 8080 | P0 |
| PostgreSQL | 5432 | P0 |
| Redis (một instance) | 6380 | P0 |
| ↳ tách thành `redis-cache` / `redis-data` | 6380 / 6381 | P0.2 — instance 6380 ở trên **đổi vai** thành `redis-cache`, thêm mới 6381. Xem [thiết kế 03](docs/design/03-redis-cache.md) |
| RabbitMQ + management UI | 5672 / 15672 | P0 |
| MinIO | 9000 / 9001 | P1 |
| imgproxy | 8081 | P1 |
| Meilisearch | 7700 | P7 |
| Mailpit (bắt email khi dev) | 8025 | P10 |
| Web (Next.js) | 3000 | P0.4 |

Hai chỗ lệch so với cổng mặc định, đều có lý do:
- **Redis 6380** thay vì 6379 — máy dev đã có một Redis của dự án khác chiếm 6379.
- **imgproxy 8081** thay vì 8080 — 8080 đã dành cho API.

### 10.2. Config

- Toàn bộ qua biến môi trường, `.env.example` luôn cập nhật
- `platform/config` **validate lúc khởi động** — thiếu biến thì fail ngay,
  không fail lúc 3 giờ sáng
- Không commit `.env`. Production dùng ⬜ SOPS (hoặc Vault khi đủ lớn)

### 10.3. Observability

| Mức | Công cụ | Giai đoạn |
|---|---|---|
| Log có cấu trúc (JSON + request ID) | `log/slog` | P0 |
| Health check `/healthz` `/readyz` | tự viết | P0 |
| Error tracking | Sentry | P0 |
| Metrics | Prometheus + Grafana | Giai đoạn 1 |
| Tracing | OpenTelemetry | Giai đoạn 1 |
| Log tập trung | Loki | Giai đoạn 2 |

`/healthz` = tiến trình còn sống. `/readyz` = ping được Postgres, Redis, RabbitMQ.
Hai cái khác nhau, đừng gộp.

### 10.4. CI (GitHub Actions)

- `lint`: golangci-lint + eslint + tsc
- `test`: Go test (có testcontainers) + Vitest
- `arch`: kiểm tra chiều phụ thuộc (mục 3.1)
- `openapi-drift`: sinh lại client TS, nếu git diff khác rỗng → fail
- `build`: build image cho api / worker / outboxrelay / web

### 10.5. Vận hành production

- Reverse proxy: ⬜ Caddy (tự động HTTPS) hoặc Nginx
- CDN + WAF: Cloudflare
- Backup: pgBackRest hoặc wal-g + PITR — **thiết lập trước khi có đơn hàng thật**
- Migration chạy như một job riêng trước khi deploy bản mới, không chạy trong `main()`

---

## 11. Checklist P0 — Nền móng + lát cắt dọc

Mục tiêu: dựng toàn bộ khung kỹ thuật và **chứng minh nó chạy** bằng một luồng
nghiệp vụ thật đi xuyên mọi tầng, thay vì khung xương trên lý thuyết.

> Đây là danh sách rút gọn. Mỗi tài liệu trong `docs/design/` có phần "Việc cần làm"
> chi tiết hơn ở cuối — đọc kèm khi bắt tay vào hạng mục tương ứng.

### Chuẩn bị
- [ ] `git init`, `.gitignore`, `.editorconfig`
- [ ] Cài công cụ: `goose`, `sqlc`, `golangci-lint`, `openapi-typescript`
- [ ] `Taskfile.yml`: `up`, `down`, `migrate`, `sqlc`, `openapi`, `test`, `lint`, `arch`
- [ ] `deploy/compose.dev.yml`: PostgreSQL, Redis, RabbitMQ
- [ ] `.env.example` + `platform/config` đọc env và validate lúc khởi động

### Nền tảng Go
- [ ] `platform/httpx`: `Wrap()`, `decodeJSON`, `respondJSON`, `writeError`, phân trang
- [ ] `platform/postgres`: pgxpool + config, `DBTX`, `txmanager` (Unit of Work)
- [ ] `platform/redis`: client + helper cache có TTL
- [ ] `platform/rabbitmq`: publisher, consumer có retry + DLQ
- [ ] `platform/observability`: slog JSON, request ID, `/healthz`, `/readyz`, Sentry
- [ ] Graceful shutdown cho cả 3 binary (`api`, `worker`, `outboxrelay`)
- [ ] `server/router.go` + middleware theo thứ tự ở mục 5.5

### Module `catalog` (lát cắt dọc)
- [ ] Migration: `categories`, `brands`, `products` (có `attributes JSONB` + GIN index)
- [ ] `domain`: `Product`, `Money`, `Slug`, sentinel errors, `ProductPublished`
- [ ] `app`: use case `CreateProduct`, `GetProductBySlug`, `ListProducts`
- [ ] `pgstore`: sqlc cho query tĩnh, squirrel cho `ListProducts` có filter
- [ ] `rediscache`: cache chi tiết sản phẩm + invalidate khi cập nhật
- [ ] `httpapi`: handler + DTO khớp OpenAPI

### Outbox & worker
- [ ] Bảng `outbox` + `outbox.Append` chạy trong transaction
- [ ] `outboxrelay`: poll outbox → publish RabbitMQ → đánh dấu đã gửi
- [ ] `worker`: consume `product.published`, ghi log (sau này thành indexer Meilisearch)
- [ ] Consumer idempotent (bảng `processed_events` hoặc khóa theo event ID)

### Hợp đồng API
- [ ] Chốt mô hình lỗi, phân trang, quy ước đặt tên JSON (mục 8.2)
- [ ] `api/openapi.yaml` cho các endpoint catalog
- [ ] `task openapi` sinh client TS vào `apps/web/lib/api/generated`

### Frontend
- [ ] Khởi tạo Next.js + TypeScript strict + Tailwind + shadcn/ui
- [ ] `lib/api/client.ts`: base URL, xử lý lỗi, request ID
- [ ] `lib/format.ts`: format VND và ngày giờ
- [ ] Trang danh sách sản phẩm (Server Component, có bộ lọc trên URL)
- [ ] Trang chi tiết sản phẩm (`generateMetadata` + JSON-LD `Product`)
- [ ] `error.tsx` + `loading.tsx`
- [ ] `sitemap.ts` + `robots.ts`

### Test & CI
- [ ] Unit test cho `domain` (không cần Docker)
- [ ] Test use case với fake repository
- [ ] Integration test repository bằng testcontainers-go
- [ ] E2E: tạo sản phẩm → đọc lại → có bản ghi trong outbox
- [ ] Playwright: mở trang danh sách → vào chi tiết
- [ ] GitHub Actions: lint + test + arch + openapi-drift + build

### Tiêu chí hoàn thành P0
`task up && task migrate && task test` chạy xanh; mở trình duyệt thấy trang sản
phẩm lấy dữ liệu từ Go/Postgres thật; tạo một sản phẩm mới thì thấy event xuất
hiện ở RabbitMQ management UI.

---

## 12. Roadmap các dự án con

Mỗi mục là một chu trình riêng: thiết kế → kế hoạch → code.

**Giai đoạn 1 — bán được hàng**

| # | Dự án con | Nội dung |
|---|---|---|
| P0 | Nền móng | Khung kỹ thuật + lát cắt dọc catalog |
| P1 | Catalog & PIM | Danh mục nhiều cấp, biến thể/SKU, thuộc tính động, media, SEO |
| P2 | Identity | Đăng ký/đăng nhập, JWT + refresh rotation, OTP, RBAC, sổ địa chỉ |
| P3 | Inventory & Pricing | Đa kho/chi nhánh, tồn khả dụng vs đã giữ chỗ, bảng giá |
| P4 | Cart & Checkout | Giỏ hàng, reserve tồn kho, tạo đơn |
| P5 | Payment | VNPay/MoMo, webhook idempotent, đối soát |
| P6 | OMS | Vòng đời đơn, hủy/hoàn, màn hình xử lý đơn |

Phụ thuộc: `P0 → P1 → (P2, P3 song song) → P4 → P5 → P6`

**Giai đoạn 2 — giống HACOM**

| # | Dự án con | Nội dung |
|---|---|---|
| P7 | Search & Facet | Meilisearch, lọc theo thuộc tính, đồng bộ qua outbox |
| P8 | Shipping | GHN/GHTK/VTPost, webhook tracking |
| P9 | Promotion Engine | Mã giảm giá, combo, quà tặng kèm |
| P10 | Notification | Email/SMS/Zalo ZNS, template, worker |
| P11 | Hóa đơn điện tử | Viettel S-Invoice / VNPT / MISA |

**Giai đoạn 3 — đặc thù**

CMS/blog · Review có ảnh · Loyalty · **PC Builder** (rule tương thích linh kiện) ·
Trả góp · Recommendation · Live chat (Subiz / Zalo OA)

---

## 13. Công nghệ còn thiếu, sẽ thêm theo giai đoạn

| Hạng mục | Lựa chọn | Thêm ở |
|---|---|---|
| Search engine | **Meilisearch** (hoặc Typesense) | P7 — Postgres full-text không kham nổi facet + tiếng Việt gõ sai |
| Object storage | **MinIO** (S3-compatible) | P1 |
| Xử lý ảnh | **imgproxy** (WebP/AVIF, resize on-the-fly) | P1 |
| Cổng thanh toán | VNPay, MoMo, ZaloPay | P5 |
| Trả góp | Home Credit, HD Saison, Kredivo | Giai đoạn 3 |
| Vận chuyển | GHN, GHTK, Viettel Post, J&T | P8 |
| Hóa đơn điện tử | Viettel S-Invoice / VNPT / MISA | P11 |
| Email / SMS / ZNS | Resend hoặc SES, Zalo ZNS | P10 |
| Bot protection | Cloudflare Turnstile | Giai đoạn 2 |
| Analytics | GA4/GTM, Meta Pixel, product feed Google Merchant | Giai đoạn 2 |
| CDN + WAF | Cloudflare | Khi lên production |
| Secrets | SOPS (hoặc Vault khi cần) | Khi lên production |
| Backup | pgBackRest hoặc wal-g + PITR | Khi lên production |

⚠️ **Lưu ý dữ liệu địa giới hành chính:** từ 01/07/2025 Việt Nam bỏ cấp huyện,
còn tỉnh/xã. Khi làm P8 phải dùng bộ dữ liệu mới và kiểm tra API các hãng vận
chuyển đã cập nhật chưa.

---

## 14. Kiến thức PostgreSQL cần nắm

Đây là phần "học cho vững" thật sự — driver chỉ là công cụ.

**Nền tảng**
- `EXPLAIN (ANALYZE, BUFFERS)` — phân biệt Seq Scan / Index Scan / Bitmap Heap Scan
- B-tree, **partial index** (`WHERE deleted_at IS NULL`), composite index và quy tắc thứ tự cột
- **GIN index** cho `JSONB` (thuộc tính sản phẩm động) và full-text
- `pg_trgm` cho tìm kiếm gần đúng

**Đồng thời — chỗ e-commerce hay chết**
- Isolation level: `READ COMMITTED` vs `REPEATABLE READ` vs `SERIALIZABLE`
- `SELECT ... FOR UPDATE` vs `FOR UPDATE SKIP LOCKED` (hợp cho hàng đợi job)
- Advisory lock (`pg_advisory_xact_lock`) — chống oversell khi flash sale
- Deadlock: luôn khóa theo **cùng một thứ tự** (ví dụ sort theo `product_id`)
- Vì sao `UPDATE stock = stock - 1 WHERE stock > 0` an toàn hơn `SELECT` rồi `UPDATE`

**Mô hình dữ liệu**
- Thuộc tính sản phẩm: `JSONB` vs EAV — ưu nhược từng cái
- `NUMERIC` cho tiền, **không bao giờ** `FLOAT`
- `timestamptz`, lưu UTC
- Soft delete, audit trail, versioning bảng giá

**Vận hành**
- `pgxpool` config: `MaxConns`, `MinConns`, `MaxConnLifetime`; khi nào cần **PgBouncer**
- Partition bảng `orders` theo tháng khi dữ liệu lớn
- Materialized view cho báo cáo doanh thu
- VACUUM / autovacuum, bloat, và tại sao `COUNT(*)` trên bảng lớn lại chậm

---

## 15. Cạm bẫy đã biết trước

1. **Oversell khi flash sale** — không dùng `SELECT` rồi `UPDATE`. Dùng
   `UPDATE ... WHERE stock >= n` hoặc advisory lock.
2. **Giá & khuyến mãi** — HACOM có combo, quà tặng kèm, giá theo chi nhánh, giá B2B.
   Đừng nhét vào một cột `price`; cần bảng price rule riêng ngay từ đầu.
3. **Đa kho** — tồn kho theo chi nhánh và "còn hàng tại showroom nào" là yêu cầu
   bắt buộc, ảnh hưởng schema gốc. Thiết kế từ P3, đừng chắp vá sau.
4. **Webhook thanh toán/vận chuyển** — phải idempotent và verify chữ ký. Nhà cung
   cấp sẽ gửi lại cùng một webhook nhiều lần.
5. **Anemic domain** — cạm bẫy số một của kiến trúc đã chọn. Xem mục 3.5.
6. **Interface phình** — xem mục 3.3.
7. **Rò rỉ kết nối** — quên `defer conn.Release()` sau `pool.Acquire()`.
8. **Dual-write** — publish RabbitMQ ngoài transaction. Luôn qua outbox.
9. **Số tiền thành `number` trong JS** — mất chính xác. Truyền dạng chuỗi.
10. **`"use client"` đặt quá cao** trong cây component — mất hết lợi ích SSR/SEO.

---

## 16. Tài liệu thiết kế chi tiết

README này là bản tóm tắt và mục lục. Chi tiết nằm ở `docs/design/`:

| Tài liệu | Nội dung |
|---|---|
| [01 — Transaction & Outbox](docs/design/01-transaction-outbox.md) | `TxManager` truyền `pgx.Tx` qua `context` để `app` không biết pgx · isolation level và retry lỗi 40001 · schema outbox · relay có publisher confirm · consumer idempotent · cấu hình exchange/queue/DLQ |
| [02 — Hợp đồng API](docs/design/02-api-contract.md) | Quy ước JSON · **UUID v7 cho toàn bộ ID** và mã hiển thị cho khách · mô hình lỗi RFC 7807 + `platform/errs` · phân trang offset và cursor · quy trình OpenAPI + contract test · CORS, rate limit, timeout |
| [03 — Redis](docs/design/03-redis-cache.md) | Tách cache và dữ liệu gốc · quy ước key có version · bảng TTL · cache-aside + singleflight + jitter · vô hiệu hóa cache · giỏ hàng · rate limit · **khi nào KHÔNG được dùng khóa Redis** |
| [04 — Test](docs/design/04-testing.md) | Bản đồ test theo tầng · fake viết tay thay mock framework · testcontainers theo mẫu template database · **test song song chống bán vượt** · Playwright |
| [05 — Triển khai](docs/design/05-deployment.md) | Dockerfile distroless · bố trí production · graceful shutdown đúng thứ tự · **migration expand/contract** · quy trình deploy và rollback · sao lưu · ngưỡng cảnh báo · sổ tay sự cố |
| [06 — Frontend](docs/design/06-frontend.md) | Stack đã chốt · chiến lược render từng loại trang · gọi API · state ở URL · xác thực bằng cookie · SEO và JSON-LD · Core Web Vitals |

### Còn lại chưa thiết kế

Những phần này thuộc dự án con tương ứng, sẽ thiết kế khi tới:

| Hạng mục | Thuộc |
|---|---|
| Mô hình thuộc tính động của sản phẩm (JSONB vs EAV) | P1 |
| Cơ chế RBAC và vòng đời refresh token | P2 |
| Mô hình tồn kho đa kho + quy tắc giữ chỗ | P3 |
| Máy quy tắc khuyến mãi | P9 |
| Quy tắc tương thích linh kiện của PC Builder | Giai đoạn 3 |
