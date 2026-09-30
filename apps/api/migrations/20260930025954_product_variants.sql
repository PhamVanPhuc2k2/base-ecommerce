-- P1.2 — EXPAND: thêm product_variants, chuyển sku/giá sang variant.
--
-- Migration này CHỈ THÊM. products.sku vẫn nằm nguyên cho tới migration
-- contract (drop_products_sku) — code cũ đang chạy vẫn đọc được nó trong lúc
-- code mới chưa lên. Xem docs/design/05-deployment.md, mục expand/contract.

-- +goose Up
CREATE TABLE product_variants (
    id         UUID          PRIMARY KEY,
    -- RESTRICT: đơn hàng (P4) sẽ trỏ vào variant, nên không bao giờ xóa lan từ
    -- sản phẩm xuống. Ngừng bán là status = 'inactive', không phải DELETE.
    product_id UUID          NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    sku        TEXT          NOT NULL,
    price      NUMERIC(15,2) NOT NULL CHECK (price >= 0),
    currency   TEXT          NOT NULL DEFAULT 'VND',
    -- Thuộc tính PHÂN BIỆT các phiên bản của cùng một sản phẩm:
    -- {"ram": "16GB", "màu": "Đen"}. Map tự do ở P1.2; P1.3 gắn định nghĩa.
    options    JSONB         NOT NULL DEFAULT '{}',
    status     TEXT          NOT NULL,
    position   INT           NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ   NOT NULL,
    updated_at TIMESTAMPTZ   NOT NULL,
    CONSTRAINT product_variants_currency_vnd CHECK (currency = 'VND'),
    CONSTRAINT product_variants_status_check CHECK (status IN ('active', 'inactive')),
    CONSTRAINT product_variants_options_object CHECK (jsonb_typeof(options) = 'object')
);

-- UNIQUE toàn bảng, KHÔNG partial như products_sku_uq cũ: variant không bao giờ
-- bị xóa (kể cả xóa mềm), nên không có "SKU của bản đã xóa" nào cần nhường chỗ.
CREATE UNIQUE INDEX product_variants_sku_uq ON product_variants (sku);

-- Nạp variant theo sản phẩm (trang chi tiết) và theo cả trang (= ANY($1)),
-- trả về đúng thứ tự hiển thị mà không cần sắp lại.
CREATE INDEX product_variants_product_idx ON product_variants (product_id, position, id);

-- Backfill: mỗi sản phẩm chưa xóa thành MỘT variant không có options.
--
-- ⚠️ id của variant = id của sản phẩm. Postgres 17 không có uuidv7() (bản 18
-- mới có), còn gen_random_uuid() là v4 — không sắp theo thời gian, phá quy ước
-- "mọi ID là UUIDv7" của thiết kế 02. ID sản phẩm đã là v7 và mỗi sản phẩm chỉ
-- sinh đúng một variant backfill, nên không thể trùng.
--
-- Sản phẩm đã xóa mềm KHÔNG được backfill: chúng không bao giờ được đọc lại,
-- và SKU của chúng có thể đã được một sản phẩm sống dùng lại (products_sku_uq
-- là partial) — chép sang sẽ đụng product_variants_sku_uq.
INSERT INTO product_variants
    (id, product_id, sku, price, currency, options, status, position, created_at, updated_at)
SELECT id, id, sku, price, currency, '{}', 'active', 0, created_at, updated_at
FROM products
WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS product_variants;
