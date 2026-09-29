# P1.1 — Quản trị danh mục, thương hiệu và trả nợ P0: Thiết kế

> Thuộc P1, xem [tổng quan](2026-09-29-p1-tong-quan.md). Không đổi schema sản
> phẩm — không có migration.

## 1. Phạm vi

**Có:**

- CRUD danh mục (`/admin/categories`) — **chặn vòng lặp kể cả khi hai request
  chạy đồng thời**, và vô hiệu hóa cache cây sau mỗi lần ghi.
- CRUD thương hiệu (`/admin/brands`) + `GET /brands` công khai, có cache.
- `PATCH /admin/products/{id}` nhận thêm `category_id`, `brand_id`.
- Cache số đếm của trang danh sách.
- Storefront hiện tên thương hiệu ở trang chi tiết (chữ + JSON-LD `brand`).

**Không:** sự kiện `category.*`/`brand.*` (xem mục 6), biến thể (P1.2), thuộc
tính động (P1.3), media (P1.4), trang thương hiệu (P1.5).

## 2. Quyết định

### 2.1. Chống vòng lặp: khóa bảng + kiểm trên cây đọc trong transaction

Vòng lặp A → B → A làm **cả nhánh biến mất khỏi `Roots()`** mà không lỗi nào
(thiết kế 04 mục 4). Kiểm "cha mới không nằm trong cây con của chính nó" là
chưa đủ, vì hai request đồng thời đều qua được phép kiểm:

```
T1: đọc cây, A không phải tổ tiên của B → chuyển A vào dưới B   ✓
T2: đọc cây, B không phải tổ tiên của A → chuyển B vào dưới A   ✓
    cả hai commit → A ↔ B, cả nhánh mất
```

Isolation `READ COMMITTED` không cứu được (mỗi bên chỉ sửa một dòng khác nhau),
còn `SERIALIZABLE` cho toàn hệ thống thì quá tay. Chọn: mọi use case **ghi danh
mục** mở transaction bằng

```sql
LOCK TABLE categories IN SHARE ROW EXCLUSIVE MODE
```

rồi mới đọc cây **từ database** (không từ cache) và kiểm. Chế độ khóa này:

| Xung đột với | Hệ quả |
|---|---|
| Chính nó | Hai lần ghi danh mục xếp hàng — đúng cái cần |
| `ROW EXCLUSIVE` (INSERT/UPDATE/DELETE thẳng vào `categories`) | SQL viết tay cũng phải chờ |
| **Không** xung đột với `ACCESS SHARE` (SELECT) | Storefront đọc bình thường |
| **Không** xung đột với `ROW SHARE` (khóa FK khi ghi `products`) | Tạo/sửa sản phẩm không bị chặn |

Ghi danh mục là thao tác quản trị hiếm — xếp hàng tuần tự không tốn gì.

### 2.2. `PATCH parent_id` cần BA trạng thái

| JSON | Nghĩa |
|---|---|
| không có khóa `parent_id` | giữ nguyên |
| `"parent_id": null` | chuyển lên làm danh mục gốc |
| `"parent_id": "<uuid>"` | chuyển vào dưới danh mục đó |

`*uuid.UUID` chỉ có hai trạng thái (nil / có giá trị) — "vắng mặt" và "null"
đều thành nil, và một lệnh `PATCH {"name": "..."}` sẽ **âm thầm chuyển danh mục
lên gốc**. Đây đúng là loại lỗi đã gặp ở P0.2 (`short_description` bị xóa).
Dùng kiểu `optionalUUID` có `UnmarshalJSON` riêng: phương thức đó chỉ chạy khi
khóa CÓ MẶT, nên `Set=false` nghĩa là vắng mặt.

### 2.3. Slug danh mục/thương hiệu KHÔNG đổi theo tên

Khác sản phẩm (đổi tên là đổi slug). Slug danh mục nằm trong URL bộ lọc
(`?category=laptop`) và sẽ là URL trang danh mục ở P1.5 — đổi tên để sửa chính
tả mà mất hết link đã được index là cái giá quá đắt. Slug sinh từ tên lúc tạo
(hoặc client gửi), và chỉ đổi khi client gửi `slug` tường minh. Slug client gửi
vẫn đi qua `NewSlug` để chuẩn hóa (bỏ dấu, chữ thường).

### 2.4. Xóa: chặn, không xóa lan

`ON DELETE RESTRICT` đã có ở cả hai khóa ngoại. Xóa danh mục còn con → 409
`CATEGORY_HAS_CHILDREN`; còn sản phẩm (**kể cả sản phẩm đã xóa mềm**, vì dòng vẫn
tồn tại) → 409 `CATEGORY_HAS_PRODUCTS`. Tương tự `BRAND_HAS_PRODUCTS`.

⚠️ `categories_parent_id_fkey` báo lỗi 23503 ở **cả hai chiều**: INSERT với cha
không tồn tại, và DELETE khi còn con. Cùng tên ràng buộc, nghĩa ngược nhau.
Repository biết mình đang làm thao tác nào nên map theo thao tác, không theo
tên ràng buộc.

### 2.5. Cache số đếm: `*int`, TTL 60 giây, không vô hiệu hóa

Khóa `product:count:<sha256 của bộ lọc chuẩn hóa>` — bộ lọc KHÔNG gồm `page`,
`limit`, `sort` (không đổi số đếm). Giữ tối ưu cũ: trang 1 chưa đầy thì không
đếm.

**Phải là `*int`, không phải `int`.** `decodeCached` từ chối `null` bằng cách
kiểm con trỏ nil — nhưng `json.Unmarshal("null", &n)` với `n int` để nguyên 0
và **không báo lỗi**. Cache hỏng thành `null` sẽ ra `total: 0`, `has_next:
false`, phân trang biến mất trong khi dữ liệu vẫn còn.

Không vô hiệu hóa khi ghi sản phẩm: số đếm lệch tối đa 60 giây nghĩa là trang
cuối có thể thiếu/thừa một sản phẩm vừa đăng — chấp nhận được, rẻ hơn nhiều so
với theo dõi mọi bộ lọc bị ảnh hưởng bởi một lần ghi.

### 2.6. Cache danh sách phải từ chối phần tử `null`

`GetOrLoad` từ chối giá trị `null` ở cấp ngoài cùng, nhưng `[]*Category` với
nội dung `[null]` giải mã thành một slice có phần tử nil → `NewTree` deref →
**panic**. Lỗ này có từ P0.2. Sửa kèm: kiểu `domain.Categories` và
`domain.Brands` có `Validate()` kiểm từng phần tử (không nil, có ID, slug, tên).

## 3. API

| Method | Đường dẫn | Trả về | Lỗi đặc thù |
|---|---|---|---|
| GET | `/brands` | 200 `{data: Brand[]}` theo tên | — |
| POST | `/admin/categories` | 201 `Category` | `CATEGORY_NAME_INVALID`, `INVALID_SLUG`, `DUPLICATE_CATEGORY_SLUG`, `CATEGORY_NOT_FOUND` (cha) |
| PATCH | `/admin/categories/{id}` | 200 `Category` | trên + `UNKNOWN_CATEGORY` (404), `CATEGORY_CYCLE` (422) |
| DELETE | `/admin/categories/{id}` | 204 | `UNKNOWN_CATEGORY`, `CATEGORY_HAS_CHILDREN`, `CATEGORY_HAS_PRODUCTS` (409) |
| POST | `/admin/brands` | 201 `Brand` | `BRAND_NAME_INVALID`, `INVALID_SLUG`, `DUPLICATE_BRAND_SLUG` |
| PATCH | `/admin/brands/{id}` | 200 `Brand` | trên + `UNKNOWN_BRAND` (404) |
| DELETE | `/admin/brands/{id}` | 204 | `UNKNOWN_BRAND`, `BRAND_HAS_PRODUCTS` (409) |
| PATCH | `/admin/products/{id}` | thêm `category_id`, `brand_id` | `CATEGORY_NOT_FOUND`, `BRAND_NOT_FOUND` (422, đã có) |

**Vì sao có `UNKNOWN_CATEGORY` riêng** mà không dùng `CATEGORY_NOT_FOUND`: mã
cũ là 422 và nghĩa là "sản phẩm tham chiếu danh mục không tồn tại" — frontend
đang dùng nó cho `?category=<slug lạ>`. Tài nguyên trên đường dẫn không tồn tại
là 404, nghĩa khác, nên mã khác. Đổi nghĩa mã cũ là phá hợp đồng.

## 4. Tầng

| Tầng | Thêm |
|---|---|
| `domain` | `NewCategory`, `Category.Rename/SetSlug/SetPosition`, `Tree.CheckMove` (→ `ErrCategoryCycle`), `NewBrand`, `Brand.Rename/SetSlug`, `Categories`/`Brands` có `Validate`, `Product.Update` nhận `categoryID`/`brandID` |
| `usecase` | `CreateCategory`, `UpdateCategory`, `DeleteCategory`, `CreateBrand`, `UpdateBrand`, `DeleteBrand`, `ListBrands`; `ListProducts` tách đếm ra và đi qua cache |
| `repository` | `CategoryRepository.LockForWrite/ByID/Insert/Update/Delete`, `BrandRepository` mới, `ProductRepository.Count`; `UpsertProduct` thêm `category_id`, `brand_id` vào `DO UPDATE SET` |
| `delivery` | handler + route admin, `optionalUUID`, DTO `brandDTO` |
| web | `getBrands()` (ISR 1 giờ), tên thương hiệu + JSON-LD `brand` ở trang chi tiết, thông điệp tiếng Việt cho mã lỗi mới |

## 5. Kiểm chứng

| # | Kiểm | Kỳ vọng |
|---|---|---|
| 1 | `task check`, `task web-check` | xanh |
| 2 | Tạo gốc A, con B dưới A, cháu C dưới B | `GET /categories` hiện ngay cây mới — **không chờ hết TTL 6 giờ** |
| 3 | Chuyển A vào dưới C | 422 `CATEGORY_CYCLE`; cây không đổi |
| 4 | **Hai PATCH đồng thời** X→dưới Y và Y→dưới X (lặp nhiều lần) | Đúng một bên thắng, bên kia `CATEGORY_CYCLE`; không bao giờ ra vòng lặp |
| 5 | `PATCH {"name": ...}` không có `parent_id` | Danh mục **ở nguyên** chỗ cũ |
| 6 | `PATCH {"parent_id": null}` | Lên làm gốc |
| 7 | Xóa danh mục còn con / còn sản phẩm | 409 đúng mã |
| 8 | CRUD thương hiệu, `GET /brands` sau khi sửa | Thấy ngay tên mới |
| 9 | `PATCH` sản phẩm đổi `category_id` | Sản phẩm xuất hiện trong danh sách của danh mục mới |
| 10 | Gọi trang 2 hai lần | Lần hai không chạy `count(*)` (khóa `product:count:*` có trong Redis) |
| 11 | Bơm `null` / `[null]` vào khóa cache số đếm và cây | WARN trong log, API vẫn đúng, không panic |
| 12 | Trang chi tiết sản phẩm | Hiện tên thương hiệu; JSON-LD có `brand.name` |

## 6. Vì sao chưa phát sự kiện `category.*`/`brand.*`

Chưa có consumer nào cần (P7 search sẽ cần, và sẽ đánh chỉ mục lại từ đầu được).
Phát sớm mà quên thêm binding thì relay thử lại mãi — xem tổng quan mục 4. Khi
P7 cần: thêm event, thêm binding `category.*`/`brand.*`, trong cùng một thay đổi.
