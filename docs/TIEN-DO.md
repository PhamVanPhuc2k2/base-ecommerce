# Tiến độ dự án

Ảnh chụp trạng thái, cập nhật 11/09/2026. Chi tiết kỹ thuật nằm ở
[`docs/design/`](design/); đặc tả và kế hoạch từng giai đoạn ở
[`docs/superpowers/`](superpowers/).

---

## Tổng quan

| Giai đoạn | Nội dung | Trạng thái |
|---|---|---|
| **P0.1** | Nền móng backend | ✅ xong, đã merge |
| **P0.2** | Module `catalog` (lát cắt dọc) | ✅ xong, đã merge |
| **P0.3** | Outbox + relay + worker | ✅ xong, đã merge |
| **P0.4** | Storefront Next.js | 🟡 **5/6 task xong**, đang ở nhánh `feat/p0-4-frontend` |
| P1 | Catalog & PIM đầy đủ | chưa bắt đầu |

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
| `arch` | Sai chiều phụ thuộc hexagonal, hạ tầng biết tới module nghiệp vụ |
| `api-codes` | Mã lỗi Go ↔ enum trong `openapi.yaml` |
| `error-messages` | Enum trong `openapi.yaml` ↔ thông điệp tiếng Việt ở frontend |
| `tree` | Cây thư mục ở README mục 4 ↔ đĩa thật |
| `sqlc-drift` | Code sqlc đã sinh ↔ file `.sql` |

Mỗi phép kiểm đều **đã được chứng minh là bắt được vi phạm** trước khi được tin —
dự án không chấp nhận một dấu xanh chưa từng thấy đỏ.

---

## P0.1 — Nền móng backend ✅

`platform/`: `config` (validate lúc khởi động), `httpx` (RFC 7807
`application/problem+json`), `postgres` (pgxpool, `DBTX`, `TxManager` truyền
`pgx.Tx` qua context), `redis`, `health` (`/healthz` vs `/readyz`),
`observability` (slog JSON + request ID). Graceful shutdown. CI GitHub Actions.

---

## P0.2 — Module `catalog` ✅

Hexagon hoàn chỉnh: `domain` ← `app` ← `adapter`. Sản phẩm, danh mục (cây),
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

## P0.4 — Storefront Next.js 🟡

**Stack:** Next.js 16.3.4, React 19.2.8, TypeScript strict (có
`noUncheckedIndexedAccess`), Tailwind CSS 4, Biome 2.5. shadcn/ui để P1.

### Đã xong (Task 1–5)

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

### Task 6 — Docker: code đã viết, mới xác minh được một phần

`apps/web/Dockerfile` và service `web` trong `compose.dev.yml` đã có, cùng
`API_URL` và `SITE_URL` trong `.env.example`.

**Đã tự chạy và đo được:**

| Kiểm | Kết quả |
|---|---|
| `docker build` | **thành công**, image **76 MB** |
| User chạy | `node` (không phải root) ✅ |
| Entrypoint | `["node","server.js"]` — dạng exec ✅ |
| **CSS có tới trình duyệt không** | **có** — `/_next/static/chunks/*.css` trả `200 text/css`, 19.319 byte. Cái bẫy `standalone` không chép `.next/static` đã được xử lý đúng |
| Font | `@font-face` trỏ `../media/*.woff2`, tải được `200 font/woff2` — `next/font/google` đã nhúng font vào build, **runtime không phụ thuộc Google Fonts** |
| `docker stop` | dừng sau **0,36 giây** |

⚠️ **`docker stop` trả exit code `143`, không phải `0`.** Cần hiểu đúng con số
này: `143 = 128 + 15` nghĩa là tiến trình **nhận được SIGTERM** — tức entrypoint
dạng exec hoạt động, binary đúng là PID 1. Nếu sai thì sẽ là `137` (SIGKILL sau
khi hết thời gian chờ), và `docker stop` sẽ mất 10 giây chứ không phải 0,36.

Nhưng `server.js` của Next.js standalone **không cài handler SIGTERM**, nên nó
chết theo tín hiệu thay vì tự thoát sạch. Hệ quả: request đang xử lý bị cắt
ngang lúc deploy. Backend Go thì thoát `0` vì có đoạn dừng êm tự viết. Đây là
khoảng cách thật giữa hai tiến trình, **chưa xử lý**.

**Chưa làm:**

- [ ] **12 mục kiểm chứng cuối chạy trong container** — còn thiếu: slug lạ có
      còn trả 404 không (xem cảnh báo `loading.tsx` ở trên), tắt container `api`
      thì trang có hiện lỗi tử tế không, phân trang, JSON-LD
- [ ] `docker build --no-cache` để lộ phụ thuộc mạng vào `fonts.gstatic.com`
      **lúc build** (runtime đã chứng minh là không phụ thuộc)
- [ ] Xử lý SIGTERM cho Next.js, hoặc ghi nhận đây là giới hạn chấp nhận được
- [ ] `compose.prod.yml` thêm service `web`
- [ ] Đồng bộ tài liệu, merge vào `main`

---

## Giới hạn đã biết, chấp nhận có ý thức

| | |
|---|---|
| **Không có test tự động** | Quyết định của chủ dự án. Rủi ro đã ghi rõ ở [thiết kế 04](design/04-kiem-chung.md) mục 4 |
| **Thiếu `GET /brands`** | `Product` chỉ có `brand_id`, nên storefront **không hiện được tên thương hiệu**. Thiếu sót của hợp đồng API, P1 phải thêm |
| **`PATCH` không đổi được `category_id`** | DTO không có trường đó. Chuyển danh mục là thao tác admin rất thường gặp — P1 |
| **Sitemap trần 20.000 sản phẩm** | `max_page 200 × limit 100`. P1 làm sitemap phân mảnh |
| **Một bản `outboxrelay`** | `FOR UPDATE SKIP LOCKED` cho phép nhiều bản nhưng **phá vỡ thứ tự event** |
| **At-least-once, không exactly-once** | Consumer bắt buộc idempotent. Không có cách nào bỏ yêu cầu này |
| **Backoff cố định 30 giây** | `x-message-ttl` hết hạn theo thứ tự đầu hàng, nên không đặt TTL riêng từng message được |
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
