-- name: AllCategories :many
SELECT id, parent_id, slug, name, position, created_at, updated_at
FROM categories
ORDER BY position, name;
