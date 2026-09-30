# P1.3 — Thuộc tính động theo danh mục: Thiết kế

> Thuộc P1, xem [tổng quan](2026-09-29-p1-tong-quan.md). Trả lời câu hỏi còn
> mở từ README "Mô hình thuộc tính động (JSONB vs EAV)".

## 1. Phạm vi

**Có:** định nghĩa thuộc tính (kiểu, đơn vị, giá trị cho phép, lọc được, là
thuộc tính biến thể); gán thuộc tính cho danh mục, **kế thừa xuống danh mục
con**; validate `attributes` của sản phẩm và `options` của variant theo định
nghĩa; lọc theo tùy chọn biến thể; API facet đếm số sản phẩm theo từng giá trị;
storefront có bộ lọc facet và bảng thông số dùng tên hiển thị.

**Không:** lọc khoảng số (`RAM ≥ 16`); facet "disjunctive" (đếm mỗi nhóm khi bỏ
lọc của chính nhóm đó); đa ngôn ngữ cho tên thuộc tính.

## 2. Quyết định

### 2.1. Giữ JSONB, thêm định nghĩa — KHÔNG chuyển sang EAV

Giá trị vẫn nằm trong `products.attributes` / `product_variants.options`
(JSONB, index GIN có sẵn, lọc bằng `@>`). Thứ thêm vào là **bảng định nghĩa**
nói khóa nào hợp lệ ở danh mục nào, kiểu gì. EAV (một dòng cho mỗi cặp sản
phẩm–thuộc tính) cho phép ràng buộc kiểu ở tầng database, nhưng mỗi bộ lọc N
thuộc tính thành N lần JOIN — đúng thứ trang danh mục không chịu nổi. Validate
kiểu làm ở domain, lúc ghi.

### 2.2. Danh mục chưa khai thuộc tính nào → vẫn tự do

Tập thuộc tính hiệu lực của một danh mục = thuộc tính gán cho nó **cộng mọi tổ
tiên**. Tập đó rỗng thì hành vi y hệt P0: khóa tự do. Có ít nhất một thì chặt:
khóa lạ bị từ chối. Nhờ vậy migration không làm vỡ 33 sản phẩm đang có, và bật
chế độ chặt là quyết định từng danh mục của người quản trị.

### 2.3. Thuộc tính sản phẩm vs thuộc tính biến thể

Mỗi định nghĩa có cờ `variant`. `variant = false` → chỉ được nằm trong
`Product.attributes` (CPU của một dòng máy). `variant = true` → chỉ được nằm
trong `Variant.options` (RAM, màu — thứ phân biệt các phiên bản). Đặt sai chỗ
là lỗi: một thuộc tính biến thể nằm ở cấp sản phẩm thì bộ lọc sẽ tìm sai bảng.

### 2.4. Kiểu và validate

| Kiểu | Giá trị hợp lệ (vẫn lưu dạng chuỗi) |
|---|---|
| `text` | chuỗi không rỗng, ≤ 200 ký tự |
| `number` | số thập phân thuần: `"16"`, `"15.6"` — KHÔNG kèm đơn vị; đơn vị nằm ở định nghĩa |
| `boolean` | `"true"` / `"false"` |
| `enum` | một trong `options` của định nghĩa |

Lỗi validate trả `422 VALIDATION_FAILED` với `errors[]` theo từng trường
(`attributes.ram`, `variants[1].options.mau`) — một lần gửi báo đủ mọi chỗ sai.

`required`: bắt buộc khi sản phẩm **live** (Publish, và mọi lần ghi khi đang
live) — nháp được phép dở dang, cùng tinh thần P0.2/P1.2. Thuộc tính biến thể
bắt buộc thì mọi variant active phải có.

### 2.5. `code`, `type`, `variant` bất biến

Đổi kiểu `text` → `number` hay chuyển một thuộc tính từ sản phẩm sang biến thể
làm dữ liệu đã lưu sai nghĩa mà không ai được báo. PATCH định nghĩa chỉ nhận
`name`, `unit`, `options`, `filterable`. Muốn đổi thứ khác: tạo định nghĩa mới.

Đổi `options` của enum (bỏ một giá trị đang dùng) được phép: dữ liệu cũ không
bị sửa, sản phẩm đó chỉ báo lỗi ở lần ghi kế tiếp. Chấp nhận — kiểm toàn bảng
mỗi lần sửa định nghĩa là chặn thao tác quản trị trên dữ liệu không liên quan.

### 2.6. Validate lúc ghi, định nghĩa đọc từ cache

Toàn bộ định nghĩa + phép gán nhỏ (vài trăm dòng) → cache một khóa
`attribute:catalog`, vô hiệu hóa sau mọi lần ghi định nghĩa/phép gán. Use case
ghi sản phẩm/variant đọc catalog đó để validate. Cache lệch với database tối đa
bằng khoảng thời gian giữa commit và lệnh xóa cache — chấp nhận được vì validate
là kiểm tra chất lượng dữ liệu, không phải ràng buộc toàn vẹn.

### 2.7. Facet: đếm theo bộ lọc hiện tại, cache 60 giây

`GET /products/facets` nhận đúng bộ lọc của `GET /products`, trả mọi thuộc tính
**lọc được** của danh mục kèm số sản phẩm cho từng giá trị. Đếm "conjunctive"
(áp mọi bộ lọc, kể cả của chính nhóm đó) — đơn giản, đúng ý "còn bao nhiêu nếu
bấm thêm". Không có `category` thì trả rỗng: thuộc tính thuộc về danh mục.
Cache như số đếm (P1.1): khóa băm bộ lọc, TTL 60 giây.

### 2.8. Lọc theo tùy chọn biến thể

`attr.<code>=<v>` đi vào `products.attributes @>` hay vào
`EXISTS (variant active có options @> …)` tùy định nghĩa. Mọi tùy chọn biến thể
gộp vào **một** `EXISTS` — `ram=16GB&mau=Den` nghĩa là *một* phiên bản thỏa cả
hai, không phải một phiên bản 16GB và một phiên bản khác màu đen.

## 3. Schema

```sql
attribute_definitions (id, code UNIQUE, name, type, unit, options TEXT[],
                       filterable, variant, created_at, updated_at)
category_attributes   (category_id → categories ON DELETE CASCADE,
                       attribute_id → attribute_definitions ON DELETE RESTRICT,
                       required, position, PRIMARY KEY (category_id, attribute_id))
+ GIN index trên product_variants(options)
```

## 4. API

| Method | Đường dẫn | |
|---|---|---|
| GET | `/attributes` | công khai, mọi định nghĩa (storefront đổi code → tên, đơn vị) |
| GET | `/categories/{slug}/attributes` | công khai, tập hiệu lực (kể cả kế thừa) |
| GET | `/products/facets` | công khai, cùng tham số với `/products` |
| POST | `/admin/attributes` | tạo định nghĩa |
| PATCH | `/admin/attributes/{id}` | `name`, `unit`, `options`, `filterable` |
| DELETE | `/admin/attributes/{id}` | 409 `ATTRIBUTE_IN_USE` nếu còn gán cho danh mục |
| PUT | `/admin/categories/{id}/attributes` | thay TOÀN BỘ phép gán của danh mục |

## 5. Kiểm chứng

| # | Kiểm | Kỳ vọng |
|---|---|---|
| 1 | `task check`, `task web-check` | xanh |
| 2 | Migration trên dữ liệu thật | 33 sản phẩm không đổi; danh mục chưa khai → sửa sản phẩm vẫn tự do |
| 3 | Gán `cpu` cho `laptop` | `laptop-gaming` (con) thừa kế; `GET /categories/laptop-gaming/attributes` thấy `cpu` |
| 4 | Sản phẩm `laptop-gaming` gửi khóa lạ, số sai, enum sai | 422 `VALIDATION_FAILED`, `errors[]` đủ cả ba trường |
| 5 | Thuộc tính biến thể đặt ở cấp sản phẩm (và ngược lại) | 422, đúng trường |
| 6 | Publish thiếu thuộc tính `required` | 422; sản phẩm nháp thì được lưu |
| 7 | Chuyển sản phẩm sang danh mục chặt | validate theo danh mục MỚI |
| 8 | `attr.ram=16` với `ram` là thuộc tính biến thể | lọc theo variant active; hai tùy chọn = một variant thỏa cả hai |
| 9 | Facet `laptop-gaming` | đếm khớp với `total` khi bấm từng giá trị |
| 10 | Xóa định nghĩa đang gán / PATCH đổi `type` | 409 `ATTRIBUTE_IN_USE` / 400 |
| 11 | Storefront `/danh-muc?category=…` | nhóm facet có số đếm, bấm là lọc, Back hoạt động |
| 12 | Trang chi tiết | bảng thông số hiện tên + đơn vị ("RAM: 16 GB") |
