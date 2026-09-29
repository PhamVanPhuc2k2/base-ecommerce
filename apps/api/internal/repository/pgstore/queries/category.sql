-- name: AllCategories :many
SELECT id, parent_id, slug, name, position, created_at, updated_at
FROM categories
ORDER BY position, name;

-- name: CategoryByID :one
SELECT id, parent_id, slug, name, position, created_at, updated_at
FROM categories
WHERE id = $1;

-- name: InsertCategory :exec
INSERT INTO categories (id, parent_id, slug, name, position, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: UpdateCategory :execrows
UPDATE categories
SET parent_id = $2, slug = $3, name = $4, position = $5, updated_at = $6
WHERE id = $1;

-- name: DeleteCategory :execrows
DELETE FROM categories WHERE id = $1;
