-- name: LockCandidateStock :many
-- MỘT câu khóa mọi dòng tồn của mọi phiên bản trong yêu cầu, theo thứ tự cố
-- định (variant_id, location_id): hai đơn chứa cùng hai món nhưng liệt kê
-- ngược thứ tự vẫn khóa cùng thứ tự — không thể deadlock (đặc tả P3.2 mục 2.3).
-- Chỉ kho đang hoạt động VÀ bán online là ứng viên.
SELECT s.location_id, s.variant_id, s.on_hand, s.reserved, s.updated_at, l.priority, l.code
FROM stock_levels s
JOIN locations l ON l.id = s.location_id
WHERE s.variant_id = ANY(sqlc.arg(variant_ids)::uuid[]) AND l.active AND l.sells_online
ORDER BY s.variant_id, s.location_id
FOR UPDATE OF s;

-- name: LockReservationStock :many
-- Khóa đúng các dòng tồn của một giữ chỗ (nhả / xuất kho), CÙNG thứ tự với
-- LockCandidateStock.
SELECT s.location_id, s.variant_id, s.on_hand, s.reserved, s.updated_at
FROM stock_levels s
JOIN reservation_lines rl ON rl.location_id = s.location_id AND rl.variant_id = s.variant_id
WHERE rl.reservation_id = $1
ORDER BY s.variant_id, s.location_id
FOR UPDATE OF s;

-- name: InsertReservation :exec
INSERT INTO reservations (id, ref, status, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertReservationLine :exec
INSERT INTO reservation_lines (reservation_id, variant_id, location_id, quantity)
VALUES ($1, $2, $3, $4);

-- name: ReservationByID :one
SELECT id, ref, status, expires_at, created_at, updated_at FROM reservations WHERE id = $1;

-- name: ReservationByIDForUpdate :one
SELECT id, ref, status, expires_at, created_at, updated_at FROM reservations WHERE id = $1 FOR UPDATE;

-- name: ReservationByRef :one
SELECT id, ref, status, expires_at, created_at, updated_at FROM reservations WHERE ref = $1;

-- name: ReservationLinesOf :many
SELECT reservation_id, variant_id, location_id, quantity
FROM reservation_lines WHERE reservation_id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY variant_id, location_id;

-- name: UpdateReservationStatus :exec
UPDATE reservations SET status = $2, updated_at = $3 WHERE id = $1;

-- name: DueReservations :many
-- SKIP LOCKED: nhiều bản worker không nhả trùng, và không đứng chờ giữ chỗ
-- đang được xuất kho ở request khác.
SELECT id, ref, status, expires_at, created_at, updated_at
FROM reservations
WHERE status = 'active' AND expires_at IS NOT NULL AND expires_at < sqlc.arg(now)
ORDER BY expires_at
LIMIT sqlc.arg(lim)
FOR UPDATE SKIP LOCKED;
