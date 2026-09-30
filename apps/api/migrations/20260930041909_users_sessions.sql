-- P2.1 — tài khoản và phiên đăng nhập.

-- +goose Up
CREATE TABLE users (
    id                UUID        PRIMARY KEY,
    -- Email đã CHUẨN HÓA (cắt khoảng trắng, chữ thường) ở domain trước khi
    -- ghi. Không dùng citext: thêm một extension chỉ để so sánh không phân
    -- biệt hoa thường, trong khi domain vốn phải chuẩn hóa để còn so sánh
    -- trong Go. Ràng buộc CHECK dưới chặn SQL tay ghi email chưa chuẩn hóa —
    -- không có nó thì "A@x.vn" và "a@x.vn" thành hai tài khoản.
    email             TEXT        NOT NULL,
    -- Chuỗi PHC của argon2id: tham số nằm trong chuỗi, nên tăng tham số sau
    -- này không vô hiệu mật khẩu cũ. KHÔNG BAO GIỜ là mật khẩu thô.
    password_hash     TEXT        NOT NULL,
    full_name         TEXT        NOT NULL,
    status            TEXT        NOT NULL,
    email_verified_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL,
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_email_normalized CHECK (email = lower(btrim(email))),
    CONSTRAINT users_status_check CHECK (status IN ('active', 'disabled'))
);

CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY,
    -- Mọi token sinh ra từ MỘT lần đăng nhập chung family_id. Phát hiện một
    -- token đã dùng bị đưa lên lại → thu hồi CẢ family (đặc tả P2.1 mục 2.4).
    family_id  UUID        NOT NULL,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- SHA-256 của token, KHÔNG phải token: lộ bảng này không lộ token dùng được.
    token_hash BYTEA       NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash)
);

-- Thu hồi cả family (phát hiện dùng lại, đăng xuất) quét theo family_id.
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
-- Dọn token hết hạn theo lô (job dọn — sau).
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens (expires_at);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
