package domain

import (
	"time"

	"github.com/google/uuid"
)

// Event là sự kiện nghiệp vụ do entity phát ra.
//
// P0.2 chỉ ghi log. P0.3 sẽ ghi chúng vào bảng outbox trong CÙNG transaction
// với dữ liệu nghiệp vụ — và làm được điều đó mà không phải sửa file này.
type Event interface {
	EventID() uuid.UUID
	EventType() string
	AggregateID() uuid.UUID
	OccurredAt() time.Time
}

type baseEvent struct {
	id          uuid.UUID
	aggregateID uuid.UUID
	occurredAt  time.Time
}

func (e baseEvent) EventID() uuid.UUID     { return e.id }
func (e baseEvent) AggregateID() uuid.UUID { return e.aggregateID }
func (e baseEvent) OccurredAt() time.Time  { return e.occurredAt }

func newBase(aggregateID uuid.UUID) baseEvent {
	// UUIDv7 có thứ tự theo thời gian — quan trọng cho outbox ở P0.3.
	id, err := uuid.NewV7()
	if err != nil {
		id = uuid.New()
	}
	return baseEvent{id: id, aggregateID: aggregateID, occurredAt: time.Now().UTC()}
}

type ProductCreated struct{ baseEvent }

func (ProductCreated) EventType() string { return "product.created" }

type ProductUpdated struct{ baseEvent }

func (ProductUpdated) EventType() string { return "product.updated" }

type ProductPublished struct{ baseEvent }

func (ProductPublished) EventType() string { return "product.published" }
