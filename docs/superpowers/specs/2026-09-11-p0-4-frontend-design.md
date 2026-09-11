# P0.4 — Next.js storefront: đặc tả

**Mục tiêu:** trang danh mục và trang chi tiết sản phẩm chạy thật, lấy dữ liệu từ
API Go qua type **sinh từ `openapi.yaml`** — để backend đổi tên một trường là
frontend **không biên dịch được**, chứ không phải `undefined` lúc chạy.

**Nền:** [thiết kế 06 — Frontend](../../design/06-frontend.md) đã chốt stack và
chiến lược render. Tài liệu này chỉ giải quyết những chỗ để ngỏ và ghi lại quyết
định.

---

## 1. Quyết định đã chốt

| | Chọn | Vì sao |
|---|---|---|
| Phạm vi | **Lát cắt dọc** — lib, layout, hai trang, error/loading/not-found, sitemap/robots | Danh sách "việc cần làm" ở thiết kế 06 là cho **toàn bộ** frontend qua nhiều giai đoạn. Giỏ hàng thuộc P4 và backend chưa có API nào cho nó |
| Component | **Tailwind thuần** | Hai trang với vài component chưa đủ việc để bù chi phí thiết lập shadcn. Thêm ở P1 khi có form và dialog thật |
| Docker | **Làm luôn** | Giữ đúng nguyên tắc đã dựng ở backend: thứ không chạy được trong container thì không deploy được |

---

## 2. Chỉ gọi API từ phía server — và vì sao điều đó xóa hẳn một lớp rắc rối

Cả hai trang của P0.4 đều là trang cần SEO, nên chúng là **Server Component**.
Không có lời gọi API nào từ trình duyệt.

Hệ quả quan trọng: **P0.4 không cần `NEXT_PUBLIC_API_URL`.**

Thiết kế 06 đã cảnh báo biến `NEXT_PUBLIC_*` bị **nhúng vào bundle lúc build**,
nên image của staging và production khác nhau nếu chúng trỏ tới API khác nhau —
phá vỡ nguyên tắc "một image cho mọi môi trường". Gọi API hoàn toàn từ server thì
vấn đề đó không tồn tại: `API_URL` là biến môi trường đọc **lúc chạy**.

```
Trình duyệt ──► Next.js (server) ──► API Go
                     API_URL
```

| Chạy ở đâu | `API_URL` |
|---|---|
| `npm run dev` trên máy | `http://localhost:8080/api/v1` |
| Trong container compose | `http://api:8080/api/v1` |

⚠️ Hai giá trị này **khác nhau** và đó là nguồn nhầm lẫn kinh điển: `localhost`
bên trong container là chính container đó, không phải máy host.

Khi P4 thêm giỏ hàng (cần gọi API từ client) thì cách đúng là **đường dẫn tương
đối `/api`** cộng một rewrite trong `next.config`, không phải thêm
`NEXT_PUBLIC_API_URL`.

---

## 3. Thông điệp lỗi tiếng Việt — và phép kiểm bằng máy

`ApiError` mang `code` (ổn định, do backend định nghĩa) và `request_id`. Giao
diện tra `code` ra câu tiếng Việt.

Backend hiện có **27 mã lỗi**. Thiếu một mã trong bảng tra nghĩa là khách thấy
một thông điệp chung chung vô dụng đúng lúc có sự cố.

**Bắt buộc có `scripts/check-error-messages.sh`** đối chiếu enum `code` trong
`api/openapi.yaml` với các khóa trong `apps/web/lib/errors.ts`:

- mã có trong spec mà thiếu trong bảng tra → **đỏ**
- khóa có trong bảng tra mà spec không khai → **đỏ** (mã đã bị xóa, hoặc gõ sai tên)

Đây là chiếc chân thứ ba của cùng một hợp đồng, và cả ba phải khớp nhau:

```
code trong Go  ──(check-openapi-codes)──►  enum trong openapi.yaml
                                                    │
                                    (check-error-messages)
                                                    ▼
                                        thông điệp trong lib/errors.ts
```

Luật này phải được chứng minh là **bắt được vi phạm** trước khi tin.

---

## 4. Tiền tệ: khi nào `Number` được phép, khi nào không

`price` là **chuỗi**. Spec ghi rõ "KHÔNG BAO GIỜ dùng number" vì JSON number qua
JS mất chính xác với `NUMERIC(15,2)` lớn.

Nhưng để **hiển thị** thì `Intl.NumberFormat` cần một `number`, nên phải nói rõ
ranh giới:

| Việc | Được dùng `Number`? |
|---|---|
| Hiển thị một giá | **Được.** Trần của `NUMERIC(15,2)` là `9_999_999_999_999.99` ≈ 10¹³, còn `Number.MAX_SAFE_INTEGER` ≈ 9×10¹⁵ — thừa chỗ |
| Cộng nhiều giá, nhân với số lượng, tính thuế | **KHÔNG.** Sai số cộng dồn, và tiền thì không được sai một đồng |

P0.4 chỉ hiển thị, nên `formatVND` nhận chuỗi và chuyển sang `Number` **bên
trong nó** — một chỗ duy nhất, có comment giải thích ranh giới trên. P4 làm giỏ
hàng thì phải dùng thư viện decimal cho mọi phép tính.

⚠️ `formatVND` phải chịu được chuỗi không parse được (trả về nguyên chuỗi thay vì
`NaN ₫`). Dữ liệu hỏng không được làm vỡ cả trang.

---

## 5. Render và cache

| Trang | Cách render |
|---|---|
| `/` trang chủ | chuyển hướng sang `/danh-muc` hoặc trang giới thiệu tối giản |
| `/danh-muc` (có bộ lọc, phân trang) | `dynamic` — vì bộ lọc nằm trên URL |
| `/san-pham/[slug]` | ISR `revalidate: 60` + `tags: ['product:'+slug]` |

Thiết kế 06 chọn ISR cho trang danh mục, nhưng đó là cho **trang danh mục cố
định** (`/laptop`, `/laptop-gaming`). Trang danh sách có bộ lọc tự do trên URL
thì mỗi tổ hợp bộ lọc là một biến thể riêng — render sẵn hết là không khả thi.
P1 tách hai thứ đó ra.

⚠️ **On-demand revalidate để P1.** Nó cần một secret chia sẻ giữa worker Go và
Next.js, và thêm một đường hỏng im lặng nữa phải kiểm. P0.4 chấp nhận trang chi
tiết cũ tối đa 60 giây.

---

## 6. SEO

- `generateMetadata` cho trang chi tiết: `title`, `description`, `openGraph`,
  `alternates.canonical`.
- JSON-LD `Product` với `offers.price`, `priceCurrency`, `availability`.
- `sitemap.ts` liệt kê sản phẩm `live`; `robots.ts` chặn `/admin`.

⚠️ **`sitemap.ts` không được gọi API không phân trang.** `/products` có trần
`max_page = 200`, nên sitemap chỉ lấy được tối đa `200 × 100 = 20.000` sản phẩm.
Ghi giới hạn đó vào code, và P1 làm sitemap phân mảnh (`sitemap/[id].ts`).

---

## 7. Giới hạn đã biết, chấp nhận ở P0.4

Ghi ra để lần sau không mất công điều tra lại:

**Không hiển thị được tên thương hiệu.** `Product` chỉ có `brand_id` (UUID), và
backend **không có endpoint `/brands`**. Trang chi tiết vì thế không hiện được
"ASUS". Lọc theo `?brand=<slug>` thì vẫn chạy vì API nhận slug. **P1 phải thêm
`GET /brands`** — đây là thiếu sót của hợp đồng API, không phải của frontend.

**Không hiển thị được tên danh mục của sản phẩm** — cùng lý do, nhưng nhẹ hơn vì
`/categories` trả cả cây nên tra `category_id` ra tên được bằng cách duyệt cây.

**`attr.<key>` không dùng được qua type đã sinh.** `openapi-typescript` sinh
object `query` đóng, không có index signature. Phải tự nối query string. Đã ghi
trong `openapi.yaml`.

**Sản phẩm `draft` trả 404** — đúng thiết kế (§5.3 của P0.2), nhưng nghĩa là
admin vừa tạo sản phẩm xong mà mở link thì thấy 404 cho tới khi publish.

---

## 8. Kiểm chứng

Không có test tự động. Phần tự động chỉ có: `biome check`, `tsc --noEmit`,
`next build`, và `check-error-messages.sh`.

| # | Kiểm | Kỳ vọng |
|---|---|---|
| 1 | `task check` | 8 bước xanh (thêm `error-messages`) |
| 2 | `npm run build` | Không lỗi, không cảnh báo type |
| 3 | Mở `/danh-muc` | Thấy sản phẩm thật từ Postgres, giá định dạng `25.990.000 ₫` |
| 4 | Lọc theo danh mục | URL đổi, kết quả đúng, **bấm Back quay lại đúng trạng thái trước** |
| 5 | Phân trang | Sang trang 2 rồi trang cuối; `has_next` false ở trang cuối và **không có nút dẫn tới 400** |
| 6 | Chi tiết sản phẩm | Tiếng Việt hiển thị đúng dấu, JSON-LD có trong HTML nguồn |
| 7 | `generateMetadata` | Xem HTML nguồn: `<title>`, `og:title`, `canonical` đúng |
| 8 | Slug không tồn tại | `not-found.tsx`, **không phải** lỗi 500 |
| 9 | **Tắt API Go rồi tải trang** | `error.tsx` với thông điệp tiếng Việt + `request_id`, **không** trang trắng và **không** lộ stack trace |
| 10 | Tab Network | Không request đỏ. Tab Console: không lỗi |
| 11 | `/sitemap.xml`, `/robots.txt` | Trả về hợp lệ, sitemap có URL sản phẩm thật |
| 12 | **Chạy trong Docker** | `task up-docker` lên đủ, mở trang thật, `docker stop` exit 0 |

**Mục 9 quan trọng nhất.** Đó là mục duy nhất kiểm đường lỗi, và nó là thứ khách
thật sẽ gặp. Trang trắng hoặc stack trace lộ ra là hỏng thật.

### 8.1. Đo được khi làm Task 3 — và nó sửa lại chính tài liệu này

React **tước sạch thuộc tính tùy biến** của lỗi ném từ Server Component trước khi
giao cho `error.tsx`. Không phải chỉ ở production như tôi viết ban đầu, mà ở
**cả hai chế độ**:

| | `next dev` | production build |
|---|---|---|
| `error.name` | `ApiError` | `Error` |
| `error.message` | `PRODUCT_NOT_FOUND (HTTP 404)` | `Minified React error #441` |
| `error.code` / `status` / `requestId` | **undefined** | **undefined** |
| `Object.keys(error)` | `name`, `environmentName`, `digest` | `digest` |

Nghĩa là cái bẫy thật không phải "đọc `error.code` chạy được ở dev rồi hỏng ở
prod" — nó **không bao giờ** chạy được. Bẫy thật là **parse `error.message`**:
chuỗi đó còn ở dev nhưng ở prod thành `Minified React error #441`. Và ở prod cả
`error.name` cũng thành `Error`, nên `instanceof ApiError` lẫn
`err.name === 'ApiError'` đều vô dụng.

Kết luận kiến trúc không đổi, chỉ mạnh hơn: **Server Component phải tự bắt
`ApiError`** và render component lỗi của chính nó. `error.tsx` chỉ hiện thông
điệp chung chung cộng `digest` — và `digest` dùng được thật, log server có đúng
cặp `digest ↔ lỗi gốc`.

---

## 9. Docker

`output: 'standalone'` trong `next.config`. Image multi-stage giống backend.

⚠️ **`standalone` KHÔNG tự chép `public/` và `.next/static/`.** Đây là cái bẫy
kinh điển: image build xong, chạy được, nhưng **mất sạch CSS và ảnh** vì hai thư
mục đó không có trong `.next/standalone`. Dockerfile phải chép tay:

```dockerfile
COPY --from=build /src/.next/standalone ./
COPY --from=build /src/.next/static ./.next/static
COPY --from=build /src/public ./public
```

⚠️ Next.js chạy bằng Node nên **không dùng được distroless/static** (ảnh đó không
có Node). Dùng `node:22-alpine` với user không phải root, và `ENTRYPOINT` dạng
exec để binary là PID 1 — cùng lý do đã đo ở [thiết kế 05](../../design/05-deployment.md) mục 4.1.
