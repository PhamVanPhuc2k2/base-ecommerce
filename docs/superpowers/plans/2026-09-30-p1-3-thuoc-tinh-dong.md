# P1.3 — Thuộc tính động: kế hoạch thực hiện

Đặc tả: [`specs/2026-09-30-p1-3-thuoc-tinh-dong.md`](../specs/2026-09-30-p1-3-thuoc-tinh-dong.md).
Nhánh: `feat/p1-3-thuoc-tinh`. Không unit test.

## Task 1 — Migration
- [ ] `attribute_definitions`, `category_attributes`, GIN `product_variants(options)`
- [ ] Up/down/up trên dữ liệu thật

## Task 2 — Domain
- [ ] `AttributeDefinition` (kiểu, validate giá trị), `AttributeCatalog` (định nghĩa + phép gán, có `Validate` cho cache)
- [ ] `Tree.Ancestors`; `AttributeCatalog.SchemaFor(tree, categoryID)` — tập hiệu lực kể cả kế thừa
- [ ] `Schema.CheckProduct(p)` → `errs.Validation` theo từng trường; chế độ tự do khi rỗng
- [ ] Mã lỗi mới → `openapi.yaml` → `errors.ts`

## Task 3 — Repository
- [ ] `queries/attribute.sql`; `AttributeRepository`
- [ ] `ListFilter.VariantOptions` → một `EXISTS`; `Facets` dựa trên `filtered()`
- [ ] Cache: `AttributeCatalog`, `ProductFacets`

## Task 4 — Use case + delivery
- [ ] Ghi sản phẩm/variant/publish: validate theo schema của danh mục hiện tại
- [ ] `ListProducts`/`ListFacets` chia `attr.*` theo định nghĩa
- [ ] CRUD định nghĩa, `PUT` phép gán, `GET /attributes`, `GET /categories/{slug}/attributes`, `GET /products/facets`
- [ ] `openapi.yaml` 0.5.0, `task openapi`

## Task 5 — Storefront
- [ ] Bộ lọc facet trên `/danh-muc`, chuyển tiếp `attr.*` xuống API
- [ ] Bảng thông số + bảng phiên bản dùng tên, đơn vị

## Task 6 — Kiểm chứng + tài liệu + merge, push
