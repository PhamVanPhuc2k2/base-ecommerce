# P1.5 — SEO & storefront: Thiết kế

> Giai đoạn cuối của P1, xem [tổng quan](2026-09-29-p1-tong-quan.md).

## 1. Phạm vi

**Có:**

1. **Trang danh mục theo đường dẫn** `/danh-muc/<slug>` và **trang thương hiệu**
   `/thuong-hieu/<slug>` (+ trang liệt kê `/thuong-hieu`). `?category=` cũ
   chuyển hướng 308 sang đường dẫn mới — đúng lời hứa của đặc tả P1.1 mục 2.3.
2. **Bộ chọn phiên bản** có tương tác trên trang chi tiết.
3. **`BreadcrumbList` JSON-LD** trên mọi trang có breadcrumb.
4. **Sitemap phân mảnh**: bỏ trần 20.000 sản phẩm của P0.4.
5. Dọn: `<Image priority>` (deprecated ở Next 16) → `preload`.

**Không:** revalidate theo sự kiện (worker gọi `revalidateTag`) — cần worker
biết địa chỉ storefront, để P7 làm cùng đánh chỉ mục tìm kiếm.

### Điều chỉnh so với tổng quan: HOÃN shadcn/ui

Storefront hiện chỉ có link, bảng và vài nút — không có form, dialog, dropdown,
là thứ shadcn/ui thật sự đỡ việc. Chúng tới cùng giỏ hàng (P4) và giao diện
quản trị (sau P2). Cài bây giờ là thêm Radix, `cva`, `tailwind-merge` và viết
lại token màu trong `globals.css` cho vài cái nút tự viết được bằng Tailwind.

## 2. Quyết định

### 2.1. Một component danh sách dùng chung, ba trang

`/danh-muc`, `/danh-muc/<slug>`, `/thuong-hieu/<slug>` khác nhau ở đúng ba thứ:
tham số cố định gửi xuống API, đường dẫn gốc cho link lọc/phân trang, và tiêu
đề/breadcrumb. Phần còn lại (gọi API, facet, sắp xếp, phân trang, trạng thái
rỗng, xử lý lỗi) tách thành `ProductListing` — ba bản chép tay sẽ lệch nhau ở
đúng những chỗ đã khó làm đúng (lỗi phải bắt tại trang, `allSettled`).

### 2.2. `?category=` → 308, không giữ song song

Hai URL cho cùng một nội dung chia đôi tín hiệu xếp hạng. 308 (vĩnh viễn, giữ
method) chuyển cả link cũ đã được index sang URL mới, kèm mọi tham số khác.

Làm trong `proxy.ts`, KHÔNG bằng `permanentRedirect()` trong page — đã đo:
`loading.tsx` xả HTTP 200 trước, redirect thành meta refresh (xem TIEN-DO).

### 2.3. Bộ chọn phiên bản không đọc/ghi URL

Trang chi tiết là ISR 60 giây. Đọc `searchParams` ở server làm trang thành
động; `useSearchParams` ở client đẩy phần trang phía trên Suspense gần nhất
sang render ở trình duyệt. Cả hai đều đánh đổi thứ quan trọng hơn (tốc độ,
HTML đầy đủ cho Google) lấy việc chia sẻ link đúng một phiên bản. HTML server
render vẫn có MỌI tùy chọn và giá mặc định; JSON-LD `AggregateOffer` mang khoảng
giá cho Google.

Giá trị không đi được với lựa chọn hiện tại hiện MỜ nhưng vẫn bấm được: bấm
vào thì nhảy sang phiên bản có giá trị đó, giữ nhiều lựa chọn cũ nhất. (Bản đầu
của đặc tả này ghi "vô hiệu hẳn" — đổi khi viết code: vô hiệu thì khách bị kẹt,
không hiểu vì sao "Bạc" bấm không được khi đang chọn 16GB.) Giá trị không có ở
phiên bản nào thì không hiện.

### 2.4. Sitemap: Route Handler, tính lúc chạy — không `generateSitemaps`

`generateSitemaps` của Next không tự sinh sitemap index, và danh sách shard có
thể bị tính lúc build — lúc đó API chưa chạy. Tự viết:

```
/sitemap.xml                 sitemap INDEX: trỏ tới các file dưới
/sitemaps/pages.xml          trang chủ, /danh-muc, mọi danh mục, mọi thương hiệu
/sitemaps/products-<n>.xml   5.000 sản phẩm mỗi file
```

API mới `GET /sitemap/products?page=N` (5.000 dòng/trang, chỉ `slug` +
`updated_at`) — **sắp theo `id`**, không theo ngày tạo giảm dần: UUIDv7 tăng
theo thời gian, nên sản phẩm mới luôn rơi vào file CUỐI, các file cũ không bị
dồn dịch mỗi khi có hàng mới (Google không phải cào lại cả bộ). Index partial
`(id) INCLUDE (slug, updated_at)` cho Index Only Scan; OFFSET trên index hẹp
này đo được trước khi chốt (mục 4).

## 3. API

| Method | Đường dẫn | |
|---|---|---|
| GET | `/sitemap/products?page=N` | `{data: [{slug, updated_at}], meta: {page, page_size, total, total_pages}}` |

## 4. Kiểm chứng

| # | Kiểm | Kỳ vọng |
|---|---|---|
| 1 | `task check`, `task web-check` | xanh |
| 2 | `/danh-muc?category=laptop&sort=price_asc` | 308 → `/danh-muc/laptop?sort=price_asc` |
| 3 | `/danh-muc/laptop` | danh sách + facet + breadcrumb theo cây; canonical đúng |
| 4 | `/danh-muc/khong-co`, `/thuong-hieu/khong-co` | HTTP 404 |
| 5 | `/thuong-hieu/<slug>` | chỉ sản phẩm của hãng; title/h1 có tên hãng |
| 6 | Bộ chọn phiên bản (Chrome) | bấm tùy chọn → giá + SKU đổi; tổ hợp không có bị vô hiệu |
| 7 | JSON-LD `BreadcrumbList` | có trên chi tiết, danh mục, thương hiệu; URL tuyệt đối |
| 8 | `/sitemap.xml` | sitemap index hợp lệ, trỏ `pages.xml` + đủ số file sản phẩm |
| 9 | 200.000 sản phẩm giả (trong transaction rồi ROLLBACK) | EXPLAIN trang sitemap cuối: Index Only Scan, thời gian chấp nhận được |
| 10 | Mọi URL trong sitemap | trả 200 |
| 11 | API chết | `/sitemap.xml` vẫn ra XML hợp lệ (tối thiểu) |
| 12 | Tab Network/Console | sạch |
