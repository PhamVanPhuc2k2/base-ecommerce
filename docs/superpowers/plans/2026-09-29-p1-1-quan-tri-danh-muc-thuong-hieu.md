# P1.1 — Quản trị danh mục, thương hiệu: kế hoạch thực hiện

Đặc tả: [`specs/2026-09-29-p1-1-quan-tri-danh-muc-thuong-hieu.md`](../specs/2026-09-29-p1-1-quan-tri-danh-muc-thuong-hieu.md).
Nhánh: `feat/p1-1-quan-tri`. Không unit test — mỗi task kết thúc bằng
`task check` xanh và phép kiểm thủ công ghi ngay trong task.

## Task 1 — Domain

- [x] `errors.go`: `CATEGORY_NAME_INVALID`, `BRAND_NAME_INVALID`,
      `DUPLICATE_CATEGORY_SLUG`, `DUPLICATE_BRAND_SLUG`, `CATEGORY_CYCLE`,
      `UNKNOWN_CATEGORY`, `UNKNOWN_BRAND`, `CATEGORY_HAS_CHILDREN`,
      `CATEGORY_HAS_PRODUCTS`, `BRAND_HAS_PRODUCTS`
- [x] `category.go`: `NewCategory`, `Rename`, `SetSlug`, `SetPosition`,
      `Tree.CheckMove`, kiểu `Categories` có `Validate`
- [x] `brand.go`: `NewBrand`, `Rename`, `SetSlug`, kiểu `Brands` có `Validate`
- [x] `product.go`: `Update` nhận `categoryID`, `brandID *uuid.UUID`
- [x] Mã lỗi mới vào `openapi.yaml` + `apps/web/lib/errors.ts` (`task check`
      đỏ nếu quên — chạy để thấy đỏ trước khi thêm)

## Task 2 — Repository

- [x] `queries/category.sql`: `CategoryByID`, `InsertCategory`,
      `UpdateCategory`, `DeleteCategory` (`LOCK TABLE` chạy bằng `Exec` trong
      `LockForWrite` — sqlc không có gì để sinh cho nó)
- [x] `queries/brand.sql`: `AllBrands`, `BrandByID`, `InsertBrand`,
      `UpdateBrand`, `DeleteBrand`
- [x] `UpsertProduct`: thêm `category_id`, `brand_id` vào `DO UPDATE SET`
- [x] `ProductRepository`: tách `Count` khỏi `List`
- [x] `mapErr` theo thao tác cho `categories_parent_id_fkey` (xem đặc tả 2.4)
- [x] `rediscache`: `Brands`, `ProductCount` (`*int`), `CategoryTree` trả `domain.Categories`
- [x] `task sqlc` → `task check` (sqlc-drift xanh)

## Task 3 — Use case

- [x] `CreateCategory`/`UpdateCategory`/`DeleteCategory`: `LockForWrite` →
      đọc cây từ DB → domain kiểm → ghi → sau commit xóa `category:tree`
- [x] `CreateBrand`/`UpdateBrand`/`DeleteBrand`/`ListBrands`: sau commit xóa `brand:all`
- [x] `UpdateProduct` chuyển `CategoryID`/`BrandID`
- [x] `ListProducts`: đếm qua `cache.ProductCount`

## Task 4 — Delivery + hợp đồng

- [x] `optionalUUID` với `UnmarshalJSON` — vắng mặt ≠ null
- [x] Handler + route; `GET /brands` công khai, còn lại sau `RequireAdminKey`
- [x] `openapi.yaml`: path + schema mới; `task openapi` sinh lại type TS
- [x] Ráp trong `cmd/api/wire.go`

## Task 5 — Storefront

- [x] `loadBrands()` trong trang chi tiết (cùng khuôn `loadCategoryTree`), ISR 1 giờ
- [x] Trang chi tiết: tên thương hiệu + JSON-LD `brand`
- [x] `task web-check`

## Task 6 — Kiểm chứng + tài liệu

- [x] 12 mục ở đặc tả §5, trên stack Docker thật
- [x] README (cây mục 4 nếu thêm file, mục 11/12), TIEN-DO
- [x] Merge vào `main`
