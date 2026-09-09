-- +goose Up
-- pg_trgm: tìm kiếm gần đúng tên sản phẩm (dùng từ P1)
CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- btree_gin: index kết hợp cột thường với cột JSONB trong bộ lọc catalog
CREATE EXTENSION IF NOT EXISTS btree_gin;

-- +goose Down
DROP EXTENSION IF EXISTS btree_gin;
DROP EXTENSION IF EXISTS pg_trgm;
