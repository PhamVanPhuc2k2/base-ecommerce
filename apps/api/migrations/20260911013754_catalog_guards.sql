-- +goose Up
-- Một UPDATE là đủ tạo vòng lặp trong cây danh mục, và khóa ngoại không ngăn
-- được. Hậu quả tệ hơn treo tiến trình: NewTree chỉ đưa danh mục vào roots khi
-- parent_id IS NULL, nên mọi thành viên của vòng lặp BIẾN MẤT khỏi Roots() —
-- điều hướng cửa hàng mất nguyên một nhánh mà không có lỗi nào.
--
-- CHECK này chỉ chặn được vòng lặp một node (tự làm cha mình). Vòng lặp nhiều
-- node vẫn tạo được bằng SQL và là rủi ro đã ghi nhận trong
-- docs/design/04-kiem-chung.md mục 4 — endpoint quản trị danh mục ở P1 phải
-- tự kiểm tổ tiên trước khi ghi.
ALTER TABLE categories
    ADD CONSTRAINT categories_no_self_parent CHECK (parent_id <> id) NOT VALID;
ALTER TABLE categories VALIDATE CONSTRAINT categories_no_self_parent;

-- currency tồn tại để chuẩn bị cho đa tiền tệ, nhưng index (category_id, price)
-- và bộ lọc price_min/price_max đều so sánh số trần, không có điều kiện tiền tệ
-- nào. Thêm một loại tiền thứ hai mà không sửa hai chỗ đó là sai âm thầm: 500
-- USD được sắp xếp lẫn với 500 VND.
--
-- CHECK này biến giả định ngầm thành ràng buộc rõ ràng. Migration đa tiền tệ
-- sau này BẮT BUỘC phải gỡ nó, và đúng lúc đó sẽ có người phải nhìn lại index
-- cùng bộ lọc.
ALTER TABLE products
    ADD CONSTRAINT products_currency_vnd CHECK (currency = 'VND') NOT VALID;
ALTER TABLE products VALIDATE CONSTRAINT products_currency_vnd;

-- Sắp xếp mặc định của trang danh sách là created_at DESC, và category là tham
-- số tùy chọn — nên GET /api/v1/products không lọc gì sẽ quét tuần tự toàn bảng.
-- Đo ở 200k dòng: 4961 buffer / 18,4 ms cho danh sách cộng 15,0 ms cho count(*)
-- chạy mỗi lần gọi. Có index này: Index Only Scan, 5 buffer, 0,048 ms.
CREATE INDEX products_live_newest_idx ON products (created_at DESC, id DESC)
    WHERE deleted_at IS NULL AND status = 'live';

-- +goose Down
DROP INDEX IF EXISTS products_live_newest_idx;
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_currency_vnd;
ALTER TABLE categories DROP CONSTRAINT IF EXISTS categories_no_self_parent;
