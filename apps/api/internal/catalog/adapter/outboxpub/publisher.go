// Package outboxpub cài đặt port app.EventPublisher bằng bảng outbox.
//
// # Đây là chỗ kiến trúc hexagonal trả công
//
// P0.2 cài EventPublisher bằng logpublisher (ghi log, không bền). Package này
// thay hẳn chỗ đó bằng ghi xuống database — mà KHÔNG sửa một dòng nào trong
// catalog/domain hay catalog/app. Use case vẫn gọi đúng một câu
// `uc.events.Publish(ctx, events...)` như cũ; thứ duy nhất đổi là dòng lắp ráp
// trong module.go. Đó chính là điều hexagonal hứa hẹn, và cũng là phép kiểm
// chứng lời hứa đó: nếu phải sửa app/ hay domain/ để thay được hạ tầng thì
// kiến trúc đã sai ở đâu đó.
//
// # Ranh giới dịch thuật
//
// Package này là chỗ duy nhất biết CẢ HAI phía: catalog/domain.Event (kiểu
// nghiệp vụ, sống trong module catalog) và outbox.Record (kiểu hạ tầng thuần).
// Việc dịch nằm ở đây chứ không nằm trong internal/outbox, để internal/outbox
// không cần biết catalog tồn tại — nếu nó biết thì module orders ở P4 sẽ kéo
// theo cả catalog vào đồ thị phụ thuộc chỉ vì dùng chung outbox. Chiều phụ
// thuộc luôn là: module -> outbox, không bao giờ ngược lại.
//
// # Bền được là nhờ GỌI ĐÚNG CHỖ
//
// Publish chỉ an toàn khi được gọi BÊN TRONG hàm mà TxManager.Run đang chạy:
// outbox.Repository lấy pgx.Tx ra khỏi context, nên lúc đó dòng outbox và dữ
// liệu nghiệp vụ cùng commit hoặc cùng rollback. Gọi ngoài transaction vẫn ghi
// được và KHÔNG báo lỗi nào — nên đừng trông vào lỗi runtime để phát hiện, hãy
// đọc kỹ use case (xem app/create_product.go).
package outboxpub

import (
	"context"
	"encoding/json"
	"fmt"

	"base-ecommerce/api/internal/catalog/domain"
	"base-ecommerce/api/internal/outbox"

	"github.com/go-chi/chi/v5/middleware"
)

// appender là phần DUY NHẤT của outbox.Repository mà adapter này cần.
//
// Khai ở phía người dùng chứ không nhận thẳng *outbox.Repository, để hợp đồng
// giữa catalog và hạ tầng outbox chỉ rộng đúng bằng thứ thật sự được dùng:
// thêm method vào Repository (relay sẽ thêm ở Task 6) không làm rộng bề mặt mà
// module catalog phụ thuộc vào.
type appender interface {
	Append(ctx context.Context, recs ...outbox.Record) error
}

type Publisher struct{ repo appender }

func New(repo appender) *Publisher { return &Publisher{repo: repo} }

// Publish dịch các sự kiện nghiệp vụ thành dòng outbox rồi ghi cả lô.
//
// Đọc mọi thứ qua interface domain.Event, KHÔNG type-switch theo từng loại
// event: thêm event mới thì không phải sửa file này, và cũng không thể quên
// sửa — vì Event bắt mỗi event tự khai AggregateType() lẫn Payload().
func (p *Publisher) Publish(ctx context.Context, events ...domain.Event) error {
	// Không có sự kiện thì không chạm database. Append cũng tự xử lý trường
	// hợp rỗng, nhưng chặn sớm ở đây tránh cấp phát slice vô ích cho nhánh
	// phổ biến nhất (use case chỉ đọc, không sinh event).
	if len(events) == 0 {
		return nil
	}

	recs := make([]outbox.Record, 0, len(events))
	for _, e := range events {
		payload, err := json.Marshal(e.Payload())
		if err != nil {
			// Không nuốt lỗi: payload không marshal được nghĩa là event sẽ đi
			// ra với nội dung rỗng, và consumer không có cách nào biết. Trả
			// lỗi ở đây làm cả transaction rollback — dữ liệu nghiệp vụ cũng
			// không được ghi, và người gọi biết ngay là hỏng.
			return fmt.Errorf("marshal payload của %s: %w", e.EventType(), err)
		}
		recs = append(recs, outbox.Record{
			ID:            e.EventID(),
			AggregateType: e.AggregateType(),
			AggregateID:   e.AggregateID(),
			EventType:     e.EventType(),
			Payload:       payload,
			// TraceID nối dòng log của worker (chạy bất đồng bộ, có thể hàng
			// phút sau) với request HTTP đã sinh ra sự kiện. Không có nó thì
			// gỡ lỗi bất đồng bộ gần như bất khả thi: chỉ còn cách mò theo
			// thời gian. Ngoài ngữ cảnh request (ví dụ job nền) thì
			// GetReqID trả chuỗi rỗng và Repository ghi xuống NULL.
			TraceID: middleware.GetReqID(ctx),
		})
	}

	return p.repo.Append(ctx, recs...)
}
