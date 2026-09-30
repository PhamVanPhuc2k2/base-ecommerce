-- name: LatestOTP :one
-- FOR UPDATE: đếm lần sai là đọc-sửa-ghi; hai lần thử song song không khóa thì
-- cùng đọc attempts = 4, cùng ghi 5 — kẻ dò được thêm lượt miễn phí.
SELECT id, user_id, purpose, code_hash, expires_at, attempts, consumed_at, created_at
FROM otp_codes
WHERE user_id = $1 AND purpose = $2
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: KillActiveOTPs :exec
UPDATE otp_codes SET consumed_at = $3
WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL;

-- name: InsertOTP :exec
INSERT INTO otp_codes (id, user_id, purpose, code_hash, expires_at, attempts, created_at)
VALUES ($1, $2, $3, $4, $5, 0, $6);

-- name: RecordOTPFailure :exec
UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1;

-- name: ConsumeOTP :exec
UPDATE otp_codes SET consumed_at = $2 WHERE id = $1;

-- name: InsertOutboundEmail :exec
INSERT INTO outbound_emails (id, to_address, subject, body_text, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: OutboundEmailForUpdate :one
SELECT id, to_address, subject, body_text, created_at, sent_at
FROM outbound_emails WHERE id = $1
FOR UPDATE;

-- name: MarkOutboundEmailSent :exec
UPDATE outbound_emails SET sent_at = $2, body_text = '' WHERE id = $1;
