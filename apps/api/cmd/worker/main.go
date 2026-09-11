// Command worker đọc queue catalog.indexer và xử lý sự kiện sản phẩm.
//
// # Nó làm gì ở P0.3, và vì sao chỉ có thế
//
// Hiện tại handler chỉ GHI LOG. Đó không phải chỗ làm dở: giá trị của P0.3 là
// chứng minh cả đường đi hoạt động — transaction nghiệp vụ ghi outbox, relay
// đẩy sang broker, worker nhận được, khử trùng lặp đúng, message hỏng vào DLQ,
// dừng êm không mất message. P7 sẽ thay đúng một chỗ (bên trong transaction ở
// handle) bằng việc đánh chỉ mục Meilisearch, và không phải sửa gì quanh nó.
//
// # Giao hàng at-least-once là chuyện đã ĐO ĐƯỢC, không phải lý thuyết
//
// Phép thử ở Task 6: seed 200 sự kiện rồi giết relay hai lần giữa chừng, kết
// quả là 282 message trong queue cho 200 sự kiện — 82 bản trùng. Không có cách
// nào làm khác được (xem phần đầu cmd/outboxrelay), nên khử trùng lặp ở đây là
// BẮT BUỘC chứ không phải "nên có". Nó nằm ở outbox.MarkProcessed, trong CÙNG
// transaction với việc xử lý.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"base-ecommerce/api/internal/outbox"
	"base-ecommerce/api/internal/platform/config"
	"base-ecommerce/api/internal/platform/observability"
	"base-ecommerce/api/internal/platform/postgres"
	"base-ecommerce/api/internal/platform/rabbitmq"

	"github.com/google/uuid"
)

// version được nhúng lúc build: -ldflags="-X main.version=$GIT_SHA"
var version = "dev"

// consumerName là khóa đứng trước event_id trong bảng processed_events.
//
// Nó là danh tính của LOẠI consumer, không phải của tiến trình: chạy ba bản
// worker thì cả ba dùng chung tên này, nên một sự kiện chỉ được xử lý một lần
// trên toàn hệ thống chứ không phải một lần cho mỗi bản. Ngược lại, P7 thêm một
// consumer khác (ví dụ gửi mail) thì nó phải có TÊN KHÁC — dùng lại tên này sẽ
// khiến consumer tới sau thấy mọi sự kiện đều "đã xử lý" và bỏ qua sạch.
//
// Đặt dấu gạch ngang chứ không phải dấu chấm để không ai nhầm nó với tên queue.
const consumerName = "catalog-indexer"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "khởi động thất bại: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err // config sai thì chết ngay, không chạy tiếp
	}
	if cfg.Version == "dev" {
		cfg.Version = version
	}

	log := observability.NewLogger(os.Stdout, cfg.LogLevel, cfg.Env, cfg.Version)
	slog.SetDefault(log)
	log.Info("đang khởi động worker", "config", cfg.String())

	// Nhận tín hiệu tắt TRƯỚC khi mở tài nguyên, để Ctrl+C lúc đang kết nối
	// database cũng thoát được. Giống hệt cmd/api và cmd/outboxrelay.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(startCtx, cfg.DB)
	if err != nil {
		return fmt.Errorf("kết nối database: %w", err)
	}
	defer pool.Close()
	log.Info("đã kết nối database")

	txManager := postgres.NewManager(pool)
	w := &worker{
		tx:   txManager,
		repo: outbox.NewRepository(txManager),
		log:  log,
	}

	// NewConsumer không kết nối ngay: Consumer.Run vốn đã phải biết dựng lại
	// phiên sau sự cố, nên broker chưa lên lúc container khởi động chỉ làm
	// vòng lặp đó chạy thêm vài nhịp — không phải lý do để tiến trình chết.
	consumer := rabbitmq.NewConsumer(cfg.RabbitMQ, rabbitmq.QueueCatalogIndexer, log)

	if err := consumer.Run(ctx, w.handle); err != nil {
		return err
	}

	// Tới đây là đã hủy consumer, xử lý xong message đang dở và đóng kết nối
	// broker. Sau dòng này là defer: đóng pool database.
	log.Info("đã dừng êm")
	return nil
}

type worker struct {
	tx   *postgres.Manager
	repo *outbox.Repository
	log  *slog.Logger
}

// envelope là hình dạng message mà relay gửi. Phải khớp với envelope ở
// cmd/outboxrelay.
//
// Cố ý KHAI BÁO LẠI ở đây thay vì tách ra một package dùng chung. Hai bên đọc
// và ghi cùng một hợp đồng dây (wire contract), nhưng chúng tiến hóa độc lập:
// relay chạy bản mới thêm một trường thì worker bản cũ vẫn phải chạy được, và
// một struct dùng chung sẽ khuyến khích đúng cái ngược lại — đổi struct là đổi
// cả hai tiến trình cùng lúc, mà chúng không bao giờ deploy cùng lúc.
//
// Payload để nguyên json.RawMessage: worker P0.3 chỉ ghi nó ra log, còn P7 mới
// biết từng event_type có hình dạng gì. Giải mã sớm ở đây sẽ kéo kiểu dữ liệu
// của module catalog vào một tiến trình chưa cần biết tới nó.
type envelope struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	OccurredAt string          `json:"occurred_at"`
	Aggregate  aggregate       `json:"aggregate"`
	TraceID    string          `json:"trace_id,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}

type aggregate struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// handle xử lý một message.
//
// Trả về lỗi gói bằng rabbitmq.Permanent cho những hỏng hóc mà thử lại chắc
// chắn vô ích, lỗi thường cho những hỏng hóc tạm thời. Xem rabbitmq.ErrPermanent
// để biết vì sao mặc định là "thử lại".
func (w *worker) handle(ctx context.Context, d rabbitmq.Delivery) error {
	var ev envelope
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		// VĨNH VIỄN. Một body không giải mã được sẽ không tự lành sau 30 giây:
		// thử lại 5 lần chỉ tốn 5 lần xử lý và chiếm một khe prefetch của
		// message tốt trong hai phút rưỡi.
		return rabbitmq.Permanent(fmt.Errorf("giải mã envelope: %w", err))
	}

	// event_id là khóa khử trùng lặp. Thiếu hoặc sai định dạng thì KHÔNG có
	// cách nào xử lý message này an toàn — xử lý mà không ghi được dấu vết
	// nghĩa là mỗi bản trùng lại được xử lý thêm một lần nữa.
	eventID, err := uuid.Parse(ev.EventID)
	if err != nil {
		return rabbitmq.Permanent(fmt.Errorf("event_id %q không phải uuid: %w", ev.EventID, err))
	}

	// TOÀN BỘ việc xử lý nằm trong MỘT transaction cùng với MarkProcessed.
	//
	// ⚠️ Đây là điểm quan trọng nhất của file này. Tách MarkProcessed ra ngoài
	// transaction sẽ mở một khe mất dữ liệu theo cả hai chiều:
	//   - Đánh dấu trước rồi crash trước khi xử lý: bảng nói "đã xử lý", bản
	//     trùng sau đó bị bỏ qua, sự kiện MẤT VĨNH VIỄN và không có lỗi nào.
	//   - Xử lý trước rồi crash trước khi đánh dấu: xử lý lại lần nữa.
	// Cùng một transaction thì hai thứ đó cùng sống hoặc cùng chết.
	//
	// Bây giờ "xử lý nghiệp vụ" mới chỉ là một dòng log nên trông có vẻ thừa.
	// Cấu trúc này viết đúng ngay từ đầu để P7 nhét việc thật (đánh chỉ mục
	// Meilisearch) vào đúng chỗ đó, thay vì phải sửa lại cả luồng khi sự cố đã
	// xảy ra rồi.
	return w.tx.Run(ctx, func(ctx context.Context) error {
		fresh, err := w.repo.MarkProcessed(ctx, consumerName, eventID)
		if err != nil {
			// TẠM THỜI: database restart, hết kết nối, deadlock. Message sang
			// queue retry và tự quay lại sau 30 giây.
			return err
		}
		if !fresh {
			// KHÔNG PHẢI LỖI. Giao hàng at-least-once khiến bản trùng là
			// chuyện xảy ra hằng ngày; coi nó là lỗi sẽ làm log đầy báo động
			// giả và che mất lỗi thật. Ghi ở mức Debug để lúc cần vẫn lần được.
			w.log.Debug("sự kiện đã được xử lý trước đó, bỏ qua",
				"event_id", ev.EventID, "event_type", ev.EventType,
				"trace_id", ev.TraceID, "consumer", consumerName)
			return nil
		}

		// ==== CHỖ DÀNH CHO VIỆC THẬT (P7: đánh chỉ mục Meilisearch) ====
		//
		// Lưu ý cho người viết tiếp: mọi thao tác I/O ngoài database đặt ở đây
		// sẽ nằm trong transaction, và postgres.Manager.RunWith dặn đừng làm
		// vậy. Meilisearch không có transaction nên không thể "cùng commit" —
		// cách đúng lúc đó là ghi kết quả vào một bảng rồi để một tiến trình
		// khác đẩy đi, hoặc chấp nhận đánh chỉ mục trùng (Meilisearch ghi đè
		// theo id nên trùng là vô hại) và giữ MarkProcessed ở đây.
		w.log.Info("đã xử lý sự kiện",
			"event_id", ev.EventID,
			"event_type", ev.EventType,
			"trace_id", ev.TraceID,
			"aggregate_type", ev.Aggregate.Type,
			"aggregate_id", ev.Aggregate.ID,
			"occurred_at", ev.OccurredAt,
			"payload", string(ev.Payload),
			"attempts", d.Attempts)
		return nil
	})
}
