-- name: UpsertVariant :exec
INSERT INTO product_variants (
    id, product_id, sku, price, currency, options, status, position, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
-- sku và product_id KHÔNG nằm trong DO UPDATE SET: SKU bất biến (định danh
-- trong kho và hóa đơn), và variant không bao giờ đổi chủ sang sản phẩm khác.
-- Cột nào sửa được trong domain mà vắng ở đây thì Save trả nil và thay đổi
-- biến mất — thêm trường sửa được thì thêm vào đây cùng lúc.
ON CONFLICT (id) DO UPDATE SET
    price      = EXCLUDED.price,
    currency   = EXCLUDED.currency,
    options    = EXCLUDED.options,
    status     = EXCLUDED.status,
    position   = EXCLUDED.position,
    updated_at = EXCLUDED.updated_at;

-- name: VariantsByProduct :many
SELECT id, product_id, sku, price, currency, options, status, position, created_at, updated_at
FROM product_variants
WHERE product_id = $1
ORDER BY position, id;

-- name: VariantsByProducts :many
-- MỘT câu cho cả trang danh sách, không phải một câu mỗi sản phẩm (N+1).
-- ANY với một tham số mảng: số sản phẩm trên trang đổi thì câu SQL vẫn giữ
-- nguyên, dùng lại được plan đã chuẩn bị.
SELECT id, product_id, sku, price, currency, options, status, position, created_at, updated_at
FROM product_variants
WHERE product_id = ANY(sqlc.arg(product_ids)::uuid[])
ORDER BY product_id, position, id;
