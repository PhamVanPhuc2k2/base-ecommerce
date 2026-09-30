-- name: AllAttributeDefinitions :many
SELECT id, code, name, type, unit, options, filterable, variant, created_at, updated_at
FROM attribute_definitions
ORDER BY code;

-- name: AttributeByID :one
-- FOR UPDATE: chỉ dùng trong use case sửa (đọc rồi ghi đè cả dòng).
SELECT id, code, name, type, unit, options, filterable, variant, created_at, updated_at
FROM attribute_definitions
WHERE id = $1
FOR UPDATE;

-- name: InsertAttribute :exec
INSERT INTO attribute_definitions
    (id, code, name, type, unit, options, filterable, variant, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: UpdateAttribute :execrows
-- code, type, variant KHÔNG có ở đây: bất biến (đặc tả P1.3 mục 2.5).
UPDATE attribute_definitions
SET name = $2, unit = $3, options = $4, filterable = $5, updated_at = $6
WHERE id = $1;

-- name: DeleteAttribute :execrows
DELETE FROM attribute_definitions WHERE id = $1;

-- name: AllCategoryAttributes :many
SELECT category_id, attribute_id, required, position
FROM category_attributes
ORDER BY category_id, position, attribute_id;

-- name: DeleteCategoryAttributes :exec
DELETE FROM category_attributes WHERE category_id = $1;

-- name: InsertCategoryAttribute :exec
INSERT INTO category_attributes (category_id, attribute_id, required, position)
VALUES ($1, $2, $3, $4);
