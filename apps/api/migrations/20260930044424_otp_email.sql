-- P2.3 — mã OTP qua email và hàng đợi thư.

-- +goose Up
CREATE TABLE otp_codes (
    id          UUID        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     TEXT        NOT NULL,
    -- HMAC-SHA256 của mã, KHÔNG phải mã: 6 chữ số băm trần thì vét cạn được
    -- trong một giây (đặc tả P2.3 mục 2.2).
    code_hash   BYTEA       NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    attempts    INT         NOT NULL DEFAULT 0,
    -- Dùng xong, hoặc bị mã mới hơn thay — cả hai đều là "mã đã chết".
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT otp_codes_purpose_check CHECK (purpose IN ('verify_email', 'reset_password')),
    CONSTRAINT otp_codes_hash_len CHECK (octet_length(code_hash) = 32),
    CONSTRAINT otp_codes_attempts_check CHECK (attempts >= 0)
);

-- Mọi truy vấn đều là "mã mới nhất của người này cho mục đích này".
CREATE INDEX otp_codes_user_purpose_idx ON otp_codes (user_id, purpose, created_at DESC);

CREATE TABLE outbound_emails (
    id         UUID        PRIMARY KEY,
    to_address TEXT        NOT NULL,
    subject    TEXT        NOT NULL,
    -- Chứa mã OTP cho tới lúc gửi xong; worker xóa về chuỗi rỗng cùng lúc
    -- đánh dấu sent_at (đặc tả P2.3 mục 2.1).
    body_text  TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    sent_at    TIMESTAMPTZ
);

-- +goose Down
DROP TABLE outbound_emails;
DROP TABLE otp_codes;
