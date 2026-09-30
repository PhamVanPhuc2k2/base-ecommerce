-- P1.2 — CONTRACT: bỏ products.sku, SKU giờ chỉ nằm ở product_variants.
--
-- ⚠️ CHỈ chạy SAU khi code đang chạy đã thôi đọc cột này. Thứ tự đúng khi
-- deploy: (1) migration expand, (2) code ghi cả hai nơi, (3) migration này,
-- (4) code thôi ghi products.sku. Chạy migration này khi code cũ còn sống thì
-- mọi lệnh INSERT của nó hỏng vì cột không còn. Xem docs/design/05-deployment.md.
--
-- products.price KHÔNG bị bỏ: nó ở lại với nghĩa mới là giá "từ" (giá thấp
-- nhất của variant active), vì index lọc/sắp theo giá của trang danh sách dựa
-- vào nó. Đặc tả P1.2 mục 2.1.

-- +goose Up
-- Xóa cột thì Postgres tự bỏ luôn products_sku_uq (index trên chính cột đó).
ALTER TABLE products DROP COLUMN sku;

-- +goose Down
-- Dựng lại cột từ variant đầu tiên của mỗi sản phẩm (theo position rồi id —
-- cùng thứ tự API trả ra), rồi mới đặt NOT NULL và index.
--
-- Sản phẩm không có variant nào (chỉ có thể là sản phẩm đã xóa mềm trước P1.2,
-- vốn không được backfill) nhận SKU giả dựng từ id: cột phải NOT NULL, và SKU
-- thật của chúng đã không còn ở đâu để lấy lại. Chấp nhận — đây là đường lùi
-- khẩn cấp, không phải đường chạy thường ngày.
ALTER TABLE products ADD COLUMN sku TEXT;
UPDATE products p
SET sku = COALESCE(
    (SELECT v.sku FROM product_variants v
     WHERE v.product_id = p.id
     ORDER BY v.position, v.id
     LIMIT 1),
    'khoi-phuc-' || p.id::text
);
ALTER TABLE products ALTER COLUMN sku SET NOT NULL;
CREATE UNIQUE INDEX products_sku_uq ON products (sku) WHERE deleted_at IS NULL;
