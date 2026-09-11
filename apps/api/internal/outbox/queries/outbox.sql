-- name: AppendOutbox :exec
INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, trace_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- FOR UPDATE SKIP LOCKED: relay khóa đúng những dòng nó đang xử lý, dòng đã bị
-- khóa thì bỏ qua thay vì xếp hàng chờ — nên nó không chặn ai khác, kể cả
-- transaction nghiệp vụ đang ghi vào outbox.
--
-- Nhưng đó cũng chính là lý do CHỈ ĐƯỢC CHẠY MỘT BẢN RELAY. Có bản thứ hai, nó
-- sẽ "nhảy cóc" qua những dòng bản thứ nhất đang giữ và publish event có id lớn
-- hơn ra RabbitMQ trước — sai thứ tự event, mà không có lỗi nào báo.
--
-- Comment này đặt ngay trên FetchUnpublished chứ không ở đầu file, vì sqlc gắn
-- mọi comment đứng trước câu truy vấn ĐẦU TIÊN thành godoc của hàm đó: để ở đầu
-- file thì lời cảnh báo về relay sẽ mọc trên AppendOutbox, nơi nó vô nghĩa.
-- name: FetchUnpublished :many
SELECT id, aggregate_type, aggregate_id, event_type, payload, trace_id, attempts
FROM outbox
WHERE published_at IS NULL
ORDER BY id
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkPublished :exec
UPDATE outbox SET published_at = now() WHERE id = ANY(@ids::uuid[]);

-- name: MarkFailed :exec
UPDATE outbox
SET attempts = attempts + 1, last_error = $2
WHERE id = $1;

-- Bí danh `o` ở truy vấn con là bắt buộc, không phải cho đẹp: thiếu nó sqlc từ
-- chối phân tích ("column reference published_at is ambiguous") vì cùng một tên
-- bảng xuất hiện ở hai tầng. Postgres hiểu, sqlc thì không.
-- name: DeletePublishedBefore :execrows
DELETE FROM outbox
WHERE id IN (
    SELECT o.id FROM outbox o
    WHERE o.published_at IS NOT NULL AND o.published_at < $1
    LIMIT $2
);

-- name: DeleteProcessedBefore :execrows
DELETE FROM processed_events
WHERE ctid IN (
    SELECT p.ctid FROM processed_events p
    WHERE p.processed_at < $1
    LIMIT $2
);

-- Trả về :execrows chứ không phải :exec — số dòng ảnh hưởng CHÍNH LÀ câu trả
-- lời cho "event này đã xử lý chưa": 1 là lần đầu, 0 là trùng, bỏ qua.
-- name: MarkProcessed :execrows
INSERT INTO processed_events (consumer, event_id)
VALUES ($1, $2)
ON CONFLICT (consumer, event_id) DO NOTHING;
