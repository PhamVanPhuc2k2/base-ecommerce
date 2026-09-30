package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrUnknownEmail: sự kiện trỏ tới thư không còn trong hàng đợi (bị xóa tay).
// Lỗi Go thường, không phải errs.Error: chỉ worker gặp nó, không bao giờ ra
// HTTP, nên không có mã lỗi API nào cho nó.
var ErrUnknownEmail = errors.New("không tìm thấy thư trong hàng đợi")

// OutboundEmail là một thư đã soạn, chờ worker gửi.
//
// Thư nằm trong bảng riêng chứ không trong payload outbox, vì nội dung có mã
// OTP — payload outbox đi qua RabbitMQ và vào log (đặc tả P2.3 mục 2.1).
type OutboundEmail struct {
	ID        uuid.UUID
	To        string
	Subject   string
	Body      string
	CreatedAt time.Time
	SentAt    *time.Time
}

// NewOutboundEmail soạn thư và sinh sự kiện EmailQueued đi kèm. Hai thứ phải
// được ghi trong CÙNG transaction — thư không có sự kiện thì không ai gửi, sự
// kiện không có thư thì worker gặp ErrUnknownEmail.
func NewOutboundEmail(to, subject, body string) (*OutboundEmail, EmailQueued) {
	e := &OutboundEmail{ID: uuid.Must(uuid.NewV7()), To: to, Subject: subject, Body: body,
		CreatedAt: time.Now().UTC()}
	return e, EmailQueued{baseEvent: newBase(e.ID)}
}

// EmailQueued báo có thư mới trong hàng đợi. Payload CHỈ có id — tuyệt đối
// không nhét người nhận hay nội dung vào đây.
type EmailQueued struct {
	baseEvent
}

func (EmailQueued) EventType() string     { return "email.queued" }
func (EmailQueued) AggregateType() string { return "email" }
func (e EmailQueued) Payload() any {
	return map[string]string{"email_id": e.AggregateID().String()}
}
