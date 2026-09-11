package rabbitmq

import (
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Tên exchange, queue và binding của hệ thống.
//
// Chúng nằm ở đây dưới dạng hằng chuỗi chứ không phải ở module nghiệp vụ, vì
// topology phải được khai báo bởi MỌI tiến trình nối vào broker (API, relay,
// worker) — kể cả tiến trình không đọc queue đó. Package này vẫn không import
// internal/catalog: "catalog.indexer" chỉ là một cái tên, không phải một kiểu
// dữ liệu của module.
const (
	// ExchangeEvents là exchange topic dùng chung cho sự kiện của mọi module.
	// Kiểu topic (không phải direct) để consumer sau này đăng ký theo mẫu
	// "product.*" hay "order.#" mà không phải sửa bên publish.
	ExchangeEvents = "ecommerce.events"

	// QueueCatalogIndexer là queue chính của worker đánh chỉ mục sản phẩm.
	QueueCatalogIndexer = "catalog.indexer"

	// QueueCatalogIndexerRetry giữ message chờ thử lại. KHÔNG AI CONSUME queue
	// này — xem giải thích ở Declare.
	QueueCatalogIndexerRetry = "catalog.indexer.retry"

	// QueueCatalogIndexerDLQ là điểm cuối của message đã thử lại đủ số lần.
	// Có message ở đây nghĩa là cần người vào xem, nên hãy gắn cảnh báo.
	QueueCatalogIndexerDLQ = "catalog.indexer.dlq"

	// bindingCatalogIndexer: worker đánh chỉ mục quan tâm mọi sự kiện sản phẩm.
	bindingCatalogIndexer = "product.*"

	// retryTTLMillis là thời gian message nằm trong queue retry trước khi tự
	// quay về queue chính. 30 giây: đủ lâu để một sự cố chớp nhoáng (Postgres
	// failover, deploy) kết thúc, đủ ngắn để không ai phải chờ dữ liệu.
	retryTTLMillis = 30000
)

// Declare tạo toàn bộ exchange, queue và binding mà hệ thống cần.
//
// Vì sao khai báo bằng code chứ không bấm tay trên management UI: hạ tầng dựng
// lại từ đầu (máy mới, CI, môi trường staging mới) phải chạy được ngay, và
// không ai phải nhớ mình đã bấm những gì. Hàm này idempotent — gọi lại bao
// nhiêu lần cũng được, AMQP coi việc khai báo lại với CÙNG tham số là no-op.
//
// # Cơ chế retry: queue retry KHÔNG có consumer
//
// Đây là điểm dễ hiểu nhầm nhất. catalog.indexer.retry không phải queue để ai
// đó đọc. Worker xử lý thất bại sẽ publish message sang đây; message nằm im
// cho tới khi hết x-message-ttl, rồi RabbitMQ TỰ đẩy nó đi theo
// x-dead-letter-exchange ("" = default exchange) với routing key
// x-dead-letter-routing-key ("catalog.indexer" = tên queue chính). Nghĩa là
// message tự quay về queue chính sau 30 giây mà không cần bất kỳ đoạn code nào
// hẹn giờ. Thấy queue này có message tồn đọng là BÌNH THƯỜNG, không phải dấu
// hiệu worker chết.
//
// ⚠️ x-message-ttl của queue hết hạn theo thứ tự ĐẦU HÀNG, không theo từng
// message. RabbitMQ chỉ nhìn message đầu queue; message phía sau dù hết hạn
// trước cũng phải chờ nó đi. Với TTL cố định 30 giây cho mọi message thì thứ
// tự hết hạn trùng thứ tự vào queue nên không sao. ĐỪNG đặt TTL riêng cho từng
// message (trường Expiration) trên queue này: một message TTL 10 phút nằm đầu
// hàng sẽ chặn toàn bộ message TTL 30 giây phía sau suốt 10 phút. Muốn nhiều
// mốc chờ khác nhau thì tạo nhiều queue retry, mỗi queue một TTL.
//
// ⚠️ Đổi tham số của queue ĐÃ TỒN TẠI không tự cập nhật mà làm QueueDeclare
// lỗi 406 PRECONDITION_FAILED. Khi phát triển: xóa queue trên management UI
// (http://localhost:15672) rồi chạy lại. Khi production: KHÔNG được xóa queue
// đang có message — phải tạo queue tên mới (ví dụ catalog.indexer.v2), cho
// worker đọc cả hai, rồi bỏ queue cũ khi nó cạn.
func Declare(ch *amqp.Channel) error {
	// durable: true để exchange sống sót qua lần restart broker. autoDelete,
	// internal, noWait đều false — exchange phải tồn tại cả khi chưa ai bind.
	if err := ch.ExchangeDeclare(ExchangeEvents, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("khai báo exchange %q: %w", ExchangeEvents, hintPreconditionFailed(err))
	}

	// Queue chính. durable: true để queue (và message persistent trong đó)
	// sống sót qua restart broker.
	if _, err := ch.QueueDeclare(QueueCatalogIndexer, true, false, false, false, nil); err != nil {
		return fmt.Errorf("khai báo queue %q: %w", QueueCatalogIndexer, hintPreconditionFailed(err))
	}
	if err := ch.QueueBind(QueueCatalogIndexer, bindingCatalogIndexer, ExchangeEvents, false, nil); err != nil {
		return fmt.Errorf("bind %q vào %q với khóa %q: %w",
			QueueCatalogIndexer, ExchangeEvents, bindingCatalogIndexer, hintPreconditionFailed(err))
	}

	// Queue retry: hết TTL thì message tự chết và được đẩy về queue chính.
	// x-dead-letter-exchange rỗng là default exchange — nó định tuyến thẳng
	// tới queue trùng tên với routing key, nên không cần binding nào cả.
	if _, err := ch.QueueDeclare(QueueCatalogIndexerRetry, true, false, false, false, amqp.Table{
		"x-message-ttl":             int32(retryTTLMillis),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": QueueCatalogIndexer,
	}); err != nil {
		return fmt.Errorf("khai báo queue %q: %w", QueueCatalogIndexerRetry, hintPreconditionFailed(err))
	}

	// DLQ: không TTL, không dead-letter. Message vào đây là nằm lại cho người
	// xem — mất dữ liệu vì hết hạn ở bước cuối cùng thì còn tệ hơn là tồn đọng.
	if _, err := ch.QueueDeclare(QueueCatalogIndexerDLQ, true, false, false, false, nil); err != nil {
		return fmt.Errorf("khai báo queue %q: %w", QueueCatalogIndexerDLQ, hintPreconditionFailed(err))
	}

	return nil
}

// hintPreconditionFailed gắn hướng dẫn xử lý vào lỗi 406.
//
// Lỗi gốc của broker chỉ nói "inequivalent arg ..." và người gặp nó lần đầu
// thường tưởng code sai, rồi đi sửa code cho khớp queue cũ — tức là để cấu hình
// bấm tay trên UI quyết định cấu hình trong repo. Nói thẳng cách xử lý đúng
// ngay trong thông báo lỗi rẻ hơn nhiều so với một buổi chiều mò mẫm.
func hintPreconditionFailed(err error) error {
	var ae *amqp.Error
	if !errors.As(err, &ae) || ae.Code != amqp.PreconditionFailed {
		return err
	}
	return fmt.Errorf("%w — queue/exchange đã tồn tại với tham số KHÁC; "+
		"AMQP không tự cập nhật. Khi dev: xóa nó trên http://localhost:15672 rồi chạy lại. "+
		"Khi production: tạo tên mới rồi chuyển dần, KHÔNG xóa queue đang có message", err)
}
