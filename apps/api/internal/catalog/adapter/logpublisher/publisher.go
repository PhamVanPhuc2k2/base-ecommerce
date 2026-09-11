// Package logpublisher cài đặt port app.EventPublisher bằng cách ghi log.
//
// Đây là bản TẠM của P0.2. P0.3 sẽ thay bằng adapter ghi vào bảng outbox trong
// cùng transaction — và thay được mà KHÔNG phải sửa domain hay app. Đó chính là
// điều kiến trúc hexagonal hứa hẹn, và là cách kiểm chứng lời hứa đó.
//
// Giới hạn phải biết: bản này KHÔNG bền. Tiến trình chết là sự kiện mất. Không
// dùng nó để kích hoạt bất cứ thứ gì quan trọng cho tới khi P0.3 xong.
package logpublisher

import (
	"context"
	"log/slog"

	"base-ecommerce/api/internal/catalog/domain"
)

type Publisher struct{ log *slog.Logger }

func New(log *slog.Logger) *Publisher { return &Publisher{log: log} }

func (p *Publisher) Publish(ctx context.Context, events ...domain.Event) error {
	for _, e := range events {
		p.log.InfoContext(ctx, "domain event",
			"event_id", e.EventID().String(),
			"event_type", e.EventType(),
			"aggregate_id", e.AggregateID().String(),
			"occurred_at", e.OccurredAt(),
		)
	}
	return nil
}
