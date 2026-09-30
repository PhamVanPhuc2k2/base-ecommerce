-- P1.5 — index cho sitemap phân mảnh (GET /sitemap/products).
--
-- Đo trên 200.000 sản phẩm live, trang cuối (OFFSET 195.000, LIMIT 5.000):
--   không có index này : Seq Scan + sort TRÀN RA ĐĨA (temp written 1229 trang)
--   có index này       : Index Only Scan, Heap Fetches 0, 1.432 buffer
-- INCLUDE (slug, updated_at) là thứ cho phép "Only": câu sitemap chỉ cần đúng
-- ba cột, không phải đọc dòng trong heap. Sắp theo id (UUIDv7 tăng theo thời
-- gian) để sản phẩm mới luôn rơi vào file sitemap CUỐI — xem đặc tả P1.5 mục 2.4.
--
-- Không CONCURRENTLY: goose chạy mỗi migration trong transaction, và bảng ở
-- quy mô hiện tại khóa ghi vài mili-giây. Khi bảng lớn, tạo index kiểu này
-- phải tách ra migration riêng, tắt transaction của goose (annotation
-- NO TRANSACTION) rồi dùng CONCURRENTLY.
--
-- ⚠️ goose coi MỌI dòng comment có chứa chữ "+" liền "goose" là chỉ thị — kể
-- cả khi chuỗi đó nằm giữa một câu giải thích — và từ chối cả file ("invalid
-- annotation"). Đã gặp hai lần khi viết chính file này. Muốn nhắc tới
-- annotation thì gọi tên nó (NO TRANSACTION), đừng chép nguyên cú pháp.

-- +goose Up
CREATE INDEX products_live_id_idx ON products (id) INCLUDE (slug, updated_at)
    WHERE deleted_at IS NULL AND status = 'live';

-- +goose Down
DROP INDEX IF EXISTS products_live_id_idx;
