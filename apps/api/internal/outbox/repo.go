package outbox

import (
	"context"
	"fmt"
	"time"

	"base-ecommerce/api/internal/outbox/gen"
	"base-ecommerce/api/internal/platform/postgres"

	"github.com/google/uuid"
)

// maxLastErrorLen là trần độ dài (tính bằng RUNE) của cột last_error.
//
// Không có trần này thì bảng phình rất nhanh: lỗi từ pgx có thể dài vài KB
// (kèm cả câu SQL và chi tiết ràng buộc), và relay ghi lại nguyên chuỗi đó sau
// MỖI lần thử. Năm trăm ký tự đủ để nhận ra lỗi gì; muốn đầy đủ thì đọc log.
const maxLastErrorLen = 500

// Repository là cổng ghi/đọc bảng outbox và processed_events.
//
// Giữ *postgres.Manager chứ không giữ *pgxpool.Pool, giống mọi repository khác
// của dự án: nhờ đó cùng một đối tượng chạy được cả trong lẫn ngoài
// transaction, do Manager.DB(ctx) tự lấy pgx.Tx ra khỏi context khi có.
type Repository struct{ db *postgres.Manager }

func NewRepository(db *postgres.Manager) *Repository { return &Repository{db: db} }

// Append ghi các sự kiện vào bảng outbox.
//
// PHẢI ĐƯỢC GỌI TRONG CÙNG TRANSACTION VỚI DỮ LIỆU NGHIỆP VỤ. Đó là toàn bộ lý
// do outbox tồn tại: nếu ghi sản phẩm và ghi sự kiện nằm ở hai transaction
// khác nhau (hay tệ hơn, publish thẳng ra RabbitMQ), thì một lần crash ở giữa
// để lại hoặc sự kiện không có dữ liệu, hoặc dữ liệu không có sự kiện — bài
// toán dual-write. Cùng một transaction thì hai thứ đó cùng sống hoặc cùng
// chết.
//
// Không cần truyền tx vào đây: Manager.DB(ctx) tự lấy pgx.Tx từ context, nên
// chỉ cần GỌI ĐÚNG CHỖ, tức bên trong hàm mà Manager.Run đang chạy. Gọi ngoài
// transaction vẫn ghi được và KHÔNG báo lỗi — đó chính là cái bẫy, nên chỗ gọi
// phải được đọc kỹ chứ đừng trông vào lỗi runtime.
func (r *Repository) Append(ctx context.Context, recs ...Record) error {
	if len(recs) == 0 {
		return nil
	}
	q := gen.New(r.db.DB(ctx))
	for _, rec := range recs {
		if err := q.AppendOutbox(ctx, gen.AppendOutboxParams{
			ID:            rec.ID,
			AggregateType: rec.AggregateType,
			AggregateID:   rec.AggregateID,
			EventType:     rec.EventType,
			Payload:       rec.Payload,
			TraceID:       nullable(rec.TraceID),
		}); err != nil {
			return fmt.Errorf("ghi sự kiện %s vào outbox: %w", rec.EventType, err)
		}
	}
	return nil
}

// FetchUnpublished lấy tối đa limit sự kiện chưa gửi và KHÓA chúng
// (FOR UPDATE SKIP LOCKED).
//
// BẮT BUỘC chạy trong transaction. Ngoài transaction, pgx tự mở rồi đóng
// transaction ngầm cho câu lệnh, nên khóa được nhả ngay khi hàm này trả về:
// SKIP LOCKED mất tác dụng và hai tiến trình cùng đọc ra cùng một lô sự kiện
// rồi publish trùng. Cách dùng đúng là gói cả vòng đời một lô — fetch,
// publish, MarkPublished/MarkFailed — trong một Manager.Run.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int32) ([]Record, error) {
	rows, err := gen.New(r.db.DB(ctx)).FetchUnpublished(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("đọc sự kiện chưa gửi: %w", err)
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		rec := Record{
			ID:            row.ID,
			AggregateType: row.AggregateType,
			AggregateID:   row.AggregateID,
			EventType:     row.EventType,
			Payload:       row.Payload,
			Attempts:      int(row.Attempts),
		}
		if row.TraceID != nil {
			rec.TraceID = *row.TraceID
		}
		out = append(out, rec)
	}
	return out, nil
}

// MarkPublished đánh dấu cả lô đã gửi thành công.
//
// Chỉ gọi SAU khi broker đã xác nhận nhận được (publisher confirm). Đánh dấu
// trước khi có confirm là tự tay vứt sự kiện đi khi broker từ chối.
func (r *Repository) MarkPublished(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	if err := gen.New(r.db.DB(ctx)).MarkPublished(ctx, ids); err != nil {
		return fmt.Errorf("đánh dấu %d sự kiện đã gửi: %w", len(ids), err)
	}
	return nil
}

// MarkFailed tăng attempts và lưu lý do thất bại của lần thử gần nhất.
//
// Không set published_at, nên lần poll sau sự kiện vẫn được lấy lại — đây là
// cơ chế thử lại, không phải chỗ để bỏ sự kiện đi.
func (r *Repository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	msg := truncateRunes(errMsg, maxLastErrorLen)
	if err := gen.New(r.db.DB(ctx)).MarkFailed(ctx, gen.MarkFailedParams{
		ID:        id,
		LastError: &msg,
	}); err != nil {
		return fmt.Errorf("đánh dấu sự kiện %s thất bại: %w", id, err)
	}
	return nil
}

// DeletePublishedBefore xóa theo lô các sự kiện đã gửi trước mốc before, trả về
// số dòng đã xóa. Bên gọi lặp lại tới khi trả về 0 nếu muốn dọn hết.
func (r *Repository) DeletePublishedBefore(ctx context.Context, before time.Time, limit int32) (int64, error) {
	n, err := gen.New(r.db.DB(ctx)).DeletePublishedBefore(ctx, gen.DeletePublishedBeforeParams{
		Before: before,
		Lim:    limit,
	})
	if err != nil {
		return 0, fmt.Errorf("dọn sự kiện đã gửi: %w", err)
	}
	return n, nil
}

// DeleteProcessedBefore dọn bảng khử trùng lặp theo lô, trả về số dòng đã xóa.
//
// Mốc before phải LỚN HƠN nhiều thời gian một sự kiện còn có thể được gửi lại
// (TTL của queue, thời gian giữ ở DLQ). Dọn quá sớm thì dấu vết "đã xử lý" mất
// trước bản sao cuối cùng, và consumer xử lý lại lần nữa.
func (r *Repository) DeleteProcessedBefore(ctx context.Context, before time.Time, limit int32) (int64, error) {
	n, err := gen.New(r.db.DB(ctx)).DeleteProcessedBefore(ctx, gen.DeleteProcessedBeforeParams{
		ProcessedAt: before,
		Limit:       limit,
	})
	if err != nil {
		return 0, fmt.Errorf("dọn dấu vết đã xử lý: %w", err)
	}
	return n, nil
}

// MarkProcessed ghi dấu "consumer này đã xử lý sự kiện này".
//
// Trả về true khi dòng MỚI được thêm, tức đây là lần đầu — bên gọi cứ thế xử
// lý sự kiện. Trả về false nghĩa là đã có dấu vết từ trước: bỏ qua và ack.
//
// false KHÔNG PHẢI LỖI, nên hàm này không trả error cho trường hợp đó. Giao
// hàng at-least-once khiến sự kiện trùng là chuyện bình thường xảy ra hằng
// ngày (relay chết sau khi publish nhưng trước khi commit, broker gửi lại sau
// khi consumer nack...). Coi trùng lặp là lỗi sẽ khiến log đầy báo động giả và
// che mất lỗi thật.
//
// Phải gọi TRONG CÙNG TRANSACTION với việc xử lý sự kiện, vì lý do giống hệt
// Append: ghi dấu vết và ghi kết quả xử lý phải cùng sống hoặc cùng chết.
func (r *Repository) MarkProcessed(ctx context.Context, consumer string, eventID uuid.UUID) (bool, error) {
	n, err := gen.New(r.db.DB(ctx)).MarkProcessed(ctx, gen.MarkProcessedParams{
		Consumer: consumer,
		EventID:  eventID,
	})
	if err != nil {
		return false, fmt.Errorf("ghi dấu vết đã xử lý (%s/%s): %w", consumer, eventID, err)
	}
	return n > 0, nil
}

// nullable đổi chuỗi rỗng thành NULL, để một cột tùy chọn không chứa lẫn lộn
// cả NULL lẫn chuỗi rỗng với cùng một ý nghĩa "không có".
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// truncateRunes cắt s về tối đa max rune.
//
// Cắt theo RUNE chứ không theo byte: s[:max] trên chuỗi tiếng Việt sẽ cắt giữa
// một ký tự UTF-8 nhiều byte, để lại byte lẻ ở cuối. Postgres từ chối chuỗi
// như vậy với lỗi "invalid byte sequence for encoding UTF8" — lỗi phát sinh từ
// chính hàm ghi lỗi, làm mất luôn thông tin thất bại gốc.
//
// Duyệt bằng `for i := range s` thay vì []rune(s): range trên string đi theo
// biên rune sẵn, còn []rune cấp phát một mảng bằng cả chuỗi — lãng phí đúng
// với trường hợp ta lo nhất là chuỗi lỗi dài vài KB.
func truncateRunes(s string, max int) string {
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}
