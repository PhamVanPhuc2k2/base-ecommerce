-- P3.2 — giữ chỗ tồn kho.

-- +goose Up
CREATE TABLE reservations (
    id         UUID        PRIMARY KEY,
    -- Mã đơn (P4). UNIQUE = khóa idempotency: giữ lại cùng ref không giữ hai lần.
    ref        TEXT        NOT NULL,
    status     TEXT        NOT NULL,
    -- NULL = không hết hạn (đơn COD, giữ tới khi xử lý).
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT reservations_ref_key UNIQUE (ref),
    CONSTRAINT reservations_status_check CHECK (status IN ('active', 'committed', 'released', 'expired'))
);

-- Worker quét giữ chỗ hết hạn: chỉ đụng dòng active có hạn.
CREATE INDEX reservations_expiry_idx ON reservations (expires_at) WHERE status = 'active' AND expires_at IS NOT NULL;

CREATE TABLE reservation_lines (
    reservation_id UUID NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    variant_id     UUID NOT NULL REFERENCES product_variants(id),
    location_id    UUID NOT NULL REFERENCES locations(id),
    quantity       INT  NOT NULL,
    PRIMARY KEY (reservation_id, variant_id, location_id),
    CONSTRAINT reservation_lines_qty_check CHECK (quantity > 0)
);

-- +goose Down
DROP TABLE reservation_lines;
DROP TABLE reservations;
