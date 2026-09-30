-- name: InsertUser :exec
INSERT INTO users (id, email, password_hash, full_name, status, email_verified_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UserByEmail :one
SELECT id, email, password_hash, full_name, status, email_verified_at, created_at, updated_at
FROM users WHERE email = $1;

-- name: UserByID :one
SELECT id, email, password_hash, full_name, status, email_verified_at, created_at, updated_at
FROM users WHERE id = $1;

-- name: InsertRefreshToken :exec
INSERT INTO refresh_tokens (id, family_id, user_id, token_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: RefreshTokenByHash :one
-- FOR UPDATE: hai request refresh cùng một token phải xếp hàng. Không khóa thì
-- cả hai cùng đọc used_at = NULL, cùng đánh dấu, cùng sinh token mới — phép
-- phát hiện dùng lại (đặc tả P2.1 mục 2.4) bị qua mặt đúng bằng một cuộc đua.
SELECT id, family_id, user_id, token_hash, expires_at, used_at, revoked_at, created_at
FROM refresh_tokens WHERE token_hash = $1
FOR UPDATE;

-- name: MarkRefreshTokenUsed :exec
UPDATE refresh_tokens SET used_at = $2 WHERE id = $1;

-- name: RevokeRefreshFamily :exec
UPDATE refresh_tokens SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL;
