-- name: AllLocations :many
SELECT id, code, name, kind, address, sells_online, priority, active, created_at, updated_at
FROM locations ORDER BY priority, code;

-- name: LocationByID :one
SELECT id, code, name, kind, address, sells_online, priority, active, created_at, updated_at
FROM locations WHERE id = $1;

-- name: InsertLocation :exec
INSERT INTO locations (id, code, name, kind, address, sells_online, priority, active, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: UpdateLocation :exec
UPDATE locations SET name = $2, kind = $3, address = $4, sells_online = $5, priority = $6, active = $7, updated_at = $8
WHERE id = $1;

-- name: VariantExists :one
SELECT EXISTS (SELECT 1 FROM product_variants WHERE id = $1);

-- name: EnsureStockLevel :exec
-- Bảo đảm có dòng để khóa — chưa có dòng thì FOR UPDATE không khóa được gì,
-- và hai lần nhập đầu tiên song song sẽ cùng INSERT.
INSERT INTO stock_levels (location_id, variant_id, on_hand, reserved, updated_at)
VALUES ($1, $2, 0, 0, $3)
ON CONFLICT (location_id, variant_id) DO NOTHING;

-- name: StockLevelForUpdate :one
SELECT location_id, variant_id, on_hand, reserved, updated_at
FROM stock_levels WHERE location_id = $1 AND variant_id = $2
FOR UPDATE;

-- name: UpdateStockLevel :exec
UPDATE stock_levels SET on_hand = $3, reserved = $4, updated_at = $5
WHERE location_id = $1 AND variant_id = $2;

-- name: InsertStockMovement :exec
INSERT INTO stock_movements (id, location_id, variant_id, kind, on_hand_delta, reserved_delta, on_hand_after,
    reserved_after, reason, ref, actor_id, idempotency_key, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: MovementByIdempotencyKey :one
SELECT id, location_id, variant_id, kind, on_hand_delta, reserved_delta, on_hand_after, reserved_after,
    reason, ref, actor_id, idempotency_key, created_at
FROM stock_movements WHERE idempotency_key = $1;

-- name: VariantStockByLocation :many
-- LEFT JOIN từ locations: kho chưa từng có hàng này vẫn hiện, với tồn 0.
SELECT l.id, l.code, l.name, l.kind, l.address, l.sells_online, l.priority, l.active,
    COALESCE(s.on_hand, 0)::int AS on_hand, COALESCE(s.reserved, 0)::int AS reserved
FROM locations l
LEFT JOIN stock_levels s ON s.location_id = l.id AND s.variant_id = $1
ORDER BY l.priority, l.code;

-- name: ListStockMovements :many
-- id là UUIDv7 (tăng theo thời gian) nên "id < con trỏ" là phân trang theo
-- thời gian, dùng được index (variant_id, id DESC).
SELECT id, location_id, variant_id, kind, on_hand_delta, reserved_delta, on_hand_after, reserved_after,
    reason, ref, actor_id, idempotency_key, created_at
FROM stock_movements
WHERE (sqlc.narg(variant_id)::uuid IS NULL OR variant_id = sqlc.narg(variant_id))
  AND (sqlc.narg(location_id)::uuid IS NULL OR location_id = sqlc.narg(location_id))
  AND (sqlc.narg(before)::uuid IS NULL OR id < sqlc.narg(before))
ORDER BY id DESC
LIMIT sqlc.arg(lim);
