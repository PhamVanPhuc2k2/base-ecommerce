package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"base-ecommerce/api/internal/platform/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// prefetchCount là số message broker được phép đẩy trước cho consumer này
	// khi chưa có ack.
	//
	// ⚠️ ĐỪNG BỎ Qos ĐI. Mặc định của AMQP là KHÔNG GIỚI HẠN: broker đẩy toàn bộ
	// queue sang consumer ngay khi nó đăng ký, và mọi message đó nằm trong RAM
	// của tiến trình worker. Với một queue đang tồn đọng vài trăm nghìn message
	// thì worker bị OOM ngay giây đầu tiên sau khi khởi động — rồi khởi động
	// lại, rồi lại OOM, trong khi tồn đọng không hề rút. Lỗi này KHÔNG lộ ra
	// lúc chạy thử với queue rỗng.
	//
	// 10 là chỗ dung hòa: đủ để luôn có message sẵn trong bộ đệm nên vòng lặp
	// không phải chờ một vòng mạng sau mỗi lần ack, đủ nhỏ để một worker chết
	// giữa chừng chỉ trả lại 10 message cho broker giao cho bản khác.
	prefetchCount = 10

	// maxAttempts là số lần một message được phép QUAY VÒNG qua queue retry
	// trước khi bị đẩy sang DLQ. Với retryTTLMillis = 30 giây thì 5 lần là
	// khoảng 2 phút rưỡi — đủ cho một sự cố chớp nhoáng (Postgres failover,
	// deploy) kết thúc, đủ ngắn để một message hỏng thật không quay vòng cả
	// ngày và chiếm chỗ của message tốt.
	maxAttempts = 5

	// handleTimeout là trần thời gian cho MỘT message. Không có nó thì một
	// handler treo (truy vấn database không bao giờ trả về) giữ luôn cả vòng
	// lặp consume: message không được ack, prefetch cạn dần, worker đứng im mà
	// vẫn "đang chạy".
	handleTimeout = 30 * time.Second

	// reconnectDelay là khoảng nghỉ trước khi dựng lại phiên consume sau sự cố.
	// Không nối lại ngay lập tức: broker vừa chết thì mọi lần dial đều hỏng, và
	// một vòng lặp không nghỉ sẽ đốt CPU cùng log trong khi không giúp được gì.
	reconnectDelay = 5 * time.Second
)

// ErrPermanent đánh dấu một lỗi KHÔNG được thử lại.
//
// # Vì sao mặc định là "tạm thời", và lỗi vĩnh viễn phải nói rõ
//
// Hai loại lỗi này không đối xứng về hậu quả. Coi nhầm một lỗi vĩnh viễn thành
// tạm thời thì message quay vòng vài lần rồi vào DLQ — ồn ào, nhưng dữ liệu còn
// nguyên và có người vào xem. Coi nhầm một lỗi tạm thời thành vĩnh viễn thì sự
// kiện bị ném thẳng vào DLQ chỉ vì database vừa restart, và nó nằm đó tới khi
// có ai đó tình cờ nhìn vào. Nên chiều mặc định phải là chiều ít mất mát hơn:
// handler trả về một lỗi bất kỳ là THỬ LẠI, muốn vứt đi thì phải nói thẳng bằng
// Permanent().
//
// Chọn sentinel + errors.Is chứ không phải một kiểu lỗi riêng hay một enum trả
// kèm, vì nó cộng dồn được: handler gói lỗi bao nhiêu tầng bằng %w thì ý định
// "đừng thử lại" vẫn còn nguyên ở tầng ngoài cùng, mà không tầng nào phải biết
// về package này.
var ErrPermanent = errors.New("lỗi vĩnh viễn: đừng thử lại")

// Permanent gói err lại thành lỗi vĩnh viễn — consumer sẽ đẩy thẳng message
// sang DLQ thay vì cho nó quay vòng qua queue retry.
//
// Dùng cho những hỏng hóc mà thử lại chắc chắn vô ích: body không phải JSON,
// thiếu trường bắt buộc, event_id không phải UUID. Message như vậy thử lại 5
// lần vẫn hỏng y hệt, và trong lúc đó nó chiếm một trong 10 khe prefetch của
// message tốt.
func Permanent(err error) error {
	return fmt.Errorf("%w: %w", ErrPermanent, err)
}

// Delivery là một message đã tới tay handler.
//
// Cố ý KHÔNG để lộ amqp.Delivery ra ngoài: handler mà cầm được amqp.Delivery sẽ
// tự gọi Ack/Nack, và lúc đó việc quyết định số phận message nằm rải ở hai nơi.
// Ở đây chỉ có consumer ack, handler chỉ trả lỗi.
//
// Package này không biết Body chứa gì — nó là []byte, việc giải mã envelope là
// của module nghiệp vụ. Xem luật phụ thuộc ở đầu publisher.go.
type Delivery struct {
	RoutingKey string
	// MessageID là thuộc tính MessageId của AMQP; relay đặt nó bằng event_id.
	// Tiện cho log, nhưng ĐỪNG khử trùng lặp bằng nó: nó do bên gửi đặt và có
	// thể rỗng. Nguồn sự thật vẫn là event_id trong thân message.
	MessageID string
	TraceID   string
	Body      []byte
	// Attempts là số lần message đã quay vòng qua queue retry, đọc từ x-death.
	// 0 nghĩa là lần đầu.
	Attempts int
}

// Handler xử lý một message. Trả nil thì consumer ack.
//
// Lỗi trả về được phân loại theo đúng hai nhánh mô tả ở ErrPermanent: gói bằng
// Permanent() thì message sang DLQ, mọi lỗi khác thì message sang queue retry
// và tự quay lại sau 30 giây.
type Handler func(ctx context.Context, d Delivery) error

// Consumer đọc một queue với manual ack và tự dựng lại phiên khi kết nối đứt.
//
// # Vì sao không dùng chung kết nối với Publisher
//
// Vì hai bên hỏng theo kiểu khác nhau và phục hồi theo kiểu khác nhau.
// Publisher nối lại LƯỜI — lần Publish kế tiếp tự dựng kênh mới. Consumer thì
// không có "lần kế tiếp" nào để bám vào: nó ngồi chờ trên một kênh delivery,
// nên phải có vòng lặp chủ động dựng lại phiên, đăng ký lại consumer và đặt lại
// Qos. Nhét cả hai hành vi vào một đối tượng sẽ làm cả hai khó hiểu.
type Consumer struct {
	url   string
	queue string
	log   *slog.Logger
}

// NewConsumer dựng consumer. KHÔNG kết nối ngay — Run làm việc đó, vì Run vốn
// đã phải biết cách dựng lại phiên sau sự cố.
func NewConsumer(cfg config.RabbitMQ, queue string, log *slog.Logger) *Consumer {
	return &Consumer{url: cfg.URL, queue: queue, log: log}
}

// Run chạy tới khi ctx bị hủy. Trả nil khi dừng êm.
//
// Lỗi của một phiên KHÔNG làm Run thoát: broker restart (nâng cấp, di chuyển
// node) là việc bình thường, và một worker chết theo broker sẽ nằm im cho tới
// khi có người khởi động lại nó — đúng lúc queue đang dồn ứ. Cùng lập luận với
// vòng lặp của cmd/outboxrelay.
func (c *Consumer) Run(ctx context.Context, h Handler) error {
	for {
		err := c.session(ctx, h)

		// Kiểm tín hiệu tắt TRƯỚC khi xét err: lúc tắt, phiên hoàn toàn có thể
		// trả về lỗi (kết nối bị đóng giữa chừng) mà đó vẫn là dừng êm, không
		// phải sự cố. Báo động ở đây sẽ khiến mỗi lần deploy sinh một dòng
		// ERROR giả.
		//
		// Viết bằng select chứ không phải `if ctx.Err() != nil`: linter nilerr
		// coi mọi mẫu "kiểm .Err() khác nil rồi return nil" là bug, và ở đây
		// thì nó nhầm — nhưng sửa cách viết rẻ hơn là tắt linter đi.
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err != nil {
			c.log.Error("phiên consume hỏng, sẽ nối lại",
				"queue", c.queue, "sau", reconnectDelay.String(), "err", err)
		}

		select {
		case <-time.After(reconnectDelay):
		case <-ctx.Done():
			return nil
		}
	}
}

// session dựng một kết nối, consume tới khi hỏng hoặc tới khi ctx bị hủy.
func (c *Consumer) session(ctx context.Context, h Handler) error {
	conn, err := amqp.DialConfig(c.url, amqp.Config{
		Heartbeat: heartbeatInterval,
		Locale:    "en_US",
		Dial:      amqp.DefaultDial(dialTimeout),
	})
	if err != nil {
		return fmt.Errorf("nối rabbitmq: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("mở kênh consume: %w", err)
	}
	defer func() { _ = ch.Close() }()

	// Khai báo topology ở đây nữa, y như Publisher: consume một queue chưa tồn
	// tại làm broker đóng kênh, nên thứ tự khởi động của các tiến trình sẽ
	// quyết định hệ thống chạy hay không. Declare idempotent.
	if err := Declare(ch); err != nil {
		return err
	}

	// prefetchCount cho từng consumer (global=false). Xem comment ở hằng số.
	if err := ch.Qos(prefetchCount, 0, false); err != nil {
		return fmt.Errorf("đặt qos: %w", err)
	}

	// Kênh RIÊNG để đẩy message sang retry/DLQ.
	//
	// Không dùng chung kênh với consume vì kênh publish phải bật chế độ confirm,
	// mà Confirm(true) trên kênh đang consume làm mọi ack của consumer lẫn lộn
	// với confirm của publish trong cùng một dòng khung tin — rất khó lần khi
	// hỏng. Hai kênh trên cùng một kết nối TCP thì gần như không tốn gì.
	pub, err := c.newRepublishChannel(conn)
	if err != nil {
		return err
	}
	defer func() { _ = pub.ch.Close() }()

	// autoAck = false. Đây là dòng làm cho cả cơ chế này có nghĩa: với autoAck
	// thì broker coi message đã xong ngay khi đẩy đi, nên worker chết giữa lúc
	// xử lý sẽ làm mất sự kiện mà không ai biết.
	deliveries, err := ch.Consume(c.queue, consumerTag(c.queue), false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("đăng ký consumer trên queue %q: %w", c.queue, err)
	}
	c.log.Info("đã đăng ký consumer", "queue", c.queue, "prefetch", prefetchCount)

	closed := conn.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-ctx.Done():
			return c.drain(ctx, ch, pub, h, deliveries)

		case amqpErr := <-closed:
			return fmt.Errorf("mất kết nối rabbitmq: %w", amqpErr)

		case d, ok := <-deliveries:
			if !ok {
				// Broker hủy consumer từ phía nó (queue bị xóa, node chuyển).
				return errors.New("broker đã hủy consumer")
			}
			if err := c.handle(ctx, pub, h, d); err != nil {
				// Lỗi tới được đây là lỗi của chính hạ tầng ack/publish, tức
				// kênh đã hỏng. Bỏ cả phiên để nối lại: mọi message chưa ack sẽ
				// được broker giao lại, không mất cái nào.
				return err
			}
		}
	}
}

// drain ngừng nhận message mới rồi xử lý nốt những message đã nhận.
//
// Đây là phần "dừng êm" thật sự, và nó phải theo ĐÚNG thứ tự này:
//
//  1. Cancel: broker ngừng đẩy message mới cho consumer này. Những message đã
//     prefetch mà thư viện chưa trao cho ta thì broker tự giao lại cho bản
//     khác — không mất.
//  2. Đọc cho tới khi kênh delivery đóng: thư viện đóng nó sau khi trao hết
//     phần đã nhận. Bỏ bước này mà thoát luôn thì những message đang nằm trong
//     bộ đệm bị bỏ dở — chúng không mất (chưa ack), nhưng mỗi lần deploy lại
//     sinh ra một đợt giao lại không cần thiết.
func (c *Consumer) drain(ctx context.Context, ch *amqp.Channel, pub *republisher, h Handler, deliveries <-chan amqp.Delivery) error {
	c.log.Info("nhận tín hiệu tắt, ngừng nhận message mới", "queue", c.queue)
	if err := ch.Cancel(consumerTag(c.queue), false); err != nil {
		// Hủy không được thì kênh đã hỏng; message chưa ack sẽ được giao lại.
		return fmt.Errorf("hủy consumer: %w", err)
	}
	for d := range deliveries {
		if err := c.handle(ctx, pub, h, d); err != nil {
			return err
		}
	}
	c.log.Info("đã xử lý xong message đang dở", "queue", c.queue)
	return nil
}

// handle quyết định số phận của MỘT message.
//
// Trả về lỗi CHỈ khi chính hạ tầng hỏng (ack không được, đẩy sang retry/DLQ
// không được) — lúc đó cả phiên phải bỏ đi. Lỗi của handler thì không bao giờ
// rơi ra ngoài đây: nó đã được dịch thành retry hoặc DLQ.
func (c *Consumer) handle(ctx context.Context, pub *republisher, h Handler, d amqp.Delivery) error {
	// WithoutCancel: tín hiệu tắt KHÔNG được cắt ngang một message đang xử lý.
	// Cắt giữa chừng thì transaction của handler rollback và message sẽ được
	// giao lại sau khi khởi động — biến mỗi lần deploy thành một đợt xử lý
	// trùng, trong khi tránh được hoàn toàn. Vẫn có trần handleTimeout nên
	// "đợi cho xong" luôn là hữu hạn.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), handleTimeout)
	defer cancel()

	traceID := headerString(d.Headers, "trace_id")
	attempts := attemptCount(d.Headers)

	// Đếm x-death TRƯỚC khi gọi handler. Gọi rồi mới đếm thì một message hỏng
	// vĩnh viễn vẫn tốn thêm một lần xử lý (và một lần ghi database) ở vòng
	// cuối, hoàn toàn vô ích.
	if attempts >= maxAttempts {
		c.log.Error("message đã thử quá số lần cho phép, đẩy sang DLQ",
			"queue", c.queue, "message_id", d.MessageId, "trace_id", traceID,
			"routing_key", d.RoutingKey, "attempts", attempts, "max_attempts", maxAttempts)
		return pub.republish(ctx, d, QueueCatalogIndexerDLQ, attempts)
	}

	err := h(ctx, Delivery{
		RoutingKey: d.RoutingKey,
		MessageID:  d.MessageId,
		TraceID:    traceID,
		Body:       d.Body,
		Attempts:   attempts,
	})

	switch {
	case err == nil:
		// multiple = false: chỉ ack đúng message này. true sẽ ack luôn mọi
		// message có delivery tag nhỏ hơn — tức ack cả những message đang chờ
		// xử lý trong bộ đệm prefetch, và chúng biến mất mà chưa ai đụng tới.
		return d.Ack(false)

	case errors.Is(err, ErrPermanent):
		c.log.Error("message hỏng vĩnh viễn, đẩy thẳng sang DLQ — KHÔNG thử lại",
			"queue", c.queue, "message_id", d.MessageId, "trace_id", traceID,
			"routing_key", d.RoutingKey, "err", err)
		return pub.republish(ctx, d, QueueCatalogIndexerDLQ, attempts)

	default:
		c.log.Warn("xử lý message thất bại, đẩy sang queue retry",
			"queue", c.queue, "message_id", d.MessageId, "trace_id", traceID,
			"routing_key", d.RoutingKey, "attempts", attempts, "err", err)
		// attempts+1: đây là lần thử thứ mấy khi message quay lại.
		return pub.republish(ctx, d, QueueCatalogIndexerRetry, attempts+1)
	}
}

// consumerTag đặt tên cho đăng ký consumer.
//
// Đặt tên cố định chứ không để thư viện sinh tag ngẫu nhiên, vì hai lý do:
// Cancel lúc dừng êm cần biết tag để hủy, và trên management UI thì một cái tên
// đọc được giúp biết ngay ai đang giữ message chưa ack.
func consumerTag(queue string) string { return queue + ".consumer" }

// headerString đọc một header dạng chuỗi, trả về rỗng nếu thiếu hoặc sai kiểu.
//
// Header do BÊN GỬI đặt, nên không được tin kiểu dữ liệu của nó. Một ép kiểu
// trần trụi ở đây sẽ làm panic cả worker chỉ vì ai đó publish thử một message
// bằng tay trên management UI với trace_id là số.
func headerString(h amqp.Table, key string) string {
	if h == nil {
		return ""
	}
	s, ok := h[key].(string)
	if !ok {
		return ""
	}
	return s
}

// headerRetryCount là bộ đếm số lần thử của CHÍNH CHÚNG TA.
//
// # Vì sao không dùng x-death của RabbitMQ
//
// Đây là một cái bẫy đã dựng lại được bằng thực nghiệm, và nó im lặng tuyệt đối.
//
// Tài liệu RabbitMQ nói x-death có trường count tăng lên mỗi lần message bị
// dead-letter qua cùng một (queue, reason) — nghe như đúng thứ ta cần. Nhưng nó
// chỉ tăng khi CHÍNH RabbitMQ dead-letter message đó. Ở đây consumer tự publish
// một BẢN SAO vào queue retry, nên mỗi vòng là một message mới: RabbitMQ
// dead-letter nó đúng một lần (lúc hết TTL) và đặt count = 1. Vòng sau lại một
// message mới, lại count = 1.
//
// Đo được: cho một message lỗi quay 5 vòng, x-death count đứng yên ở 1 suốt,
// và message KHÔNG BAO GIỜ tới DLQ — nó đi mãi giữa queue chính và queue retry,
// mỗi 30 giây một lần, không một dòng lỗi nào. Đúng thứ retry/DLQ sinh ra để
// tránh.
//
// Bộ đếm của chính mình thì nằm hoàn toàn trong tầm kiểm soát và không phụ
// thuộc vào ngữ nghĩa dead-letter của broker.
const headerRetryCount = "x-retry-count"

// attemptCount cho biết message đã được thử mấy lần.
//
// Lấy giá trị LỚN NHẤT giữa bộ đếm của ta và count trong x-death: x-death vẫn
// hữu ích cho những message tới đây bằng đường dead-letter thật (ví dụ bị
// reject ở một queue khác) mà chưa từng đi qua republish của ta.
func attemptCount(h amqp.Table) int {
	n := headerInt(h, headerRetryCount)
	if d := deathCount(h); d > n {
		n = d
	}
	return n
}

// headerInt đọc một header dạng số. Header do bên gửi đặt nên không tin kiểu
// dữ liệu của nó: ép kiểu trần trụi ở đây sẽ panic cả worker chỉ vì ai đó
// publish thử một message bằng tay trên management UI.
func headerInt(h amqp.Table, key string) int {
	if h == nil {
		return 0
	}
	switch v := h[key].(type) {
	case int64:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}

// deathCount đọc header x-death của RabbitMQ.
//
// x-death là một MẢNG CÁC BẢNG, mỗi phần tử ứng với một cặp (queue, reason) và
// có các khóa: count (int64), queue, reason ("expired" khi hết TTL), time,
// exchange, routing-keys.
//
// ⚠️ ĐỪNG dùng một mình để đếm số vòng retry — xem headerRetryCount ở trên.
func deathCount(h amqp.Table) int {
	if h == nil {
		return 0
	}
	entries, ok := h["x-death"].([]any)
	if !ok {
		return 0
	}
	maxCount := 0
	for _, raw := range entries {
		entry, ok := raw.(amqp.Table)
		if !ok {
			continue
		}
		// count là int64 khi đi qua dây, nhưng bắt thêm int32/int cho chắc:
		// ép kiểu sai ở đây sẽ âm thầm trả về 0 và message quay vòng vĩnh viễn.
		var n int
		switch v := entry["count"].(type) {
		case int64:
			n = int(v)
		case int32:
			n = int(v)
		case int:
			n = v
		default:
			continue
		}
		if n > maxCount {
			maxCount = n
		}
	}
	return maxCount
}

// republisher đẩy message sang một queue khác (retry hoặc DLQ) rồi ack bản gốc.
type republisher struct {
	ch      *amqp.Channel
	returns chan amqp.Return
	log     *slog.Logger
}

func (c *Consumer) newRepublishChannel(conn *amqp.Connection) (*republisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("mở kênh publish cho retry/DLQ: %w", err)
	}
	// Bật confirm TRƯỚC mọi publish. Không có nó thì Publish trả về thành công
	// ngay khi ghi được vào socket, và ta sẽ ack bản gốc cho một message chưa
	// hề tới broker — mất sự kiện, không lỗi nào.
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("bật publisher confirm cho kênh retry/DLQ: %w", err)
	}
	return &republisher{
		ch: ch,
		// Đăng ký nhận message bị trả về ngay khi dựng kênh, trước mọi publish.
		returns: ch.NotifyReturn(make(chan amqp.Return, 8)),
		log:     c.log,
	}, nil
}

// republish gửi message sang queue đích rồi ack bản gốc.
//
// # Thứ tự publish-rồi-mới-ack là bắt buộc
//
// Ack trước rồi publish là tự tay mở một khe mất dữ liệu: crash ở giữa thì
// message đã biến khỏi queue gốc mà chưa tới queue đích. Chiều ngược lại (chết
// sau publish, trước ack) chỉ tạo ra một bản trùng — và trùng thì đã có
// processed_events lo. Mất thì không cứu được.
//
// # Vì sao sao chép d.Headers thay vì dựng bảng mới
//
// Vì trace_id và x-death nằm trong đó. Dựng một amqp.Table mới cho "sạch" sẽ
// xóa mất dấu vết nối message với request HTTP đã sinh ra nó.
//
// Sao chép chứ không sửa tại chỗ: d.Headers thuộc về delivery gốc, và sửa nó
// là sửa dữ liệu mà thư viện vẫn đang giữ.
func (r *republisher) republish(ctx context.Context, d amqp.Delivery, queue string, attempt int) error {
	drainReturns(r.returns)

	headers := amqp.Table{}
	for k, v := range d.Headers {
		headers[k] = v
	}
	headers[headerRetryCount] = int32(attempt)

	// Exchange rỗng là default exchange: nó định tuyến thẳng tới queue trùng
	// tên với routing key, nên không cần binding nào. mandatory = true để một
	// queue đích không tồn tại bị TRẢ VỀ thay vì biến mất trong im lặng.
	conf, err := r.ch.PublishWithDeferredConfirmWithContext(ctx, "", queue, true, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  d.ContentType,
		MessageId:    d.MessageId,
		Timestamp:    d.Timestamp,
		Headers:      headers,
		Body:         d.Body,
	})
	if err != nil {
		return fmt.Errorf("đẩy message sang %q: %w", queue, err)
	}

	ok, err := conf.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("chờ confirm khi đẩy sang %q: %w", queue, err)
	}
	if !ok {
		return fmt.Errorf("broker nack khi đẩy sang %q — KHÔNG ack bản gốc", queue)
	}

	// Ack chỉ có nghĩa "broker đã nhận", không có nghĩa "có queue nào giữ nó".
	// Cùng lập luận (và cùng bảo đảm thứ tự khung tin) như ở Publisher.Publish:
	// mỗi lúc chỉ có một message đang bay vì vòng lặp consume xử lý tuần tự.
	if ret, returned := readReturn(r.returns); returned {
		return fmt.Errorf("không queue nào nhận %q (%d %s) — KHÔNG ack bản gốc",
			queue, ret.ReplyCode, ret.ReplyText)
	}

	return d.Ack(false)
}
