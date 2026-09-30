# P1.3 — Thuộc tính động: kế hoạch thực hiện

Đặc tả: [`specs/2026-09-30-p1-3-thuoc-tinh-dong.md`](../specs/2026-09-30-p1-3-thuoc-tinh-dong.md).
Nhánh: `feat/p1-3-thuoc-tinh`. Không unit test.

## Task 1 — Migration
- [x] `attribute_definitions`, `category_attributes`, GIN `product_variants(options)`
- [x] Up/down/up trên dữ liệu thật

## Task 2 — Domain
- [x] `AttributeDefinition` (kiểu, validate giá trị), `AttributeCatalog` (định nghĩa + phép gán, có `Validate` cho cache)
- [x] `Tree.Ancestors`; `AttributeCatalog.SchemaFor(tree, categoryID)` — tập hiệu lực kể cả kế thừa
- [x] `Schema.CheckProduct(p)` → `errs.Validation` theo từng trường; chế độ tự do khi rỗng
- [x] Mã lỗi mới → `openapi.yaml` → `errors.ts`

## Task 3 — Repository
- [x] `queries/attribute.sql`; `AttributeRepository`
- [x] `ListFilter.VariantOptions` → một `EXISTS`; `Facets` dựa trên `filtered()`
- [x] Cache: `AttributeCatalog`, `ProductFacets`

## Task 4 — Use case + delivery
- [x] Ghi sản phẩm/variant/publish: validate theo schema của danh mục hiện tại
- [x] `ListProducts`/`ListFacets` chia `attr.*` theo định nghĩa
- [x] CRUD định nghĩa, `PUT` phép gán, `GET /attributes`, `GET /categories/{slug}/attributes`, `GET /products/facets`
- [x] `openapi.yaml` 0.5.0, `task openapi`

## Task 5 — Storefront
- [x] Bộ lọc facet trên `/danh-muc`, chuyển tiếp `attr.*` xuống API
- [x] Bảng thông số + bảng phiên bản dùng tên, đơn vị

## Task 6 — Kiểm chứng + tài liệu + merge, push ✅ (12/12, xem TIEN-DO)
