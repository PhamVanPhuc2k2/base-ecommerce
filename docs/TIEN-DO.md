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
| **P1** | Catalog & PIM | ✅ **P1.1 → P1.5 xong** — xem [tổng quan](superpowers/specs/2026-09-29-p1-tong-quan.md) |
| **P2** | Identity | ✅ **Xong** — tài khoản + phiên, RBAC, OTP email, trang tài khoản + sổ địa chỉ trên storefront, xem [tổng quan](superpowers/specs/2026-09-30-p2-tong-quan.md) |

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

## P2.4 — Storefront: tài khoản + sổ địa chỉ ✅

- **Trang:** `/dang-nhap`, `/dang-ky`, `/quen-mat-khau`, `/dat-lai-mat-khau`,
  `/tai-khoan` (xác minh email, sửa họ tên, đăng xuất), `/tai-khoan/dia-chi`.
- **Phiên là BFF bằng cookie httpOnly** `bec_at`/`bec_rt`. Token không bao giờ
  tới JavaScript phía trình duyệt; API vẫn chỉ nhận Bearer.
- **`proxy.ts`** chặn `/tai-khoan/*` và làm mới token trước khi render.
- **API mới:** sổ địa chỉ (`/me/addresses`), `PATCH /me`, `TRUSTED_PROXIES`.
- **Mã nguồn đo bằng trình duyệt thật** (Puppeteer + Chrome) qua đúng các form
  và Server Action.
- **Chi tiết:** [đặc tả](superpowers/specs/2026-09-30-p2-4-storefront-tai-khoan.md).

| # | Kiểm | Kết quả |
|---|---|---|
| 1 | `task check`, `npm run lint/typecheck/build` | qua |
| 2 | `/tai-khoan/dia-chi?x=1` chưa đăng nhập | 307 → `/dang-nhap?next=%2Ftai-khoan%2Fdia-chi%3Fx%3D1` |
| 3 | Đăng ký trên trang → nhập mã từ Mailpit | cookie HttpOnly + SameSite=Lax, `bec_at` sống 839 s (token 900 s − 60); mã sai hiện lỗi; mã đúng → "Đã xác minh" |
| 4 | Xóa `bec_at`, mở `/tai-khoan` | trang render bình thường; `bec_at` mới, `bec_rt` đã xoay vòng |
| 5 | Hai request SONG SONG chỉ mang `bec_rt` | cả hai 200, cùng nhận một token mới; API chỉ thấy MỘT lần refresh; token mới dùng tiếp được (phiên không bị thu hồi) |
| 6 | `next=//evil.com`, `next=https://evil.com/x` | về `localhost:3000/tai-khoan`; `next=/tai-khoan/dia-chi` thì về đúng đó; sai mật khẩu giữ lại ô email |
| 7 | 6 lần đăng nhập sai qua web từ IP 203.0.113.7 (6 email khác nhau), rồi 1 lần từ 198.51.100.9 | `[401×5, 429]` rồi `401` — rate limit tính theo IP khách |
| 8 | Nối thẳng API từ mạng 10.99.0.0/24, gửi `X-Forwarded-For: 6.6.6.6` | log ghi IP socket 10.99.0.3; cùng header từ container web (tin cậy) → 6.6.6.6 |
| 9 | Sổ địa chỉ trên trang | SĐT `0123` báo lỗi ở đúng ô (`aria-invalid`), giữ các ô khác; `+84 912.345.678` lưu `0912345678`; đặt mặc định, xóa địa chỉ mặc định → địa chỉ còn lại lên thay; sửa có điền sẵn |
| 10 | Người B sửa/xóa/đặt mặc định địa chỉ của A; id rác | cả bốn 404 `UNKNOWN_ADDRESS`; địa chỉ thứ 11 → 422 `ADDRESS_LIMIT_REACHED`, luôn đúng 1 mặc định |
| 11 | Quên → đặt lại trên trang → đăng nhập | email điền sẵn; sau đặt lại về `/dang-nhap?da-doi-mat-khau=1`; mật khẩu cũ bị từ chối, mới vào được |
| 12 | Đăng xuất | về `/`, không còn cookie `bec_*`, refresh token cũ 401 |
| 13 | Trang sản phẩm có bị động hóa không | không đổi so với `main` — xem phát hiện dưới |

**Phát hiện, KHÔNG do P2.4:** `/san-pham/[slug]` trả `Cache-Control: private,
no-store` — trang render động, không phải "ISR 60 s" như ghi ở P1.5. Đã build
lại chính `main` để so: y hệt. Cần điều tra riêng (cache dữ liệu ở tầng fetch
vẫn còn, mất là cache HTML).

**Lỗi của script kiểm, không phải của code** — ghi lại để lần sau khỏi mất công:
cắt cookie `bec_rt=` bằng `slice(6)` (dài 7 ký tự) làm token gửi lại thừa dấu
`=`; và `waitForNetworkIdle` treo vì prefetch của `<Link>` — chờ response POST
của Server Action thay vào.

---

## P2.3 — OTP qua email ✅

Mã 6 số cho **xác minh email** và **quên / đặt lại mật khẩu**. Thư đi qua outbox
→ RabbitMQ (queue `mailer`, bind `email.*`) → worker → SMTP; dev dùng Mailpit
(`http://localhost:8025`). Mã KHÔNG nằm trong payload outbox — thư đã soạn nằm
ở `outbound_emails`, sự kiện chỉ mang id, worker gửi xong thì xóa nội dung.
Chi tiết: [đặc tả](superpowers/specs/2026-09-30-p2-3-otp-email.md).

| # | Kiểm | Kết quả |
|---|---|---|
| 1 | `task check` | 8/8 bước qua |
| 2 | Đăng ký | thư tới Mailpit sau 0,7 s; mã không nằm ở tiêu đề; `body_text` rỗng sau khi gửi |
| 3 | Mã đúng, rồi dùng lại | 200 `email_verified: true`; 409 `EMAIL_ALREADY_VERIFIED` |
| 4 | Xin lại ngay / sau 60 giây | 429 + `Retry-After: 60` / 202; mã cũ 422, mã mới 200 |
| 5 | 5 lần sai rồi mã ĐÚNG | lần 6 vẫn 422 `INVALID_OTP`; `attempts = 5` |
| 6 | Mã đúng nhưng hết hạn | 422 `INVALID_OTP` |
| 7 | Quên mật khẩu: có / không có / đang chờ | cả ba 202; chỉ email có mới nhận đúng một thư |
| 8 | Đặt lại mật khẩu | 204; refresh cũ 401; mật khẩu cũ 401, mới 200; dùng lại mã / email không tồn tại cùng 422 `INVALID_OTP` |
| 9 | Mã xác minh email đem đặt lại mật khẩu | 422 — mục đích nằm trong HMAC |
| 10 | Mailpit tắt khi đăng ký | đăng ký 201; bật lại → thư tới sau 34 s (queue retry 30 s) |
| 11 | Publish tay bản trùng `email.queued` | không có thư thứ hai (khóa là `sent_at`) |
| — | Log api/relay/worker | 0/8 mã xuất hiện |
| — | Worker `APP_ENV=production` thiếu `SMTP_HOST`/`MAIL_FROM` | từ chối khởi động |

**Hai chỗ rò thời gian đo ra được và đã sửa** (thời gian phía server, n=15, trung vị):

| Endpoint | Trước | Sau |
|---|---|---|
| `forgot`: không có / có | 0,8 ms / **21,6 ms** — transaction phát mã chờ fsync | 0,5 / 0,5 ms — trả 202 rồi mới phát mã ở goroutine nền |
| `reset`: không có / có, sai mã | 21,7 ms / **45,1 ms** — commit lần sai chờ fsync | 20,9 / 21,9 ms — `SET LOCAL synchronous_commit TO OFF` cho transaction ghi lần sai |

Còn ~1 ms (ba câu truy vấn) ở `reset`, dưới mức dao động mạng và bị rate limit 5/phút.

---

## P2.2 — Phân quyền (RBAC) ✅

Quyền cố định trong code (`catalog.products.write`, `catalog.taxonomy.write`,
`media.upload`, `iam.roles.manage`), vai trò và phép gán nằm trong DB.
`super_admin` là vai trò hệ thống: mọi quyền, không sửa/xóa được. **Bỏ hẳn
`X-Admin-Key`** — mọi `/admin/*` đi qua Bearer token rồi mới tới quyền. Super
admin đầu tiên cấp bằng `go run ./cmd/admintool grant-role <email> super_admin`.
Chi tiết: [đặc tả](superpowers/specs/2026-09-30-p2-2-rbac.md).

| # | Kiểm | Kết quả |
|---|---|---|
| 1 | `task check` | 8/8 bước qua |
| 2 | `X-Admin-Key` cũ | 401 `UNAUTHENTICATED` |
| 3 | Không token / khách thường | 401 / 403 `FORBIDDEN` (401 trước 403) |
| 4 | `admintool grant-role` (email CHỮ HOA) | super admin vào được mọi `/admin/*`; email chưa đăng ký → exit 1 kèm hướng dẫn |
| 5 | Vai trò "Biên tập sản phẩm" | tạo sản phẩm 201, tạo danh mục 403 |
| 6 | Thêm quyền cho vai trò đang giữ (cache quyền đã nạp sẵn) | tạo danh mục 201 ngay request kế tiếp |
| 7 | Gỡ vai trò, CÙNG access token còn hạn | 403 ngay; `/me` → `permissions: []` |
| 8 | Sửa/xóa `super_admin`; tự đổi vai trò mình; quyền lạ; vai trò không tồn tại | 422 `SYSTEM_ROLE_IMMUTABLE` ×2, `CANNOT_CHANGE_OWN_ROLES`, `UNKNOWN_PERMISSION`, `ROLE_NOT_FOUND` |
| 9 | Xóa vai trò còn người giữ | 409 `ROLE_IN_USE`; gỡ hết rồi xóa 204 |
| 10 | `/me` | super admin đủ 4 quyền; khách `[]` |

**Lỗi bắt được khi kiểm:** `/me` của khách thường MẤT trường `permissions` thay
vì trả `[]` — `omitempty` bỏ cả slice rỗng chứ không chỉ `nil`. Đổi sang
`*[]string`: `/me` luôn có trường, các endpoint khác vẫn không.

---

## P2.1 — Tài khoản và phiên đăng nhập ✅

Đăng ký / đăng nhập bằng email + mật khẩu, refresh token xoay vòng có phát hiện
dùng lại, đăng xuất, `GET /me`, rate limit. Chi tiết:
[đặc tả](superpowers/specs/2026-09-30-p2-1-tai-khoan-phien.md).

| # | Kiểm | Kết quả |
|---|---|---|
| 2 | Đăng ký → `/me` | 201 + `Cache-Control: no-store`; email lưu đã chuẩn hóa |
| 3 | `ZZ.P21@Example.VN ` rồi đăng ký lại bằng CHỮ HOA | 409 `EMAIL_TAKEN`; đăng nhập chữ thường 200 |
| 4 | Sai mật khẩu vs email không tồn tại | cùng 401 `INVALID_CREDENTIALS`; thời gian **phía server** 20,7 ms vs 20,4 ms (n=20, xen kẽ) |
| 5 | Refresh rồi DÙNG LẠI token cũ | 401, và token MỚI cũng chết — family thu hồi 2/2 |
| 6 | Đăng xuất → refresh | 401; đăng xuất token lạ vẫn 204 |
| 7 | Token `alg: none`, hết hạn (chữ ký đúng), sai chữ ký, sai `iss`, thiếu `exp` | tất cả 401; token tự ký hợp lệ (đối chứng) 200 |
| 8 | 6 lần đăng nhập sai | 5 × 401 rồi 429 + `Retry-After: 60` |
| 9 | Redis chết | đăng nhập 200 (fail-open) |
| 10 | DB và log | `$argon2id$v=19$m=19456,t=2,p=1$…`; `token_hash` 32 byte; không mật khẩu/token nào trong log |
| 11 | `APP_ENV=production` + `JWT_SECRET` dev / khóa < 32 byte | từ chối khởi động |

**Bẫy tránh được khi viết:** phát hiện dùng lại phải thu hồi cả family RỒI trả
lỗi — trả lỗi ngay trong closure của `TxManager` là transaction rollback và
lệnh thu hồi biến mất. Closure trả `nil` để commit, lỗi trả sau `Run`.

**Đo sai rồi đo lại:** lần đầu đo thời gian phía client thấy 39,7 ms vs 23,8
ms — tưởng lộ email qua thời gian. Lần đó loại "có tài khoản" luôn chạy trước
trong mỗi cặp. Đo lại phía server, xen kẽ thứ tự: không có chênh lệch.

---

## P1.5 — SEO & storefront ✅

Trang danh mục theo đường dẫn `/danh-muc/<slug>`, trang thương hiệu
`/thuong-hieu[/<slug>]`, một `ProductListing` dùng chung cho cả ba trang danh
sách; bộ chọn phiên bản; `BreadcrumbList` JSON-LD trên mọi breadcrumb; sitemap
phân mảnh thay trần 20.000 sản phẩm. **Hoãn shadcn/ui** tới khi có form/dialog
(giỏ hàng P4, admin UI sau P2). Chi tiết:
[đặc tả](superpowers/specs/2026-09-30-p1-5-seo-storefront.md).

| # | Kiểm | Kết quả |
|---|---|---|
| 2 | `/danh-muc?category=laptop&sort=price_asc` | **308** → `/danh-muc/laptop?sort=price_asc` (sau khi sửa — xem dưới) |
| 3 | `/danh-muc/laptop` | 33 sp khớp API, canonical đúng, link danh mục theo đường dẫn |
| 4 | `/danh-muc/<lạ>`, `/thuong-hieu/<lạ>` | **404 thật** |
| 5 | `/thuong-hieu/asus` | khớp `?brand=` của API; lọc danh mục ở lại trong hãng |
| 6 | Bộ chọn (Chrome, 3 phiên bản không đủ tổ hợp) | 5/5 bước đúng giá + SKU, kể cả hai lần "nhảy" sang phiên bản gần nhất; HTML không JS có đủ mọi tùy chọn |
| 7 | `BreadcrumbList` + **danh mục tên `</script><script>alert(1)</script>`** | JSON-LD đúng, URL tuyệt đối; KHÔNG có `<script>alert` thô trong HTML |
| 8 | `/sitemap.xml` | index hợp lệ: `pages.xml` + đúng `ceil(33/5000)` = 1 file sản phẩm |
| 9 | 200.000 sản phẩm giả | trang cuối: không index → Seq Scan + sort **tràn đĩa**; có index → **Index Only Scan, Heap Fetches 0** |
| 10 | 41 URL trong sitemap | tất cả 200; tên file lạ → 404 |
| 11 | API chết | index còn `pages.xml`; file sản phẩm đã cache vẫn 200, chưa cache → **503 + Retry-After** |
| 12 | Chrome, 6 trang | 0 lỗi Console/HTTP |

**Ba lỗi tìm ra khi kiểm, đều im lặng nếu để lọt:**

- **`permanentRedirect()` trong page trả HTTP 200.** `/danh-muc` nằm dưới
  `loading.tsx`: Next xả 200 trước, rồi chèn `<meta http-equiv="refresh">`.
  Trình duyệt vẫn chuyển trang, nhưng Google thấy 200 chứ không thấy 308. Dời
  sang `proxy.ts` (chạy trước render). Cùng họ với soft 404 của P0.4.
- **`/thuong-hieu` (ISR) được dựng sẵn LÚC BUILD** — khi `docker build` không
  có API — nên thứ bị cache là trang lỗi "Không kết nối được", và nằm đó một giờ
  sau MỖI lần deploy. Đổi sang render theo yêu cầu.
- **Tôi tự gây ra một lỗ XSS rồi tự bắt được:** JSON-LD breadcrumb viết
  `replaceAll('<', '\u003c')` trong source bị công cụ sửa file biến thành ký tự
  `<` thật — tức không thoát gì. Phép thử tên danh mục chứa `</script>` ở mục 7
  là để chắc nó đã đúng.

Cũng dời `loading.tsx` của `/danh-muc` vào route group `(tat-ca)`: đặt ở
`app/danh-muc/` nó bọc luôn `/danh-muc/<slug>` và biến slug lạ thành soft 404.
Migration: goose coi MỌI dòng comment chứa chuỗi annotation là chỉ thị, kể cả
giữa câu — đã ghi cảnh báo trong migration `sitemap_index`.

---

## P1.4 — Media ✅

MinIO giữ ảnh gốc, imgproxy thu nhỏ + đổi định dạng; upload bằng **presigned
POST** thẳng lên storage; `Product.images` giờ là **khóa media** chỉ nhận ảnh
đã upload xong; storefront bỏ `remotePatterns: '**'`. 33 sản phẩm mẫu đã có
ảnh thật (upload qua đúng luồng) thay cho URL bịa. Chi tiết:
[đặc tả](superpowers/specs/2026-09-30-p1-4-media.md).

**Đo hạ tầng TRƯỚC khi thiết kế** (imgproxy chế độ chỉ-preset, chỉ đọc bucket):

| Thử | Kết quả |
|---|---|
| Preset `w640` | 1600×1200 → 640×480; AVIF / WebP / JPEG theo `Accept`; cache 1 năm |
| Tham số tùy ý `rs:fit:5000:5000` | 404 `Invalid URL` — không ai đốt CPU bằng kích thước lạ |
| Nguồn ngoài `https://example.com/…` | 404 `Invalid source URL` — không thành proxy ảnh internet |

| # | Kiểm | Kết quả |
|---|---|---|
| 2 | Upload JPEG, PNG, WebP | cả ba `ready`, khóa `products/<uuidv7>.<đuôi>`; gọi `complete` lần hai an toàn |
| 3 | 11 MB / đổi Content-Type trong form | MinIO 400 `EntityTooLarge` / 403 `AccessDenied` — API không nhận byte nào |
| 4 | **File HTML khai `image/png`** | MinIO nhận (policy chỉ so chuỗi khai báo) → `complete` 422 `INVALID_IMAGE`, **object bị xóa** |
| 5 | `complete` khi chưa upload | 422 `UPLOAD_NOT_FOUND` |
| 6 | Policy hết hạn | **chưa kiểm** — TTL 15 phút không cấu hình được để thử nhanh |
| 7 | Sản phẩm với khóa pending + khóa bịa + khóa ready | 422, đúng `images[0]`, `images[1]` |
| 8 | `/img/w640/<khóa>` | AVIF/WebP/JPEG theo `Accept`, `Vary: Accept`, `immutable` |
| 9 | 5 URL xấu (preset lạ, `../`, URL ngoài, `.gif`, tham số) | 404 ở Next; **0 request** tới imgproxy |
| 10 | Chrome: `/danh-muc` + chi tiết | 24/24 ảnh tải được; **0** request `/_next/image`; 0 lỗi |
| 11 | `/_next/image?url=https://example.com/…` | 404 — hết proxy ảnh ngoài |
| 12 | og:image / JSON-LD | URL tuyệt đối, tải được từ ngoài (200) |

**Tưởng là lỗi, đo lại thì không:** lần đầu đọc header thấy `Vary` của Next
(`rsc, next-router-…`) mà không thấy `Accept`. Đọc `send-response.js`: Next
**nối thêm** chứ không ghi đè — response có HAI dòng `Vary`, và RFC 9110 gộp
chúng. Công cụ đo chỉ lấy dòng đầu.

Production: config **từ chối khởi động** nếu `S3_SECRET_KEY` còn là mật khẩu
MinIO dev (đã thử với `APP_ENV=production`).

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
| **Một bản `outboxrelay`** | `FOR UPDATE SKIP LOCKED` cho phép nhiều bản nhưng **phá vỡ thứ tự event** |
| **At-least-once, không exactly-once** | Consumer bắt buộc idempotent. Không có cách nào bỏ yêu cầu này |
| **Backoff cố định 30 giây** | `x-message-ttl` hết hạn theo thứ tự đầu hàng, nên không đặt TTL riêng từng message được |
| **Trang lỗi trả HTTP 200** | API chết thì `/danh-muc` hiện `<ErrorState>` với status 200 — App Router không cho Server Component đặt 503. Crawler có thể index trang lỗi nếu API chết đúng lúc nó ghé |
| **Ký hiệu ₫ không có trong Geist** | Hiện bằng font dự phòng — y như bản Google Fonts trước đây |
| **File upload dở không ai dọn** | `media` pending mà không bao giờ `complete` để lại object mồ côi. Cần job dọn hoặc lifecycle rule của bucket |
| **Ảnh cũ chỉ bị kiểm khi ghi lại ảnh** | Sản phẩm có URL cũ vẫn sửa được giá, phiên bản; chỉ lệnh đặt `images` hoặc publish mới kiểm |
| **`imgproxy:latest`, `minio:latest` chưa ghim phiên bản** | Nên ghim tag trước production |
| **Facet chọn một giá trị mỗi nhóm, đếm conjunctive** | "8GB hoặc 16GB" cần API hỗ trợ OR; số đếm áp cả bộ lọc của chính nhóm đó (P1.3) |
| **Chưa lọc khoảng số** | `RAM ≥ 16` chưa có — chỉ lọc bằng đúng giá trị |
| **Dữ liệu cũ chỉ bị kiểm ở lần ghi kế tiếp** | Bật chế độ chặt cho danh mục hay bỏ giá trị enum không sửa dữ liệu đã lưu; sản phẩm sai báo lỗi khi sửa lần sau |
| **Bộ chọn phiên bản không ghi URL** | Không chia sẻ được link tới đúng một phiên bản — đổi lại giữ ISR và HTML đầy đủ (đặc tả P1.5 mục 2.3). Chưa đổi ảnh theo phiên bản |
| **Chưa revalidate theo sự kiện** | Đổi giá thấy trên trang chi tiết sau tối đa 60 giây (ISR). Worker gọi `revalidateTag` — để P7 |
| **Trang danh mục/thương hiệu không ISR** | `force-dynamic` vì bộ lọc trên query string; cache nằm ở Redis backend và CDN |
| **Không xóa được variant** | Có chủ đích: đơn hàng (P4) sẽ trỏ vào variant. Ngừng bán là `inactive` |
| **Số đếm lệch tối đa 60 giây** | Cache số đếm không vô hiệu hóa khi ghi — trang cuối có thể thiếu/thừa sản phẩm vừa đăng trong một phút (P1.1) |
| **Chưa phát sự kiện `category.*`/`brand.*`** | Chưa consumer nào cần; phát mà thiếu binding thì relay thử lại mãi. P7 thêm cả hai cùng lúc |
| **Đăng ký lộ email đã tồn tại** | 409 `EMAIL_TAKEN`. Quên / đặt lại mật khẩu đã kín (P2.3), nhưng đăng ký thì chưa: giấu được cần đăng ký trả 202 rồi gửi thư "bạn đã có tài khoản" — đổi hợp đồng API, để lúc làm màn hình đăng ký (P2.4) cân nhắc. Rate limit làm việc dò hàng loạt đắt (P2.1) |
| **Đăng xuất không giết access token đang có** | Sống tối đa 15 phút — cái giá của JWT không tra DB mỗi request (P2.1) |
| **Gộp refresh chỉ trong MỘT tiến trình web** | `proxy.ts` giữ Map trong bộ nhớ. Chạy nhiều bản web thì hai bản vẫn có thể cùng refresh một token và API thu hồi phiên — lúc đó gộp qua Redis (P2.4) |
| **Header không hiện tên người đăng nhập** | Link "Tài khoản" tĩnh để trang danh mục/sản phẩm không bị render động vì đọc cookie; hiện tên bằng đảo client khi có giỏ hàng (P4) |
| **Dev: request từ máy host tới web/api trông như đến từ proxy tin cậy** | Đi qua gateway Docker 172.x.0.1, nằm trong `TRUSTED_PROXIES`. Production: api/web chỉ mở loopback, người duy nhất đi qua gateway là reverse proxy (P2.4) |
| **Quên mật khẩu phát mã ở goroutine nền** | Tiến trình tắt đúng lúc đó thì mã không được phát, người dùng bấm gửi lại. Cái giá của việc không để thời gian phản hồi lộ email (P2.3) |
| **Hai tab refresh cùng lúc = bị đăng xuất** | Không phân biệt được với token bị trộm. Storefront refresh ở server một chỗ nên hiếm (P2.1) |
| **Chưa có job dọn refresh token hết hạn** | Bảng tăng dần; có index `expires_at` sẵn cho job dọn |
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
