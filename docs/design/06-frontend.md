# 06 — Frontend (Next.js)

---

## 1. Stack — đã chốt

| Hạng mục | Chọn | Lý do |
|---|---|---|
| Framework | **Next.js App Router** | SSR/ISR cho SEO — nguồn traffic chính |
| Ngôn ngữ | **TypeScript**, `strict: true` | |
| CSS | **Tailwind CSS** | Không phải đặt tên class, không có CSS chết, dễ giữ nhất quán |
| Component | **shadcn/ui** | Copy vào repo → sở hữu code, sửa được, không bị khóa version |
| Icon | **lucide-react** | Đi kèm shadcn |
| Data phía server | `fetch` trong Server Component | Mặc định cho mọi thứ cần SEO |
| Data phía client | **TanStack Query** | Chỉ cho giỏ hàng, tài khoản, admin |
| State trên URL | **nuqs** | Bộ lọc, sắp xếp, phân trang |
| State toàn cục | **Zustand** | Chỉ giỏ hàng + UI state. Không dùng cho dữ liệu server |
| Form | **react-hook-form + zod** | |
| Type API | **openapi-typescript** | Sinh từ `api/openapi.yaml` |
| Kiểm chứng | **Thủ công** — trình duyệt + tab Network | Dự án không dùng test tự động |
| Format/lint | **Biome** | Nhanh hơn ESLint + Prettier, một công cụ thay hai |

**Không dùng:** Redux (thừa cho nhu cầu này) · axios (`fetch` đã đủ và tích hợp với
cache của Next) · CSS-in-JS runtime (emotion, styled-components — không hợp Server
Component) · component library đóng gói sẵn kiểu MUI/Antd (khó đổi giao diện theo
thương hiệu, bundle lớn).

---

## 2. Cấu trúc thư mục

```
apps/web/
├── app/
│   ├── (shop)/                       # storefront — có SEO
│   │   ├── layout.tsx                # header, footer, breadcrumb
│   │   ├── page.tsx                  # trang chủ
│   │   ├── [category]/page.tsx       # danh mục + bộ lọc
│   │   ├── san-pham/[slug]/page.tsx  # chi tiết sản phẩm
│   │   ├── tim-kiem/page.tsx
│   │   ├── gio-hang/page.tsx
│   │   └── thanh-toan/page.tsx
│   ├── (account)/                    # tài khoản — cần đăng nhập
│   ├── admin/                        # quản trị — noindex
│   ├── api/
│   │   └── revalidate/route.ts       # nhận webhook từ worker Go
│   ├── sitemap.ts   robots.ts
│   ├── error.tsx    not-found.tsx    # bắt buộc có
│   └── layout.tsx
├── components/
│   ├── ui/                           # shadcn/ui — không sửa logic, chỉ sửa style
│   ├── product/  cart/  layout/      # component theo nghiệp vụ
├── lib/
│   ├── api/
│   │   ├── generated/                # openapi-typescript — KHÔNG sửa tay
│   │   ├── client.ts                 # fetch wrapper: base URL, cookie, lỗi
│   │   └── server.ts                 # bản dùng trong Server Component
│   ├── errors.ts                     # map code lỗi → tiếng Việt
│   ├── format.ts                     # tiền VND, ngày giờ
│   ├── seo.ts                        # helper JSON-LD
│   └── cart-store.ts                 # Zustand
└── next.config.js                    # output: 'standalone'
```

---

## 3. Server Component là mặc định

`"use client"` chỉ khi cần state, event handler hoặc API trình duyệt. **Đẩy ranh
giới client xuống càng sâu càng tốt.**

```tsx
// ĐÚNG — trang là server, chỉ nút là client
export default async function ProductPage({ params }) {
  const product = await api.getProduct(params.slug)   // chạy trên server
  return (
    <article>
      <ProductGallery images={product.images} />       {/* server */}
      <ProductSpecs attributes={product.attributes} /> {/* server */}
      <AddToCartButton productId={product.id} />       {/* client */}
    </article>
  )
}
```

Đặt `"use client"` ở `ProductPage` sẽ kéo toàn bộ cây con sang client: mất SSR cho
nội dung cần index, và tăng bundle bằng cả thư viện gallery.

---

## 4. Chiến lược render

| Trang | Cách render | Lý do |
|---|---|---|
| Trang chủ | ISR `revalidate: 300` | Dữ liệu đổi chậm |
| Danh mục | ISR `revalidate: 60` + tag theo danh mục | SEO, có phân trang |
| Chi tiết sản phẩm | ISR + **on-demand revalidate** theo tag | Giá/tồn kho phải mới |
| Tìm kiếm | `dynamic` | Không index kết quả tìm kiếm |
| Giỏ hàng, thanh toán | `dynamic`, client | Riêng tư |
| Tài khoản | `dynamic`, client | |
| Admin | `dynamic`, `robots: noindex` | |

**On-demand revalidate:** worker Go nghe `product.updated` → gọi
`POST /api/revalidate` của Next.js kèm secret trong header → `revalidateTag('product:'+id)`.
Nhờ vậy sửa giá ở admin thì trang public mới ngay, không phải chờ hết TTL.

⚠️ **Tồn kho không đi qua ISR.** Trang render sẵn hiển thị thông tin tĩnh; trạng
thái còn/hết hàng lấy bằng một request client nhỏ lúc mount. Render sẵn "còn hàng"
rồi cache 60 giây là cách tạo ra đơn phải hủy.

---

## 5. Gọi API

### 5.1. Type sinh từ OpenAPI

```ts
import type { paths } from '@/lib/api/generated'
type Product = paths['/products/{slug}']['get']['responses']['200']['content']['application/json']
```

Backend đổi trường mà quên sửa web → **lỗi compile**, không phải `undefined` lúc chạy.

### 5.2. Wrapper

```ts
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    credentials: 'include',
  })

  if (!res.ok) {
    const problem = await res.json().catch(() => null)
    throw new ApiError(
      problem?.code ?? 'UNKNOWN',
      problem?.request_id,
      res.status,
    )
  }
  return res.json()
}
```

`ApiError` mang theo `code` và `request_id`. Giao diện hiển thị thông điệp tiếng
Việt tra từ `code`, và hiện `request_id` nhỏ ở góc để khách báo lỗi có mã tra cứu.

Trong Server Component dùng bản `server.ts` — tự chuyển tiếp cookie xác thực và
gắn `next: { tags, revalidate }`.

### 5.3. Khi nào dùng TanStack Query

Chỉ cho dữ liệu **không cần SEO và thay đổi theo thao tác người dùng**: giỏ hàng,
danh sách đơn của tôi, bảng trong admin.

Không dùng để tải lại thứ Server Component đã render. Đó là làm hai lần cùng một việc.

---

## 6. Quản lý state

**Thứ tự ưu tiên: URL → server → local state → global store.**

```tsx
// Bộ lọc PHẢI ở URL
const [brand, setBrand] = useQueryState('brand')
const [page, setPage]   = useQueryState('page', parseAsInteger.withDefault(1))
```

Lý do bắt buộc: chia sẻ được link đã lọc, nút back hoạt động đúng, và Google index
được `?brand=asus`. Nhét bộ lọc vào `useState` là mất cả ba.

**Zustand chỉ giữ:** số lượng trong giỏ (để hiện badge ngay lập tức), trạng thái
mở/đóng của drawer và modal. Không giữ danh sách sản phẩm, không giữ thông tin
người dùng.

---

## 7. Xác thực

- Access token + refresh token trong **cookie `HttpOnly`, `Secure`, `SameSite=Lax`**.
  Không lưu ở `localStorage` — đó là lỗ hổng XSS trực tiếp.
- `middleware.ts` chặn `(account)` và `admin`, chuyển hướng về trang đăng nhập.
- Server Component chuyển tiếp cookie khi gọi API.
- Refresh token xoay vòng: mỗi lần dùng sinh token mới, thu hồi token cũ. Phát hiện
  token cũ được dùng lại → thu hồi cả chuỗi (dấu hiệu bị đánh cắp).
- Đăng xuất xóa cookie **và** thu hồi refresh token ở server.

---

## 8. SEO

Đây là nguồn traffic chính, làm ngay từ P0 chứ không phải sau.

**Metadata mỗi trang:**
```ts
export async function generateMetadata({ params }): Promise<Metadata> {
  const p = await api.getProduct(params.slug)
  return {
    title: `${p.name} - Giá tốt tại ...`,
    description: p.short_description,
    alternates: { canonical: `/san-pham/${p.slug}` },
    openGraph: { images: [p.images[0]?.url], type: 'website' },
    robots: p.status === 'live' ? undefined : { index: false },
  }
}
```

**JSON-LD bắt buộc:** `Product` + `Offer` (kèm `availability`, `price`,
`priceCurrency: "VND"`) · `BreadcrumbList` · `Organization` và `WebSite` ở layout gốc ·
`AggregateRating` khi đã có đánh giá (từ giai đoạn 3).

**Khác:**
- URL tiếng Việt không dấu, có nghĩa: `/laptop-gaming/asus-rog-strix-g16`
- Đúng một `<h1>` mỗi trang
- `sitemap.ts` sinh động từ API, chia file khi > 50.000 URL
- Sản phẩm ngừng kinh doanh: **301** về danh mục cha, không trả 404
- Trang danh mục lọc: `canonical` trỏ về trang danh mục gốc để tránh trùng nội dung

---

## 9. Định dạng dữ liệu

```ts
// Tiền: backend trả CHUỖI, không bao giờ ép sang number để tính toán
export const formatVND = (v: string) =>
  new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' })
    .format(Number(v))          // chỉ ép khi hiển thị, không khi tính

// Ngày: backend trả UTC, hiển thị theo giờ VN
export const formatDate = (iso: string) =>
  new Intl.DateTimeFormat('vi-VN', {
    dateStyle: 'short', timeStyle: 'short', timeZone: 'Asia/Ho_Chi_Minh',
  }).format(new Date(iso))
```

Mọi phép tính tiền (tổng giỏ hàng, giảm giá) **do backend tính**. Frontend chỉ hiển
thị. Tính ở hai nơi là chắc chắn có ngày lệch nhau.

---

## 10. Hiệu năng

**Mục tiêu Core Web Vitals** (đo bằng dữ liệu thật, không phải Lighthouse máy dev):
LCP < 2.5s · INP < 200ms · CLS < 0.1

- `next/image` + custom loader trỏ imgproxy. Bắt buộc có `sizes`; ảnh chính của
  trang sản phẩm đặt `priority`
- Ảnh phải khai `width`/`height` để không nhảy layout (CLS)
- Font: `next/font` với `display: swap`, chỉ nạp subset `latin` + `vietnamese`
- **Ngân sách JS:** trang sản phẩm < 150 KB gzip. Kiểm tra trong CI, vượt thì fail
- Skeleton trong `loading.tsx` phải đúng kích thước nội dung thật
- `dynamic()` cho thành phần nặng dưới màn hình đầu (gallery ảnh, tab thông số)

---

## 11. Xử lý lỗi & trạng thái rỗng

Mỗi route group có `error.tsx` (kèm nút thử lại) và `loading.tsx`.
`not-found.tsx` gợi ý sản phẩm liên quan thay vì trang trắng.

Lỗi API hiển thị thông điệp tiếng Việt tra từ `code` — người dùng không bao giờ
được thấy chuỗi lỗi kỹ thuật. Mã `request_id` hiện nhỏ ở góc để hỗ trợ tra cứu.

Trạng thái rỗng (giỏ trống, không có kết quả lọc) phải có hướng dẫn hành động tiếp
theo, không để trống.

---

## 12. Khả năng tiếp cận

Dùng thẻ ngữ nghĩa (`<nav>`, `<main>`, `<article>`) · mọi ảnh có `alt` (ảnh sản
phẩm dùng tên sản phẩm) · thao tác được bằng bàn phím, có viền focus rõ · độ tương
phản ≥ 4.5:1 · thông báo thêm vào giỏ dùng `aria-live`.

Không chỉ vì đúng đắn — Google dùng nhiều tín hiệu trong số này để xếp hạng.

---

## 13. Kiểm chứng

Không có test tự động. Sau mỗi thay đổi, tự chạy các luồng dưới đây trên trình
duyệt, mở sẵn tab **Network** (không được có request đỏ) và **Console** (không
được có lỗi):
1. Danh mục → lọc theo thương hiệu → URL đổi → kết quả đúng
2. Chi tiết sản phẩm → thêm vào giỏ → badge tăng
3. (từ P4) Đặt hàng đến khi có mã đơn

---

## 14. Việc cần làm

- [ ] Khởi tạo Next.js + TypeScript strict + Tailwind + shadcn/ui + Biome
- [ ] `next.config.js`: `output: 'standalone'`, cấu hình image loader
- [ ] `lib/api/client.ts` + `server.ts` + `ApiError`
- [ ] `lib/errors.ts` map code → tiếng Việt (CI kiểm khớp enum trong OpenAPI)
- [ ] `lib/format.ts`: `formatVND`, `formatDate`
- [ ] `lib/seo.ts`: helper JSON-LD cho Product, Breadcrumb, Organization
- [ ] Layout: header, footer, breadcrumb, mega menu danh mục
- [ ] Trang danh mục: bộ lọc trên URL bằng nuqs, phân trang có số trang
- [ ] Trang chi tiết sản phẩm: gallery, thông số, `generateMetadata`, JSON-LD
- [ ] `AddToCartButton` + Zustand store + badge giỏ hàng
- [ ] `error.tsx`, `loading.tsx`, `not-found.tsx` cho từng route group
- [ ] `sitemap.ts`, `robots.ts`
- [ ] `app/api/revalidate/route.ts` (xác thực bằng secret)
- [ ] Kiểm tra ngân sách bundle trong CI
- [ ] Chạy tay 3 luồng ở mục 13 sau mỗi thay đổi lớn
