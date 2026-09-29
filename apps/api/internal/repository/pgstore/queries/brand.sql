-- name: AllBrands :many
SELECT id, slug, name, created_at, updated_at
FROM brands
ORDER BY name, id;

-- name: BrandByID :one
-- FOR UPDATE: chỉ dùng trong use case sửa, đọc-rồi-ghi-đè cả dòng. Không khóa
-- thì hai lệnh PATCH đồng thời (một đổi tên, một đổi slug) ghi đè lẫn nhau.
SELECT id, slug, name, created_at, updated_at
FROM brands
WHERE id = $1
FOR UPDATE;

-- name: InsertBrand :exec
INSERT INTO brands (id, slug, name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateBrand :execrows
UPDATE brands SET slug = $2, name = $3, updated_at = $4 WHERE id = $1;

-- name: DeleteBrand :execrows
DELETE FROM brands WHERE id = $1;
