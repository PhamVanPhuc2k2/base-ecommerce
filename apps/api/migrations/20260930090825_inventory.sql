-- P3.1 — kho, tồn kho, sổ cái biến động.

-- +goose Up
CREATE TABLE locations (
    id           UUID        PRIMARY KEY,
    -- Không đổi được sau khi tạo: sổ cái và hệ thống ngoài tham chiếu mã này.
    code         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    kind         TEXT        NOT NULL,
    address      TEXT,
    sells_online BOOLEAN     NOT NULL,
    -- Số nhỏ = phân bổ trước (P3.2).
    priority     INT         NOT NULL,
    active       BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT locations_code_key UNIQUE (code),
    CONSTRAINT locations_code_format CHECK (code ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(code) <= 32),
    CONSTRAINT locations_kind_check CHECK (kind IN ('warehouse', 'showroom')),
    -- Showroom hiện cho khách trên storefront (P3.4) nên phải có địa chỉ.
    CONSTRAINT locations_showroom_address CHECK (kind <> 'showroom' OR address IS NOT NULL)
);

-- available = on_hand - reserved: TÍNH, không lưu — lưu cả ba thì sớm muộn lệch.
CREATE TABLE stock_levels (
    location_id UUID        NOT NULL REFERENCES locations(id),
    variant_id  UUID        NOT NULL REFERENCES product_variants(id),
    on_hand     INT         NOT NULL,
    reserved    INT         NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (location_id, variant_id),
    -- Lớp chặn CUỐI nếu code sai: không bao giờ âm, không giữ nhiều hơn có.
    CONSTRAINT stock_levels_on_hand_check CHECK (on_hand >= 0),
    CONSTRAINT stock_levels_reserved_check CHECK (reserved >= 0 AND reserved <= on_hand)
);

-- Tra tồn theo phiên bản (trang sản phẩm, phân bổ P3.2) — khóa chính bắt đầu
-- bằng location_id nên không dùng được cho truy vấn này.
CREATE INDEX stock_levels_variant_idx ON stock_levels (variant_id);

-- Sổ cái CHỈ THÊM. Không có UPDATE/DELETE nào trong code chạm bảng này.
CREATE TABLE stock_movements (
    id              UUID        PRIMARY KEY,
    location_id     UUID        NOT NULL REFERENCES locations(id),
    variant_id      UUID        NOT NULL REFERENCES product_variants(id),
    kind            TEXT        NOT NULL,
    on_hand_delta   INT         NOT NULL,
    reserved_delta  INT         NOT NULL,
    on_hand_after   INT         NOT NULL,
    reserved_after  INT         NOT NULL,
    reason          TEXT,
    ref             TEXT,
    actor_id        UUID        REFERENCES users(id) ON DELETE SET NULL,
    -- Chống bấm hai lần (đặc tả P3.1 mục 2.5): UNIQUE trong CÙNG transaction
    -- với thay đổi tồn — thứ Redis không làm được.
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL,
    CONSTRAINT stock_movements_kind_check CHECK (kind IN ('receipt', 'adjustment', 'count', 'reserve', 'release', 'commit')),
    CONSTRAINT stock_movements_idem_key UNIQUE (idempotency_key)
);

CREATE INDEX stock_movements_variant_idx ON stock_movements (variant_id, id DESC);
CREATE INDEX stock_movements_location_idx ON stock_movements (location_id, id DESC);

-- +goose Down
DROP TABLE stock_movements;
DROP TABLE stock_levels;
DROP TABLE locations;
