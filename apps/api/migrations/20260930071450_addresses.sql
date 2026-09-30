-- P2.4 — sổ địa chỉ giao hàng.

-- +goose Up
CREATE TABLE addresses (
    id             UUID        PRIMARY KEY,
    user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_name TEXT        NOT NULL,
    -- Đã chuẩn hóa về 0xxxxxxxxx ở domain; CHECK chặn SQL tay ghi dạng khác.
    phone          TEXT        NOT NULL,
    province       TEXT        NOT NULL,
    ward           TEXT        NOT NULL,
    street         TEXT        NOT NULL,
    is_default     BOOLEAN     NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT addresses_phone_check CHECK (phone ~ '^0[35789][0-9]{8}$')
);

CREATE INDEX addresses_user_idx ON addresses (user_id, created_at DESC);

-- Nhiều nhất MỘT địa chỉ mặc định cho mỗi người — index một phần chỉ chứa
-- các dòng is_default. Đổi mặc định phải bỏ cờ cũ TRƯỚC rồi mới đặt cờ mới,
-- trong cùng transaction.
CREATE UNIQUE INDEX addresses_one_default ON addresses (user_id) WHERE is_default;

-- +goose Down
DROP TABLE addresses;
