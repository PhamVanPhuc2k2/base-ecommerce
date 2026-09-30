-- name: UpsertProduct :exec
INSERT INTO products (
    id, slug, name, short_description, category_id, brand_id,
    price, currency, status, attributes, images, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
-- SKU không còn ở đây từ P1.2 — nó thuộc product_variants (variant.sql).
-- price là giá "từ", domain tính lại sau mọi thay đổi variant.
--
-- ⚠️ Cột nào vắng mặt ở đây thì Save trả nil mà thay đổi BIẾN MẤT. category_id
-- và brand_id từng vắng mặt tới P1.1 — thêm trường sửa được vào Product.Update
-- thì phải thêm cột vào đây trong cùng thay đổi.
ON CONFLICT (id) DO UPDATE SET
    slug              = EXCLUDED.slug,
    category_id       = EXCLUDED.category_id,
    brand_id          = EXCLUDED.brand_id,
    name              = EXCLUDED.name,
    short_description = EXCLUDED.short_description,
    price             = EXCLUDED.price,
    currency          = EXCLUDED.currency,
    status            = EXCLUDED.status,
    attributes        = EXCLUDED.attributes,
    images            = EXCLUDED.images,
    updated_at        = EXCLUDED.updated_at;

-- name: ProductByID :one
-- FOR UPDATE: ByID chỉ được dùng trong các use case GHI (Update, Publish), và
-- cả hai đều đọc rồi ghi đè nguyên dòng. Không khóa thì hai request đồng thời
-- sẽ ghi đè lẫn nhau — Publish commit 'live' xong Update ghi đè lại 'draft',
-- sản phẩm bị gỡ bán mà không có lỗi nào.
SELECT sqlc.embed(products)
FROM products
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE;

-- name: ProductBySlug :one
SELECT sqlc.embed(products)
FROM products
WHERE slug = $1 AND deleted_at IS NULL AND status = 'live';
