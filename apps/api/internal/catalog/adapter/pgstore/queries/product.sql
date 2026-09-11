-- name: UpsertProduct :exec
INSERT INTO products (
    id, sku, slug, name, short_description, category_id, brand_id,
    price, currency, status, attributes, images, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
ON CONFLICT (id) DO UPDATE SET
    slug              = EXCLUDED.slug,
    name              = EXCLUDED.name,
    short_description = EXCLUDED.short_description,
    price             = EXCLUDED.price,
    currency          = EXCLUDED.currency,
    status            = EXCLUDED.status,
    attributes        = EXCLUDED.attributes,
    images            = EXCLUDED.images,
    updated_at        = EXCLUDED.updated_at;

-- name: ProductByID :one
SELECT sqlc.embed(products)
FROM products
WHERE id = $1 AND deleted_at IS NULL;

-- name: ProductBySlug :one
SELECT sqlc.embed(products)
FROM products
WHERE slug = $1 AND deleted_at IS NULL;
