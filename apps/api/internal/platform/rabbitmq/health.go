package rabbitmq

import "context"

// HealthChecker cài đặt health.Checker cho RabbitMQ.
//
// # Vì sao Optional là THAM SỐ chứ không phải hằng true như ở Redis
//
// platform/redis trả Optional() = true cứng, và đúng: Redis chết thì API vẫn
// phục vụ đúng, chỉ chậm hơn. Với RabbitMQ thì câu trả lời phụ thuộc vào việc
// AI đang hỏi:
//
//   - cmd/api: optional = TRUE. Broker chết không ảnh hưởng gì tới request —
//     use case chỉ ghi vào bảng outbox trong cùng transaction với dữ liệu
//     nghiệp vụ, relay sẽ đẩy sau. Cho nó làm /readyz trả 503 nghĩa là một sự
//     cố của broker sẽ rút hết instance API khỏi load balancer trong khi từng
//     instance vẫn trả 200 cho mọi endpoint thật — biến sự cố bất đồng bộ
//     thành sự cố toàn hệ thống, đúng cái bẫy mà health.Optional sinh ra để
//     tránh.
//
//   - cmd/outboxrelay (và cmd/worker): optional = FALSE. Broker chính là lý do
//     hai tiến trình đó tồn tại. Một relay không nối được broker thì không làm
//     được việc gì cả, và /readyz của nó PHẢI đỏ để cảnh báo nổ và để hạ tầng
//     biết mà khởi động lại — báo "ok, chỉ hơi kém" cho một tiến trình đang
//     không đẩy nổi một message nào là nói dối.
//
// Cùng một phụ thuộc, hai câu trả lời khác nhau, vì /readyz trả lời câu hỏi
// "tiến trình NÀY có làm được việc của nó không?" chứ không phải "thứ kia có
// sống không?".
type HealthChecker struct {
	p        *Publisher
	optional bool
}

// NewHealthChecker dựng checker. optional = true cho cmd/api, false cho các
// tiến trình mà RabbitMQ là điều kiện sống còn (relay, worker). Xem giải thích
// ở HealthChecker.
func NewHealthChecker(p *Publisher, optional bool) *HealthChecker {
	return &HealthChecker{p: p, optional: optional}
}

func (h *HealthChecker) Name() string { return "rabbitmq" }

func (h *HealthChecker) Optional() bool { return h.optional }

// Check thử lấy kênh, nối lại nếu đang đứt.
//
// Cố ý KHÔNG chỉ đọc trạng thái kết nối đang giữ: làm vậy thì sau khi broker
// sống lại, /readyz vẫn đỏ cho tới khi tình cờ có ai đó gọi Publish. Ở đây
// health check chủ động nối lại, nên nó vừa báo đúng vừa rút ngắn thời gian
// hồi phục. Thời gian dial bị chặn bởi deadline của ctx (health.Handler cho 2
// giây), nên broker chết cũng không làm /readyz treo.
func (h *HealthChecker) Check(ctx context.Context) error {
	_, err := h.p.channel(ctx)
	return err
}
