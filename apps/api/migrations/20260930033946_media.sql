-- P1.4 — media: file đã (hoặc đang) upload lên object storage.
--
-- products.images vẫn là TEXT[] và giờ chứa object_key của media ở đây
-- ("products/0190….jpg"), không chứa URL. Không có khóa ngoại từ mảng sang
-- bảng này (Postgres không có FK cho phần tử mảng) — use case kiểm mọi key là
-- media 'ready' lúc ghi sản phẩm. Đặc tả P1.4 mục 3.3.

-- +goose Up
CREATE TABLE media (
    id           UUID        PRIMARY KEY,
    object_key   TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL,
    -- pending: đã cấp URL upload, chưa xác nhận file. ready: đã dò định dạng
    -- thật và dùng được cho sản phẩm.
    status       TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT media_object_key_key UNIQUE (object_key),
    CONSTRAINT media_status_check CHECK (status IN ('pending', 'ready')),
    CONSTRAINT media_content_type_check CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    CONSTRAINT media_size_check CHECK (size_bytes > 0)
);

-- +goose Down
DROP TABLE IF EXISTS media;
