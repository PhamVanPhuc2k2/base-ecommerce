package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/rabbitmq"

	"github.com/google/uuid"
)

// mailWorker gửi thư khi nhận sự kiện email.queued (đặc tả P2.3 mục 2.1).
//
// Khử trùng lặp KHÔNG qua processed_events như catalog-indexer: cột sent_at
// của chính bức thư đã là dấu "đã xử lý", và nó được ghi cùng transaction với
// việc gửi (usecase.Mailing.Deliver).
type mailWorker struct {
	mailing *usecase.Mailing
	log     *slog.Logger
}

func (m *mailWorker) handle(ctx context.Context, d rabbitmq.Delivery) error {
	var ev struct {
		EventID string `json:"event_id"`
		Payload struct {
			EmailID string `json:"email_id"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		return rabbitmq.Permanent(fmt.Errorf("giải mã envelope: %w", err))
	}
	id, err := uuid.Parse(ev.Payload.EmailID)
	if err != nil {
		return rabbitmq.Permanent(fmt.Errorf("email_id %q không phải uuid: %w", ev.Payload.EmailID, err))
	}
	sent, err := m.mailing.Deliver(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrUnknownEmail) {
			return rabbitmq.Permanent(err) // thư đã bị xóa — thử lại cũng không hiện ra
		}
		// SMTP chết, database chết: tạm thời, sang queue retry.
		return err
	}
	if !sent {
		m.log.Debug("thư đã gửi trước đó, bỏ qua bản trùng", "email_id", id, "event_id", ev.EventID)
		return nil
	}
	// KHÔNG ghi người nhận hay nội dung: nội dung có mã OTP, người nhận là dữ
	// liệu cá nhân. email_id đủ để tra lại trong DB khi cần.
	m.log.Info("đã gửi thư", "email_id", id, "event_id", ev.EventID, "trace_id", d.TraceID, "attempts", d.Attempts)
	return nil
}
