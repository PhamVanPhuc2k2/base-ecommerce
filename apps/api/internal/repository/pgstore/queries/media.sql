-- name: InsertMedia :exec
INSERT INTO media (id, object_key, content_type, size_bytes, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: MediaByID :one
-- FOR UPDATE: hai lệnh "hoàn tất" đồng thời cho cùng một file xếp hàng thay vì
-- cùng đọc 'pending' rồi cùng ghi.
SELECT id, object_key, content_type, size_bytes, status, created_at, updated_at
FROM media
WHERE id = $1
FOR UPDATE;

-- name: UpdateMedia :exec
UPDATE media SET content_type = $2, size_bytes = $3, status = $4, updated_at = $5
WHERE id = $1;

-- name: ReadyMediaKeys :many
-- MỘT câu cho cả danh sách ảnh của sản phẩm, trả những key ĐÃ ready.
SELECT object_key FROM media
WHERE object_key = ANY(sqlc.arg(keys)::text[]) AND status = 'ready';
