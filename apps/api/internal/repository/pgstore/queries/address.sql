-- name: AddressesOf :many
SELECT id, user_id, recipient_name, phone, province, ward, street, is_default, created_at, updated_at
FROM addresses WHERE user_id = $1
ORDER BY is_default DESC, created_at DESC;

-- name: AddressForUpdate :one
-- Lọc theo CẢ user_id: địa chỉ của người khác "không tồn tại" với người gọi.
SELECT id, user_id, recipient_name, phone, province, ward, street, is_default, created_at, updated_at
FROM addresses WHERE id = $1 AND user_id = $2
FOR UPDATE;

-- name: CountAddresses :one
SELECT count(*) FROM addresses WHERE user_id = $1;

-- name: InsertAddress :exec
INSERT INTO addresses (id, user_id, recipient_name, phone, province, ward, street, is_default, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: UpdateAddress :exec
UPDATE addresses SET recipient_name = $2, phone = $3, province = $4, ward = $5, street = $6, updated_at = $7
WHERE id = $1;

-- name: DeleteAddress :exec
DELETE FROM addresses WHERE id = $1;

-- name: ClearDefaultAddress :exec
UPDATE addresses SET is_default = false WHERE user_id = $1 AND is_default;

-- name: SetDefaultAddress :exec
UPDATE addresses SET is_default = true WHERE id = $1;

-- name: NewestAddressID :one
SELECT id FROM addresses WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1;

-- name: UpdateUserFullName :exec
UPDATE users SET full_name = $2, updated_at = $3 WHERE id = $1;
