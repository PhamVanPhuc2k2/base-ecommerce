# Tiến độ dự án

Ảnh chụp trạng thái, cập nhật 30/09/2026. Chi tiết kỹ thuật nằm ở
[`docs/design/`](design/); đặc tả và kế hoạch từng giai đoạn ở
[`docs/superpowers/`](superpowers/).

---

## Tổng quan

| Giai đoạn | Nội dung | Trạng thái |
|---|---|---|
| **P0.1** | Nền móng backend | ✅ xong, đã merge |
| **P0.2** | Module `catalog` (lát cắt dọc) | ✅ xong, đã merge |
| **P0.3** | Outbox + relay + worker | ✅ xong, đã merge |
| **P0.4** | Storefront Next.js | ✅ xong, đã merge |
| **P1** | Catalog & PIM | 🟡 **P1.1 → P1.3 xong** — còn P1.4 (media), P1.5 (SEO & storefront), xem [tổng quan](superpowers/specs/2026-09-29-p1-tong-quan.md) |

---

## Chạy dự án

```bash
task up                 # hạ tầng: postgres, redis, rabbitmq
task migrate
task run                # API Go trên máy (cần: set -a && . ./.env && set +a)
task web-dev            # Next.js trên máy, cổng 3000

task up-docker          # dựng TẤT CẢ trong container
task down-docker
```

`task check` chạy **8 bước kiểm tra tĩnh** — đây là lưới an toàn tự động duy nhất
của dự án (không có unit test):

| Bước | Bắt được gì |
|---|---|
| `build`, `vet`, `lint` | Lỗi biên dịch, phân tích tĩnh Go, golangci-lint |
| `arch` | 6 luật phụ thuộc của Clean Architecture (xem README mục 3.1) |
| `api-codes` | Mã lỗi Go ↔ enum trong `openapi.yaml` |
| `error-messages` | Enum trong `openapi.yaml` ↔ thông điệp tiếng Việt ở frontend |
| `tree` | Cây thư mục ở README mục 4 ↔ đĩa thật |
| `sqlc-drift` | Code sqlc đã sinh ↔ file `.sql` |

Mỗi phép kiểm đều **đã được chứng minh là bắt được vi phạm** trước khi được tin —
dự án không chấp nhận một dấu xanh chưa từng thấy đỏ.

---

## Kiến trúc backend — Clean Architecture + go-chi ✅

Nhánh `refactor/clean-architecture` (29/09/2026) chuyển backend từ bố cục
"module trước, tầng sau" (`internal/catalog/{domain,app,adapter}`) sang bố cục
**phân theo tầng** của Clean Architecture. Chỉ đổi vị trí và tên package —
**không đổi hành vi**; code sqlc sinh lại giống hệt từng byte.

```
apps/api/
├── cmd/                 # composition root — cmd/api/wire.go ráp các tầng
├── internal/
│   ├── domain/          # Entities
│   ├── usecase/         # Use Cases + interface (ports.go)
│   ├── repository/      # pgstore, rediscache, outboxpub, outbox
│   └── delivery/        # httpapi: router go-chi + handler
├── pkg/                 # config, errs, httpx, postgres, redis, rabbitmq...
├── migrations/
└── .air.api.toml, .air.worker.toml
```

- **Router:** go-chi (`chi/v5`), dựng ở `internal/delivery/httpapi/router.go`.
- **Không viết unit test** — vẫn giữ nguyên quyết định cũ; `task check` là lưới an toàn.
- `check-arch.sh` viết lại thành **6 luật**, thêm luật mới: `delivery` và
  `repository` không được import lẫn nhau. Cả 6 luật đã được chứng minh báo đỏ
  bằng cách cài vi phạm thật rồi gỡ ra.
- Thêm `task dev` / `task dev-worker` chạy hot-reload bằng air (cần
  `go install github.com/air-verse/air@latest`). **Chưa chạy thử** vì máy dev
  chưa cài air.

**Đánh đổi đã chấp nhận:** `domain/` và `usecase/` giờ là một package phẳng cho
mọi nghiệp vụ. Tới khi có 3–4 nghiệp vụ (P2–P4) mà package quá đông, tách thành
`internal/domain/<nghiep-vu>/` — luật trong `check-arch.sh` đã khớp cả thư mục
con nên không phải sửa.

---

## P0.1 — Nền móng backend ✅

`pkg/`: `config` (validate lúc khởi động), `httpx` (RFC 7807
`application/problem+json`), `postgres` (pgxpool, `DBTX`, `TxManager` truyền
`pgx.Tx` qua context), `redis`, `health` (`/healthz` vs `/readyz`),
`observability` (slog JSON + request ID). Graceful shutdown. CI GitHub Actions.

---

## P0.2 — Module `catalog` ✅

Lát cắt dọc qua đủ các tầng: `domain` ← `usecase` ← `repository`/`delivery`. Sản phẩm, danh mục (cây),
thương hiệu. sqlc cho query tĩnh, squirrel cho query động. Cache-aside Redis với
singleflight và TTL jitter. Hợp đồng OpenAPI 3.1 + type TypeScript sinh tự động.

**Đợt kiểm chứng thủ công tìm ra 12 lỗi thật, 3 Critical:**

| | Lỗi |
|---|---|
| Critical | Cache chứa `null` → panic → **500 vĩnh viễn** cho đúng sản phẩm đó |
| Critical | Cache chứa `{}` → 200 kèm sản phẩm bịa, **giá 0**, suốt cả TTL |
| Critical | `openapi.yaml` không hợp lệ với 3.1 — dấu phẩy trong flow mapping đẻ ra property rác |
| Important | `PATCH` xóa âm thầm `short_description` |
| Important | `HTTP_WRITE_TIMEOUT` = `middleware.Timeout` → client nhận "Empty reply" thay vì mã lỗi |
| Important | Redis chết → API **không khởi động được**; và mỗi lệnh tốn ~820 ms |

Đo trên **200.000 dòng thật**: `count(*)` kèm mỗi lần gọi danh sách tốn **5714
buffer** — gấp 1400 lần bản thân trang dữ liệu. Đó là lý do P1 phải cache số đếm.

---

## P0.3 — Outbox, relay, worker ✅

Sự kiện đi từ transaction database ra RabbitMQ tới worker mà **không mất và
không xử lý trùng**, bằng cách thay đúng **một adapter** — không sửa một dòng nào
trong `domain` hay `app`.

Ba phép thử đáng giá nhất, cả ba đều **không lộ ra khi mọi thứ chạy bình thường**:

| | Kết quả |
|---|---|
| Request lỗi | **Không** dòng outbox nào — qua ba đường lỗi khác nhau |
| RabbitMQ chết | API vẫn 201, bật lại thì relay tự đẩy đi |
| Giết relay giữa chừng | 200 sự kiện → 282 message, **0 sự kiện mất** |

**Hai lỗi Critical tìm ra:** consumer đếm số lần thử bằng `x-death` của RabbitMQ —
nhưng `count` chỉ tăng khi **chính RabbitMQ** dead-letter, mà consumer tự publish
bản sao, nên nó đứng yên ở 1 và message quay vòng **vĩnh viễn** không bao giờ tới
DLQ. Và message không route được vẫn được broker ack, nên relay đánh dấu
`published_at` cho message đã bị vứt.

Thông lượng publish: **19 → 1028 msg/s** sau khi bỏ một khe chờ 50 ms thừa.

---

## P0.4 — Storefront Next.js ✅

**Stack:** Next.js 16.3.4, React 19.2.8, TypeScript strict (có
`noUncheckedIndexedAccess`), Tailwind CSS 4, Biome 2.5. shadcn/ui để P1.

### Task 1–5

| | |
|---|---|
| **Lớp `lib/`** | `apiGet` xử lý **ba đường lỗi** (problem+json, body không phải JSON, `fetch` ném → `NETWORK_ERROR`); `ApiError` mang `code` + `request_id`; bảng tra 27 mã lỗi ra tiếng Việt; `formatVND` nhận **chuỗi** |
| **Layout** | header, footer, breadcrumb, `error.tsx`, `global-error.tsx`, `not-found.tsx` |
| **`/danh-muc`** | bộ lọc trên URL (không `useState` — nút Back hoạt động đúng), sắp xếp, phân trang theo `meta.has_next`, trạng thái rỗng |
| **`/san-pham/[slug]`** | ISR 60 s, `generateMetadata`, JSON-LD `Product`, bảng thông số |
| **SEO** | `sitemap.ts` (dừng theo `has_next`, API chết vẫn ra XML tối thiểu), `robots.ts` |

### Ba phát hiện đáng nhớ

**`loading.tsx` ở gốc gây soft 404.** Đo được: có `app/loading.tsx` thì
`/san-pham/<slug-lạ>` trả **HTTP 200**; dời nó đi thì trả **404**. Nguyên nhân:
`loading.tsx` bọc mọi route con trong Suspense, nên Next xả vỏ kèm status 200 rồi
mới stream nội dung. Trình duyệt vẫn hiện trang 404 đúng đắn — chỉ Google đọc
200, kết luận soft 404, và giữ sản phẩm ngừng bán trong chỉ mục vĩnh viễn. Loại
hỏng câm, và tệ hơn lỗi 500 vì không ai nhận ra.

**`error.tsx` không bao giờ đọc được `ApiError.code`.** React tước sạch thuộc
tính tùy biến của lỗi ném từ Server Component, ở **cả dev lẫn production**
(`code`/`status`/`requestId` đều `undefined`; ở production cả `name` cũng thành
`Error`). Nên mọi trang phải **tự `try/catch`** và render component lỗi của chính
nó — đó là chỗ duy nhất `code` và `request_id` còn nguyên vẹn.

**`subsets: ['latin']` làm vỡ chữ tiếng Việt.** Dấu tiếng Việt nằm ở `latin-ext`.
Thiếu nó thì trình duyệt vẫn hiện chữ nhưng lấy riêng các chữ có dấu từ font dự
phòng của hệ điều hành — một dòng tiêu đề pha hai bộ chữ.

### Task 6 — Docker và kiểm chứng đầy đủ ✅

Dựng cả stack bằng `task up-docker` (api, outboxrelay, worker, web đều healthy)
rồi chạy 12 mục của đặc tả §8. Mục 4, 5, 10 chạy bằng **Chrome thật** (headless,
bấm link, bấm Back) chứ không chỉ đọc HTML.

| # | Kiểm | Kết quả |
|---|---|---|
| 1 | `task check` | 8/8 xanh |
| 2 | build | `next build` sạch trong Docker, cả khi **chặn Google Fonts** |
| 3 | `/danh-muc` | 24 sản phẩm thật, giá `22.590.000 ₫` |
| 4 | Lọc + Back | Lọc → sắp xếp → Back → Back: URL, số sản phẩm, sản phẩm đầu **khớp từng bước** |
| 5 | Phân trang | Trang 1 có "trang sau", trang 2 (9 sp) thì **không**; mọi link phân trang đều 200 |
| 6 | Chi tiết | Dấu tiếng Việt đúng, JSON-LD `Product` có `offers.price` |
| 7 | Metadata | `<title>`, `og:title`, `canonical` đúng |
| 8 | Slug lạ | **HTTP 404** thật (không phải soft 404) |
| 9 | API chết / API treo | Thông điệp tiếng Việt, không stack trace — xem phát hiện 2 |
| 10 | Network/Console | Không lỗi JS. Có `/_next/image` **500** — do dữ liệu mẫu dùng ảnh bịa `https://vi.du/anh.jpg`, xem giới hạn |
| 11 | sitemap/robots | 35 URL (33 sản phẩm), `Disallow: /admin` |
| 12 | Docker | Lên đủ, dừng êm — xem phát hiện 3 |

### Bốn phát hiện của Task 6

**1. Build phụ thuộc Google Fonts — đã cắt.** `docker build --no-cache` xanh chỉ
chứng minh "có mạng thì build được". Chặn riêng hai tên miền font
(`--add-host fonts.googleapis.com:127.0.0.1 ...`) thì build **hỏng hẳn**:
`Failed to fetch Geist from Google Fonts`. Chuyển sang `next/font/local` với
Geist 1.7.2 (OFL) trong `app/fonts/`; kiểm bằng fontTools là đủ mọi chữ có dấu
tiếng Việt. Build lại khi vẫn chặn Google: xanh, không một cảnh báo.

**2. `apiGet` không có timeout.** API *chết* thì không sao — kết nối bị từ chối
ngay. API *treo* (`docker pause`) thì `fetch` chờ tới 300 giây mặc định của
undici, request dồn ứ. Thêm `AbortSignal.timeout(10s)`: đo lại, trang ra
thông điệp lỗi tiếng Việt sau 12 giây. Next.js 16 xử lý `signal` riêng nên
không phá cache ISR (`patch-fetch.js`).

**3. Ghi chú cũ về SIGTERM là SAI — Next.js có dừng êm.** Bản trước của file
này viết "server.js không cài handler SIGTERM, request đang xử lý bị cắt
ngang". Đọc `next/dist/server/lib/start-server.js`: Next 16 **có** handler — nó
gọi `server.close()` (ngừng nhận kết nối mới, chờ request đang dở) rồi **cố ý**
`process.exit(143)`. Đo thật: API bị pause, gửi request, `docker stop` ở giây
thứ 2 → client vẫn nhận **đủ HTTP 200, 19 KB ở giây 10,2**, container thoát
**143** (không phải 137 = SIGKILL). Không cần sửa gì; 143 là con số đúng của
Next.js, không phải lỗi.

**4. Container không ghi được cache ISR.** Log đầy
`Failed to update prerender cache ... EACCES mkdir '/app/.next/cache'`: mọi
file COPY vào thuộc root, tiến trình chạy bằng `node`. Trang vẫn đúng nhờ cache
RAM nên thử nhanh không thấy. Sửa: chỉ `chown` đúng `.next/cache` — `server.js`
vẫn chỉ đọc với `node` (đã thử `touch`: Permission denied).

Ngoài ra: `compose.prod.yml` thêm `web`, và mở `worker` + `outboxrelay` (vẫn bị
comment dù P0.3 đã xong), kèm `RABBITMQ_URL` còn thiếu. Biến chung tách thành
anchor `x-go-env` vì `<<` của YAML chỉ gộp nông. Thiếu `SITE_URL` thì compose từ
chối chạy.

---

## P1.3 — Thuộc tính động theo danh mục ✅

Định nghĩa thuộc tính (kiểu text/number/boolean/enum, đơn vị, lọc được, thuộc
tính biến thể), gán cho danh mục và **kế thừa xuống danh mục con**; validate ở
mọi đường ghi; lọc theo tùy chọn biến thể; `GET /products/facets`; storefront
có bộ lọc facet và bảng thông số dùng tên + đơn vị. Chi tiết:
[đặc tả](superpowers/specs/2026-09-30-p1-3-thuoc-tinh-dong.md).

**Quyết định lớn:** giữ JSONB, thêm bảng định nghĩa — không chuyển sang EAV
(bộ lọc N thuộc tính thành N lần JOIN). Danh mục **chưa khai thuộc tính nào
vẫn tự do như P0**, nên 33 sản phẩm hiện có không vỡ khi triển khai.

| # | Kiểm (stack Docker) | Kết quả |
|---|---|---|
| 2 | Danh mục chưa khai | PATCH khóa tùy ý → 200, như P0 |
| 3 | Gán `cpu` ở `laptop` | `laptop-gaming` thừa kế (`inherited: true`, `required: true`) |
| 4 | Khóa lạ + text rỗng + boolean "có" + số "16GB" + enum "Đỏ" | **Một** lần 422 `VALIDATION_FAILED`, `errors[]` đủ **5** trường |
| 5 | Thuộc tính sai cấp (biến thể ↔ sản phẩm) | 422 đúng từng trường |
| 6 | Publish thiếu `required` | 422 `ATTRIBUTE_REQUIRED`; bản nháp vẫn lưu được |
| 7 | Chuyển sản phẩm từ danh mục tự do sang danh mục chặt | Validate theo danh mục **mới** |
| 8 | `ram=16&mau=Đen` | Chỉ sản phẩm có **một** phiên bản thỏa cả hai; `ram=8&mau=Đen` → rỗng |
| 9 | Facet `laptop-gaming` | 6/6 giá trị: số facet **khớp** `total` khi bấm |
| 10 | Xóa định nghĩa đang gán / PATCH `type` | 409 `ATTRIBUTE_IN_USE` / 400 |
| 11 | `/danh-muc?category=laptop-gaming` | Nhóm RAM, Màu có số đếm; link `attr.*`; đổi danh mục bỏ `attr.*` cũ |
| 12 | Trang chi tiết | "RAM: 16 GB · Màu: Đen" theo **thứ tự quản trị đặt** |

**Lỗi tìm ra khi kiểm mục 12:** JSON từ Go có khóa sắp theo bảng chữ cái, nên
"Màu" luôn đứng trước "RAM" dù quản trị đặt RAM lên đầu. Trang chi tiết giờ đọc
`GET /categories/{slug}/attributes` để sắp theo `position`.

Mục 11 chưa bấm Back bằng trình duyệt thật: facet dùng đúng cơ chế link của bộ
lọc danh mục, cơ chế đó đã kiểm bằng Chrome ở P0.4.

---

## P1.2 — Biến thể / SKU ✅

`Product` thành aggregate chứa `Variant` (SKU, giá, options, active/inactive).
`products.price` ở lại với nghĩa **giá "từ"**, domain tự tính lại sau mọi thay
đổi variant. API quản trị variant; storefront hiện "Từ …", bảng phiên bản,
JSON-LD `AggregateOffer`. Chi tiết: [đặc tả](superpowers/specs/2026-09-30-p1-2-bien-the.md).

**Migration expand/contract đầu tiên của dự án**, làm đúng bốn bước:

| Bước | Việc | Đã kiểm |
|---|---|---|
| 1 | Expand: tạo `product_variants`, backfill | 33/33 variant khớp sku, giá, tiền tệ, ngày tạo; down → up lại |
| 2 | Code ghi CẢ `products.sku` (= SKU variant đầu) lẫn variant | Chạy trên schema expand: đọc sản phẩm cũ, tạo sản phẩm 2 variant, cột cũ vẫn được ghi |
| 3 | Contract: `DROP COLUMN products.sku` | Down dựng lại cột: md5 của mọi cặp `(id, sku)` **trùng khớp** trước contract; index dựng lại đúng định nghĩa |
| 4 | Code thôi ghi cột cũ | `sqlc generate`: model không còn `Sku` ở cấp sản phẩm |

ID variant backfill = ID sản phẩm: Postgres 17 chưa có `uuidv7()`, còn
`gen_random_uuid()` là v4 — phá quy ước "mọi ID là UUIDv7".

| # | Kiểm (stack Docker) | Kết quả |
|---|---|---|
| 4 | Tạo 2 variant 25tr/20tr | `price` = 20tr; thiếu variant → 422 `VARIANT_REQUIRED` |
| 5 | Variant 20tr → 30tr | `price` = 25tr trong response **và** trong DB |
| 6 | Tắt variant rẻ nhất | `price` đổi theo; công khai chỉ thấy variant active |
| 7 | Tắt variant active cuối / giá 0 khi live | 422 `NO_ACTIVE_VARIANT` (DB không đổi) / 422 `PRICE_REQUIRED` |
| 8 | Options trùng (`" ram "` = `"ram"`) / SKU trùng | 409 `DUPLICATE_VARIANT_OPTIONS` / 409 `DUPLICATE_SKU` |
| 9 | Variant của sản phẩm khác trên URL | 404 `UNKNOWN_VARIANT` |
| 10 | Lọc/sắp theo giá | Theo giá "từ"; plan vẫn `Index Only Scan using products_live_price_idx` |
| 11 | Một trang 24 sản phẩm | **Đúng 1** câu `VariantsByProducts` — không N+1 (đếm trong log Postgres) |
| 12 | Web | "Từ 25.000.000 ₫", bảng phiên bản, JSON-LD `AggregateOffer` low/high/offerCount |

**Thay đổi hợp đồng có chủ đích:** `Product.sku` bỏ (SKU nằm trong `variants`);
`POST /admin/products` nhận `variants[]`; `PATCH` sản phẩm gửi `price` giờ trả
400 `MALFORMED_REQUEST` — báo to thay vì âm thầm không đổi giá. Payload sự kiện
`product.*` bỏ `sku`, còn `{slug}`.

---

## P1.1 — Quản trị danh mục, thương hiệu ✅

API quản trị danh mục (tạo, sửa, chuyển, xóa) và thương hiệu, `GET /brands`,
`PATCH` sản phẩm đổi được danh mục/thương hiệu, cache số đếm, storefront hiện
tên thương hiệu. Không có migration. Chi tiết:
[đặc tả](superpowers/specs/2026-09-29-p1-1-quan-tri-danh-muc-thuong-hieu.md).

| # | Kiểm (trên stack Docker) | Kết quả |
|---|---|---|
| 2 | Tạo A > B > C | Cây mới hiện **ngay** (trước đây phải chờ TTL 6 giờ) |
| 3 | Chuyển A vào dưới C / dưới chính A | 422 `CATEGORY_CYCLE`, cây không đổi |
| 4 | **20 lượt hai PATCH chéo nhau đồng thời** (X→dưới Y ‖ Y→dưới X) | 20/20 lượt: đúng một bên 200, bên kia `CATEGORY_CYCLE`; **0 vòng lặp** |
| 5 | `PATCH {"name"}` không có `parent_id` | Ở nguyên chỗ, slug giữ nguyên |
| 6 | `PATCH {"parent_id": null}` | Lên làm gốc |
| 7 | Xóa còn con / còn sản phẩm / id lạ | 409 `CATEGORY_HAS_CHILDREN` / 409 `CATEGORY_HAS_PRODUCTS` / 404 `UNKNOWN_CATEGORY` |
| 8 | CRUD thương hiệu | `/brands` thấy ngay tên mới; trùng slug 409; không khóa 401 |
| 9 | `PATCH` sản phẩm đổi danh mục + thương hiệu | Có mặt trong `?category=` mới (cả danh mục cha) và `?brand=` mới |
| 10 | Cache số đếm | Log Postgres: `count(*)` chạy ở lần gọi 1, **không chạy** ở lần 2 và 3 |
| 11 | Bơm `null` / `[null]` vào cache đếm, cây, thương hiệu | Cả ba bị từ chối kèm WARN, API trả đúng, không panic |
| 12 | Trang chi tiết | Tên hãng + link `?brand=` + JSON-LD `brand` |

**Hai lỗ tìm ra khi thiết kế, đều im lặng nếu để lọt:**

- `[null]` trong cache cây danh mục làm `NewTree` **panic** — có từ P0.2.
  `GetOrLoad` chỉ từ chối `null` ở cấp ngoài cùng. Sửa bằng kiểu
  `domain.Categories`/`Brands` có `Validate()` kiểm từng phần tử.
- Cache số đếm kiểu `int`: `json.Unmarshal("null")` để nguyên 0 **không báo
  lỗi** → `has_next: false`, phân trang biến mất. Phải là `*int`.

**Đo khóa chống vòng lặp:** kiểm "cha mới không nằm trong cây con" là chưa đủ —
hai request đồng thời đều qua được phép kiểm. `LOCK TABLE categories IN SHARE
ROW EXCLUSIVE MODE` trước khi đọc cây xếp hàng mọi lần ghi danh mục mà không
chặn SELECT lẫn khóa FK khi ghi sản phẩm.

---

## Giới hạn đã biết, chấp nhận có ý thức

| | |
|---|---|
| **Không có test tự động** | Quyết định của chủ dự án. Rủi ro đã ghi rõ ở [thiết kế 04](design/04-kiem-chung.md) mục 4 |
| **Sitemap trần 20.000 sản phẩm** | `max_page 200 × limit 100`. P1 làm sitemap phân mảnh |
| **Một bản `outboxrelay`** | `FOR UPDATE SKIP LOCKED` cho phép nhiều bản nhưng **phá vỡ thứ tự event** |
| **At-least-once, không exactly-once** | Consumer bắt buộc idempotent. Không có cách nào bỏ yêu cầu này |
| **Backoff cố định 30 giây** | `x-message-ttl` hết hạn theo thứ tự đầu hàng, nên không đặt TTL riêng từng message được |
| **Trang lỗi trả HTTP 200** | API chết thì `/danh-muc` hiện `<ErrorState>` với status 200 — App Router không cho Server Component đặt 503. Crawler có thể index trang lỗi nếu API chết đúng lúc nó ghé |
| **Dữ liệu mẫu dùng ảnh bịa** | `https://vi.du/anh.jpg` làm `/_next/image` trả 500. Kèm theo: `remotePatterns` đang cho `hostname: '**'` — lỗ hổng lạm dụng băng thông/SSRF, P1 phải siết về CDN thật (đã có TODO trong `next.config.ts`) |
| **Ký hiệu ₫ không có trong Geist** | Hiện bằng font dự phòng — y như bản Google Fonts trước đây |
| **Facet chọn một giá trị mỗi nhóm, đếm conjunctive** | "8GB hoặc 16GB" cần API hỗ trợ OR; số đếm áp cả bộ lọc của chính nhóm đó (P1.3) |
| **Chưa lọc khoảng số** | `RAM ≥ 16` chưa có — chỉ lọc bằng đúng giá trị |
| **Dữ liệu cũ chỉ bị kiểm ở lần ghi kế tiếp** | Bật chế độ chặt cho danh mục hay bỏ giá trị enum không sửa dữ liệu đã lưu; sản phẩm sai báo lỗi khi sửa lần sau |
| **Chưa có bộ chọn phiên bản** | Trang chi tiết hiện bảng phiên bản tĩnh; chọn để đổi giá/ảnh/thêm vào giỏ cần client component — P1.5 và P4 |
| **Không xóa được variant** | Có chủ đích: đơn hàng (P4) sẽ trỏ vào variant. Ngừng bán là `inactive` |
| **Số đếm lệch tối đa 60 giây** | Cache số đếm không vô hiệu hóa khi ghi — trang cuối có thể thiếu/thừa sản phẩm vừa đăng trong một phút (P1.1) |
| **Chưa phát sự kiện `category.*`/`brand.*`** | Chưa consumer nào cần; phát mà thiếu binding thì relay thử lại mãi. P7 thêm cả hai cùng lúc |
| **Tách hai instance Redis** | Hoãn tới P4 khi có giỏ hàng — P0 chỉ dùng vai trò cache |

---

## Cạm bẫy của chính môi trường dev này (Windows)

Những thứ đã làm hỏng **phép kiểm chứng**, không phải làm hỏng sản phẩm — ghi ra
để lần sau không mất công điều tra lại:

- **Ký tự tiếng Việt gõ thẳng vào lệnh shell bị biến thành `U+FFFD`** trước cả
  khi `curl` nhìn thấy (console cp1258). Ghi JSON ra file bằng Python
  (`encoding='utf-8'`, chế độ binary) rồi `curl --data-binary @file`.
- **`command -v python3` tìm thấy shim rỗng của Microsoft Store.** Phải thử
  `python3 -c "import yaml"` thật.
- **Python trên Windows in `CRLF`** → `comm`/`diff` coi `MÃ\r` khác `MÃ`.
- **Sửa file Go/TS bằng Python phải ghi chế độ binary**, nếu không CRLF làm
  formatter báo đỏ cả file.
- **`docker build ... | tail` che mất exit code** — build hỏng mà tưởng thành công.
- **Cổng 8080/3000 còn bị tiến trình cũ giữ** thì bản mới không bind được, và log
  vẫn in "server đang lắng nghe" **trước** khi bind thất bại.
- **Xóa một route Next rồi mà `tsc` vẫn đỏ** → `rm -rf apps/web/.next/dev`.
