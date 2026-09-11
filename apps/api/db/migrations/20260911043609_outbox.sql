-- +goose Up
CREATE TABLE outbox (
    id             UUID        PRIMARY KEY,
    aggregate_type TEXT        NOT NULL,
    aggregate_id   UUID        NOT NULL,
    event_type     TEXT        NOT NULL,
    payload        JSONB       NOT NULL,
    trace_id       TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at   TIMESTAMPTZ,
    attempts       INT         NOT NULL DEFAULT 0,
    last_error     TEXT,
    CONSTRAINT outbox_aggregate_type_not_blank CHECK (btrim(aggregate_type) <> ''),
    CONSTRAINT outbox_event_type_not_blank     CHECK (btrim(event_type) <> '')
);

-- Index PARTIAL: bảng tích lũy hàng triệu dòng nhưng phần chưa gửi luôn nhỏ.
-- Index đầy đủ trên cột này sẽ to bằng cả bảng mà 99,99% nội dung không bao giờ
-- được đọc tới.
CREATE INDEX outbox_unpublished_idx ON outbox (id) WHERE published_at IS NULL;

-- Cho job dọn: tìm dòng đã gửi quá hạn.
CREATE INDEX outbox_published_at_idx ON outbox (published_at) WHERE published_at IS NOT NULL;

CREATE TABLE processed_events (
    consumer     TEXT        NOT NULL,
    event_id     UUID        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- consumer PHẢI là một phần của khóa chính. Thiếu nó thì consumer thứ hai
    -- thêm vào ở P7 sẽ thấy mọi event đều "đã xử lý" và không làm gì cả.
    PRIMARY KEY (consumer, event_id)
);

CREATE INDEX processed_events_processed_at_idx ON processed_events (processed_at);

-- +goose Down
DROP TABLE processed_events;
DROP TABLE outbox;
