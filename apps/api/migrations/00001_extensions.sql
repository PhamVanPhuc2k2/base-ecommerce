-- +goose Up
-- pg_trgm: tìm kiếm gần đúng tên sản phẩm (dùng từ P1)
CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- btree_gin: index kết hợp cột thường với cột JSONB trong bộ lọc catalog
CREATE EXTENSION IF NOT EXISTS btree_gin;

-- +goose Down
-- Phần Down này chỉ dùng khi phát triển và SẼ NGỪNG CHẠY ĐƯỢC từ P1, khi có
-- index trigram phụ thuộc vào pg_trgm (Postgres báo SQLSTATE 2BP01).
-- Đó là hành vi ĐÚNG, không phải lỗi. TUYỆT ĐỐI KHÔNG thêm CASCADE để "sửa" —
-- CASCADE sẽ xóa im lặng mọi index đang phụ thuộc.
-- Production không bao giờ migrate down; rollback bằng cách deploy lại image cũ
-- (xem docs/design/05-deployment.md mục 1 và 5).
DROP EXTENSION IF EXISTS btree_gin;
DROP EXTENSION IF EXISTS pg_trgm;
