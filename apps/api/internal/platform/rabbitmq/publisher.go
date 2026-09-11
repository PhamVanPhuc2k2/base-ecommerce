// Package rabbitmq bọc kết nối AMQP, khai báo topology và publish có xác nhận.
//
// # Luật phụ thuộc
//
// Package này TUYỆT ĐỐI KHÔNG được import module nghiệp vụ nào
// (internal/catalog, internal/outbox, và sau này internal/orders...). Nó nhận
// một routing key và một mảng byte, không biết sự kiện là gì. Lý do y hệt như
// ở internal/outbox: Go không có import vòng, nên chỉ cần hạ tầng import một
// module nghiệp vụ là mọi module khác dùng chung hạ tầng ấy sẽ kéo theo cả
// module kia vào đồ thị phụ thuộc. Việc dịch từ sự kiện của module sang byte là
// trách nhiệm của adapter thuộc module đó.
//
// # Hỏng thì chậm đi, không sập — nhưng "không sập" tùy tiến trình
//
// Giống platform/redis, NewPublisher không trả lỗi khi broker chết: với cmd/api
// thì RabbitMQ chết KHÔNG sao, vì use case chỉ ghi vào bảng outbox trong cùng
// transaction với dữ liệu nghiệp vụ, và relay sẽ đẩy sau. Nhưng với
// cmd/outboxrelay thì RabbitMQ chính là lý do tiến trình đó tồn tại. Chỗ khác
// biệt được xử lý ở HealthChecker (xem health.go), không phải ở đây.
package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"base-ecommerce/api/internal/platform/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

// dialTimeout là trần cho MỘT lần bắt tay với broker. Ngắn, vì mọi chỗ gọi
// Publish đều đang giữ một thứ gì đó (một dòng outbox đang chờ, một request
// HTTP): thà báo lỗi nhanh để vòng lặp relay thử lại còn hơn treo.
const dialTimeout = 5 * time.Second

// heartbeatInterval để hai phía phát hiện kết nối đã chết. Không có nó, một kết
// nối TCP bị tường lửa im lặng cắt đứt vẫn "mở" về phía Go: Publish sẽ treo cho
// tới khi hết context thay vì hỏng ngay và nối lại.
const heartbeatInterval = 10 * time.Second

// Publisher đẩy message vào ExchangeEvents và CHỜ broker xác nhận đã nhận.
//
// # Vì sao tự nối lại chứ không nối một lần lúc khởi động
//
// "Kết nối lúc khởi động, hỏng thì thôi" là mô hình sai cho tiến trình sống
// lâu: RabbitMQ restart (nâng cấp, di chuyển node) là việc bình thường, và một
// relay chết theo broker sẽ nằm im cho tới khi có người khởi động lại nó —
// đúng lúc outbox đang dồn ứ. Ở đây kết nối được dựng LƯỜI và dựng lại mỗi khi
// phát hiện đã đứt: lần Publish kế tiếp sau sự cố tự nối lại, không cần ai can
// thiệp, không cần vòng lặp reconnect riêng chạy nền.
//
// Cố ý KHÔNG dùng amqp.Config.Recovery của thư viện: ở v1.14 nó vẫn được đánh
// dấu Experimental, và nó khôi phục cả consumer lẫn topology theo cách khó
// đoán. Nối lại lười ở đây chỉ vài chục dòng và hành vi thì rõ ràng.
type Publisher struct {
	url string
	log *slog.Logger

	// mu bảo vệ conn/ch. Chỉ giữ trong lúc dựng hoặc vứt kết nối, KHÔNG giữ
	// trong lúc chờ confirm — chờ confirm mất cả chục mili giây và giữ khóa
	// suốt thời gian đó sẽ biến mọi publish thành một hàng đợi nối tiếp.
	// *amqp.Channel tự nó đã an toàn khi dùng đồng thời.
	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewPublisher dựng publisher và thử kết nối ngay một lần.
//
// KHÔNG trả lỗi khi broker chết — chỉ ghi WARN, y như platform/redis. Lần thử
// đầu này chỉ để lỗi cấu hình (sai URL, sai mật khẩu) lộ ra ngay ở dòng log đầu
// tiên thay vì tới lúc có message đầu tiên; còn broker chưa lên thì lần Publish
// đầu tiên sẽ nối lại.
func NewPublisher(ctx context.Context, cfg config.RabbitMQ, log *slog.Logger) *Publisher {
	p := &Publisher{url: cfg.URL, log: log}
	if _, err := p.channel(ctx); err != nil {
		p.log.Warn("không nối được rabbitmq lúc khởi động — sẽ tự nối lại ở lần publish sau",
			"err", err)
	}
	return p
}

// Publish gửi body vào ExchangeEvents với routingKey và CHỜ broker xác nhận.
//
// Trả nil chỉ khi broker đã ack. Mọi trường hợp khác đều là lỗi, kể cả khi
// message có thể đã rời khỏi tiến trình: bên gọi (relay) phải coi như CHƯA gửi
// và thử lại. Gửi trùng còn cứu được bằng khử trùng lặp ở consumer; mất hẳn thì
// không.
//
// Hạn chế đã biết: chữ ký này chỉ chở được routing key và thân message, nên id
// sự kiện (outbox.Record.ID) và trace id phải nằm SẴN trong body. Task 6/7 nếu
// cần chúng ở dạng thuộc tính AMQP (MessageId, headers) thì phải mở rộng chữ ký
// — đừng lén nhét vào routing key.
func (p *Publisher) Publish(ctx context.Context, routingKey string, body []byte) error {
	ch, err := p.channel(ctx)
	if err != nil {
		return err
	}

	conf, err := ch.PublishWithDeferredConfirmWithContext(ctx, ExchangeEvents, routingKey,
		// mandatory: true — message không route được tới queue nào sẽ bị TRẢ
		// VỀ thay vì biến mất trong im lặng. Nhưng nó chỉ có tác dụng khi có
		// goroutine đọc ch.NotifyReturn: không đọc thì thư viện vứt message
		// trả về y hệt như khi không bật cờ này. Goroutine đó là watchReturns,
		// khởi động cùng lúc với channel.
		true,
		// immediate: từ RabbitMQ 3.0 không còn được hỗ trợ, bật lên là broker
		// đóng kênh. Luôn false.
		false,
		amqp.Publishing{
			// DeliveryMode: Persistent — thiếu nó thì message chỉ nằm trong
			// RAM của broker và bay mất khi broker restart, dù queue durable.
			// Outbox bảo đảm message TỚI ĐƯỢC broker; nó không bảo đảm broker
			// GIỮ ĐƯỢC message. Hai việc khác nhau, và chỉ có phép thử restart
			// broker mới phát hiện được thiếu dòng này.
			DeliveryMode: amqp.Persistent,
			// Quy ước toàn dự án: payload sự kiện luôn là JSON (cột payload
			// của bảng outbox là JSONB nên Postgres đã chặn mọi thứ không phải
			// JSON từ trước đó rồi).
			ContentType: "application/json",
			Timestamp:   time.Now().UTC(),
			Body:        body,
		})
	if err != nil {
		// Lỗi ở đây gần như luôn là kênh/kết nối đã chết. Vứt nó đi để lần gọi
		// sau dựng lại, thay vì kẹt vĩnh viễn với một kênh hỏng.
		p.discard(ch)
		return fmt.Errorf("publish routing key %q: %w", routingKey, err)
	}

	// CHỜ confirm. Đây là dòng làm cho cả cơ chế outbox có nghĩa.
	//
	// Không có Confirm(false) ở connectLocked cộng với chỗ chờ này thì
	// PublishWithContext trả về thành công ngay khi ghi được vào socket — tức
	// là thành công cả khi broker chưa hề nhận, hoặc nhận rồi nhưng chết trước
	// khi kịp ghi xuống đĩa. Relay sẽ đánh dấu published_at cho một message
	// không tồn tại, và sự kiện mất vĩnh viễn trong im lặng. Toàn bộ công sức
	// ghi outbox trong cùng transaction đổ đi vì thiếu đúng một lần chờ.
	ok, err := conf.WaitContext(ctx)
	if err != nil {
		p.discard(ch)
		return fmt.Errorf("chờ confirm cho routing key %q: %w", routingKey, err)
	}
	if !ok {
		// nack: broker đã nhận nhưng từ chối nhận trách nhiệm (hết đĩa, lỗi
		// nội bộ), hoặc kênh đứt khi message còn treo. Không được coi là đã
		// gửi xong.
		return fmt.Errorf("broker nack message với routing key %q — coi như CHƯA gửi", routingKey)
	}
	return nil
}

// Close đóng kết nối. An toàn khi gọi nhiều lần.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeLocked()
}

// channel trả về một kênh dùng được, dựng lại kết nối nếu cần.
func (p *Publisher) channel(ctx context.Context) (*amqp.Channel, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// IsClosed() bắt được cả trường hợp broker đóng kênh vì lỗi giao thức (ví
	// dụ 406 lúc khai báo lại queue với tham số khác), không chỉ trường hợp
	// mất mạng.
	if p.ch != nil && !p.ch.IsClosed() && p.conn != nil && !p.conn.IsClosed() {
		return p.ch, nil
	}
	_ = p.closeLocked()

	if err := p.connectLocked(ctx); err != nil {
		return nil, err
	}
	return p.ch, nil
}

func (p *Publisher) connectLocked(ctx context.Context) error {
	// Không để một lần dial kéo dài hơn phần thời gian còn lại của người gọi:
	// /readyz chỉ cho 2 giây, mà dialTimeout mặc định là 5.
	timeout := dialTimeout
	if dl, ok := ctx.Deadline(); ok {
		if remaining := time.Until(dl); remaining < timeout {
			timeout = remaining
		}
	}
	if timeout <= 0 {
		return fmt.Errorf("nối rabbitmq: %w", context.DeadlineExceeded)
	}

	conn, err := amqp.DialConfig(p.url, amqp.Config{
		Heartbeat: heartbeatInterval,
		Locale:    "en_US",
		Dial:      amqp.DefaultDial(timeout),
	})
	if err != nil {
		return fmt.Errorf("nối rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mở kênh rabbitmq: %w", err)
	}

	// Bật chế độ confirm. Phải gọi TRƯỚC mọi publish trên kênh này; gọi sau thì
	// những message đã gửi không có confirm nào cả.
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("bật publisher confirm: %w", err)
	}

	// Khai báo topology trên MỌI kết nối, kể cả của tiến trình chỉ publish.
	// Vì sao: publish vào một exchange không tồn tại làm broker đóng kênh, nên
	// thứ tự khởi động của các tiến trình sẽ quyết định hệ thống chạy hay
	// không. Declare idempotent nên trả giá gần như bằng không.
	if err := Declare(ch); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}

	p.conn, p.ch = conn, ch
	go p.watchReturns(ch)
	go p.watchClose(conn)
	return nil
}

func (p *Publisher) closeLocked() error {
	var errs []error
	if p.ch != nil {
		if err := p.ch.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			errs = append(errs, err)
		}
		p.ch = nil
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			errs = append(errs, err)
		}
		p.conn = nil
	}
	return errors.Join(errs...)
}

// discard vứt kênh hỏng đi để lần Publish sau nối lại.
//
// Chỉ vứt khi ch VẪN là kênh hiện tại: nếu một goroutine khác đã nối lại rồi
// thì kênh mới là kênh tốt, đóng nó đi sẽ gây ra đúng sự cố mà ta đang sửa.
func (p *Publisher) discard(ch *amqp.Channel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch == ch {
		_ = p.closeLocked()
	}
}

// watchReturns ghi log message bị broker trả về vì không route được.
//
// Đây là nửa còn lại của cờ mandatory. Thiếu goroutine này, message không khớp
// binding nào sẽ bị thư viện vứt đi trong im lặng và Publish vẫn trả nil —
// broker ack message nó nhận được, còn việc không queue nào nhận là chuyện
// khác. Mức ERROR chứ không phải WARN: đây luôn là lỗi cấu hình (sai routing
// key hoặc thiếu binding) và luôn đồng nghĩa với mất dữ liệu.
//
// Vòng lặp tự kết thúc khi kênh đóng, vì thư viện đóng luôn channel này.
func (p *Publisher) watchReturns(ch *amqp.Channel) {
	for r := range ch.NotifyReturn(make(chan amqp.Return, 1)) {
		p.log.Error("rabbitmq TRẢ VỀ message: không queue nào nhận — message bị mất",
			"exchange", r.Exchange,
			"routing_key", r.RoutingKey,
			"reply_code", r.ReplyCode,
			"reply_text", r.ReplyText,
			"body", string(r.Body))
	}
}

// watchClose ghi log lúc kết nối đứt, để trong log có mốc thời gian giải thích
// vì sao loạt lỗi publish ngay sau đó xuất hiện.
func (p *Publisher) watchClose(conn *amqp.Connection) {
	if err := <-conn.NotifyClose(make(chan *amqp.Error, 1)); err != nil {
		p.log.Warn("mất kết nối rabbitmq — sẽ nối lại ở lần publish sau", "err", err)
	}
}
