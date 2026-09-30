# P1.5 — SEO & storefront: kế hoạch thực hiện

Đặc tả: [`specs/2026-09-30-p1-5-seo-storefront.md`](../specs/2026-09-30-p1-5-seo-storefront.md).
Nhánh: `feat/p1-5-seo`. Không unit test.

## Task 1 — Sitemap phía API
- [x] Đo 200.000 sản phẩm giả TRƯỚC khi chọn index; migration `sitemap_index`
- [x] `GET /sitemap/products`, hợp đồng 0.7.0

## Task 2 — Danh sách dùng chung
- [x] `lib/categories.ts`, `CategoryFilter` nhận `hrefFor`, `ProductListing`
- [x] `/danh-muc` (route group cùng `loading.tsx`), `/danh-muc/[slug]`, `/thuong-hieu[/slug]`
- [x] `?category=` → 308 qua `proxy.ts`

## Task 3 — Trang chi tiết
- [x] `VariantSelector`; link breadcrumb/thương hiệu theo đường dẫn mới; `priority` → `preload`
- [x] `BreadcrumbList` JSON-LD trong `Breadcrumb`

## Task 4 — Sitemap phía web
- [x] `/sitemap.xml` (index), `/sitemaps/[file]`; bỏ `app/sitemap.ts`

## Task 5 — Kiểm chứng + tài liệu + merge, push
- [x] 12/12 mục đặc tả §4 (xem TIEN-DO); README, TIEN-DO
