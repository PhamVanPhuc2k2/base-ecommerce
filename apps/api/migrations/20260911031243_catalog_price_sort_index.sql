-- +goose Up
-- Sắp xếp theo giá trên trang danh sách không lọc danh mục hiện không có index
-- nào phục vụ: products_category_live_idx đòi có điều kiện bằng trên
-- category_id. Đo ở 200k dòng: 20,5 ms và 5147 buffer cho Parallel Seq Scan
-- kèm top-N heapsort, so với 0,049 ms và 4 buffer của nhánh created_at.
--
-- Một index (price, id) phục vụ được cả ASC lẫn DESC vì Postgres quét ngược
-- được trên B-tree.
CREATE INDEX products_live_price_idx ON products (price, id)
    WHERE deleted_at IS NULL AND status = 'live';

-- +goose Down
DROP INDEX IF EXISTS products_live_price_idx;
