// Command outboxrelay đọc bảng outbox và đẩy sự kiện sang RabbitMQ.
//
// # Vì sao là một tiến trình riêng chứ không phải một goroutine trong cmd/api
//
// Hai lý do, cả hai đều có hậu quả không lộ ra lúc chạy thử:
//
//   - cmd/api chạy nhiều bản sau load balancer. Một goroutine relay trong mỗi
//     bản nghĩa là N relay cùng đọc outbox, và FOR UPDATE SKIP LOCKED sẽ cho
//     chúng "nhảy cóc" qua nhau — event #2 ra broker trước event #1. Không lỗi
//     nào báo, chỉ có consumer nhận sai thứ tự.
//   - Relay và API có hồ sơ vận hành ngược nhau: API cần khởi động nhanh và
//     chết nhanh, relay cần chạy liền mạch. Gộp vào nhau thì mỗi lần deploy API
//     (vài lần một ngày) lại cắt ngang một lô đang publish.
//
// Đổi lại, PHẢI chạy ĐÚNG MỘT bản relay — xem comment ở deploy/compose.dev.yml.
//
// # Giao hàng at-least-once, không phải exactly-once
//
// Không có cách nào làm khác: rabbitmq.Publish trả lỗi KHÔNG có nghĩa là message
// chưa tới broker (đã dựng lại được cả hai: message bị nack và message hết giờ
// chờ confirm đều đã nằm trong queue). Relay buộc phải thử lại, nên consumer
// BẮT BUỘC khử trùng lặp bằng event_id — đó là việc của outbox.MarkProcessed ở
// Task 7. Đừng đọc bất cứ dòng nào dưới đây như một lời hứa gửi đúng một lần.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
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

const (
	// pollInterval là nhịp hỏi database. 500ms là chỗ dung hòa: đủ nhanh để
	// người dùng không cảm thấy hệ thống "trễ", đủ chậm để một relay rảnh rỗi
	// chỉ tốn 2 truy vấn/giây thay vì nghìn truy vấn/giây.
	//
	// Đây KHÔNG phải độ trễ tối đa lúc dồn ứ: mỗi nhịp xử lý trọn một lô, nên
	// thông lượng khi có tồn đọng là batchSize mỗi nhịp.
	pollInterval = 500 * time.Millisecond

	// batchSize là số dòng khóa trong một transaction. Lớn hơn thì ít vòng lặp
	// hơn nhưng transaction giữ khóa lâu hơn, mà mỗi dòng còn phải chờ confirm
	// của broker — 100 dòng đã là cả giây giữ một kết nối Postgres.
	batchSize = 100

	// roundTimeout là trần cho MỘT vòng (fetch + publish cả lô + đánh dấu).
	// Không có nó thì một broker "sống nhưng không trả lời" sẽ giữ transaction
	// mở vô hạn, mà transaction mở vô hạn chặn autovacuum của cả database.
	roundTimeout = 60 * time.Second

	// retention là thời gian giữ lại dòng đã gửi trước khi dọn.
	retention = 7 * 24 * time.Hour

	// cleanupInterval là nhịp chạy job dọn.
	cleanupInterval = 24 * time.Hour

	// cleanupBatch là số dòng xóa mỗi lô. Xóa theo lô vì DELETE hàng loạt trên
	// bảng lớn giữ khóa rất lâu và làm phình WAL.
	cleanupBatch = 10000

	// cleanupPause là khoảng nghỉ giữa hai lô xóa, để job dọn không chiếm hết
	// I/O của database trong lúc nó vẫn đang phục vụ request thật.
	cleanupPause = 100 * time.Millisecond
)

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
	log.Info("đang khởi động outboxrelay", "config", cfg.String())

	// Nhận tín hiệu tắt TRƯỚC khi mở tài nguyên, để Ctrl+C lúc đang kết nối
	// database cũng thoát được. Giống hệt cmd/api.
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

	// NewPublisher KHÔNG trả lỗi khi broker chết — chỉ ghi WARN rồi nối lại ở
	// lần Publish sau. Với relay thì đó vẫn là hành vi đúng: broker chưa lên
	// lúc container khởi động là chuyện thường, và một relay chết vì lý do đó
	// sẽ nằm im trong khi outbox dồn ứ.
	pub := rabbitmq.NewPublisher(startCtx, cfg.RabbitMQ, log)
	defer func() { _ = pub.Close() }()

	txManager := postgres.NewManager(pool)
	r := &relay{
		tx:   txManager,
		repo: outbox.NewRepository(txManager),
		pub:  pub,
		log:  log,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.runCleanup(ctx)
	}()

	r.runPoll(ctx) // chỉ trả về khi ctx bị hủy VÀ vòng đang chạy đã xong

	wg.Wait()
	// Sau dòng này là các defer: đóng publisher rồi đóng pool, theo đúng chiều
	// ngược lúc khởi tạo.
	log.Info("đã dừng êm")
	return nil
}

type relay struct {
	tx   *postgres.Manager
	repo *outbox.Repository
	pub  *rabbitmq.Publisher
	log  *slog.Logger
}

// runPoll chạy vòng lặp đẩy sự kiện cho tới khi ctx bị hủy.
//
// Tín hiệu tắt chỉ được kiểm ở ĐẦU mỗi vòng. Một vòng đã bắt đầu thì chạy cho
// xong — xem publishBatch để biết vì sao cắt ngang giữa chừng lại tệ.
func (r *relay) runPoll(ctx context.Context) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info("nhận tín hiệu tắt, ngừng vòng lặp đẩy sự kiện")
			return
		case <-t.C:
		}

		if err := r.publishBatch(ctx); err != nil {
			// Lỗi tới được đây là lỗi DATABASE (fetch hỏng, đánh dấu hỏng,
			// transaction rollback) — lỗi publish đã được nuốt bên trong và
			// ghi vào last_error. Không thoát tiến trình: Postgres restart hay
			// failover là chuyện bình thường, còn một relay chết theo nó sẽ
			// nằm im cho tới khi có người khởi động lại, đúng lúc outbox đang
			// dồn ứ.
			r.log.Error("vòng đẩy sự kiện thất bại, sẽ thử lại ở nhịp sau", "err", err)
		}
	}
}

// publishBatch chạy một vòng: khóa một lô, publish từng dòng, đánh dấu.
//
// # Vì sao publish nằm TRONG transaction, dù postgres.Manager.RunWith dặn đừng
// làm I/O bên ngoài ở đó
//
// Đây là ngoại lệ có ý thức, không phải quên đọc tài liệu. Đưa publish ra ngoài
// transaction thì FOR UPDATE SKIP LOCKED mất tác dụng ngay khi fetch trả về —
// khóa nhả theo transaction, nên bản relay thứ hai (hoặc chính bản này ở nhịp
// sau, khi một lô chạy lâu hơn 500ms) sẽ lấy lại đúng lô đó và publish trùng.
// Cái giá phải trả là một kết nối Postgres bị giữ trong lúc chờ broker confirm;
// roundTimeout chặn trần cho chuyện đó, và relay chỉ cần vài kết nối.
//
// # Chết giữa chừng thì sao
//
// Tiến trình chết sau khi publish nhưng trước khi commit: transaction rollback,
// published_at không được ghi, vòng sau publish LẠI. Trùng lặp — đúng thiết kế,
// consumer khử trùng lặp bằng event_id. Chiều ngược lại (đánh dấu đã gửi cho
// message chưa hề tới broker) mới là chiều làm MẤT event, và nó không xảy ra
// được vì MarkPublished chỉ nhận id đã có confirm.
//
// # MarkFailed nằm trong CÙNG transaction với FetchUnpublished — chấp nhận được
//
// Hệ quả: transaction này rollback thì attempts và last_error của cả lô cũng
// mất theo, tức attempts đếm "số lần thất bại đã GHI ĐƯỢC", không phải "số lần
// đã thử". Chấp nhận, vì ba lẽ:
//
//   - attempts/last_error là dữ liệu CHẨN ĐOÁN, không phải dữ liệu nghiệp vụ.
//     Mất một lần đếm không làm mất sự kiện: published_at vẫn NULL nên vòng sau
//     vẫn lấy dòng đó ra thử lại. Không có đường nào đi từ "đếm thiếu" tới "mất
//     event".
//   - Giữ được attempts qua rollback đòi hỏi ghi ở một transaction RIÊNG, tức
//     mở thêm kết nối Postgres cho mỗi dòng lỗi — đúng vào lúc hệ thống đang
//     hỏng và kết nối là thứ khan hiếm nhất. Đổi một chỉ số chẩn đoán lấy nguy
//     cơ cạn pool giữa sự cố là đổi sai chiều.
//   - Rollback ở đây gần như luôn có nghĩa là database đang hỏng, mà database
//     hỏng thì relay không làm được gì hết — con số attempts lúc ấy là mối lo
//     nhỏ nhất.
//
// ⚠️ Nếu sau này attempts được dùng để RA QUYẾT ĐỊNH (bỏ qua dòng đã thử quá N
// lần, chuyển nó sang bảng chết), lựa chọn này phải xem lại: một bộ đếm thiếu
// sẽ khiến dòng hỏng quay vòng vĩnh viễn. Lúc đó hãy chuyển MarkFailed ra
// transaction riêng, đừng nới điều kiện cho khớp với bộ đếm sai.
func (r *relay) publishBatch(ctx context.Context) error {
	// WithoutCancel: tín hiệu tắt KHÔNG được cắt ngang một vòng đang chạy. Nếu
	// cắt, transaction rollback giữa chừng và mọi message đã publish trong lô
	// đó sẽ được gửi lại ở lần khởi động sau — biến mỗi lần deploy thành một
	// đợt message trùng, trong khi tránh được hoàn toàn. Vẫn có trần thời gian
	// (roundTimeout) nên "đợi vòng đang chạy xong" luôn là hữu hạn.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), roundTimeout)
	defer cancel()

	return r.tx.Run(ctx, func(ctx context.Context) error {
		recs, err := r.repo.FetchUnpublished(ctx, batchSize)
		if err != nil {
			return err
		}
		if len(recs) == 0 {
			return nil
		}

		// published CHỈ chứa id đã được broker xác nhận.
		//
		// ⚠️ Đây là chỗ dễ viết sai nhất của cả tiến trình: đánh dấu nguyên lô
		// khi mới chỉ một phần thành công sẽ khiến những dòng còn lại có
		// published_at mà chưa bao giờ ra khỏi máy — mất event VĨNH VIỄN, và
		// không có lỗi nào ở bất cứ đâu để lần ra. Đừng "đơn giản hóa" bằng
		// cách truyền thẳng id của cả recs vào MarkPublished.
		published := make([]uuid.UUID, 0, len(recs))

		for _, rec := range recs {
			body, err := buildEnvelope(rec)
			if err != nil {
				// Payload trong cột JSONB mà không ghép được vào envelope là
				// chuyện gần như không thể (Postgres đã chặn JSON hỏng từ lúc
				// Append). Vẫn xử lý như một lần thất bại thay vì return: một
				// dòng lạ không được phép chặn cả lô.
				if err := r.markFailed(ctx, rec, err); err != nil {
					return err
				}
				continue
			}

			if err := r.pub.Publish(ctx, rabbitmq.Message{
				// Routing key CHÍNH LÀ event_type. Nhờ vậy thêm sự kiện mới
				// không phải sửa relay; đổi lại, một event_type không khớp
				// binding nào sẽ làm Publish trả lỗi (broker trả message về)
				// và dòng đó thử lại mãi với attempts tăng dần. Đó là hành vi
				// MONG MUỐN: thà ồn ào còn hơn mất event trong im lặng.
				RoutingKey: rec.EventType,
				Body:       body,
				EventID:    rec.ID.String(),
				TraceID:    rec.TraceID,
			}); err != nil {
				if err := r.markFailed(ctx, rec, err); err != nil {
					return err
				}
				continue
			}
			published = append(published, rec.ID)
		}

		if err := r.repo.MarkPublished(ctx, published); err != nil {
			return err
		}
		r.log.Info("đã đẩy một lô sự kiện",
			"lay_ra", len(recs), "thanh_cong", len(published),
			"that_bai", len(recs)-len(published))
		return nil
	})
}

// markFailed ghi nhận một dòng publish hỏng rồi cho vòng lặp đi tiếp.
//
// Trả về lỗi CHỈ khi chính việc ghi nhận hỏng (database có vấn đề) — lúc đó cả
// transaction phải rollback, vì publish tiếp trong một transaction đã hỏng là
// gửi đi những message mà không có gì đánh dấu lại được.
func (r *relay) markFailed(ctx context.Context, rec outbox.Record, cause error) error {
	r.log.Warn("đẩy sự kiện thất bại, sẽ thử lại ở nhịp sau",
		"event_id", rec.ID, "event_type", rec.EventType,
		"attempts", rec.Attempts, "trace_id", rec.TraceID, "err", cause)
	return r.repo.MarkFailed(ctx, rec.ID, cause.Error())
}

// envelope là hình dạng message mà consumer nhận được.
//
// Thứ tự trường ở đây quyết định thứ tự khóa trong JSON gửi đi. Giữ đúng theo
// thiết kế 01 để message đọc bằng mắt trên management UI lúc sự cố còn dễ nhìn.
type envelope struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	// OccurredAt là ISO-8601 UTC kết thúc bằng Z, lấy từ outbox.created_at —
	// lúc transaction nghiệp vụ commit, KHÔNG phải lúc relay đọc được dòng.
	OccurredAt string    `json:"occurred_at"`
	Aggregate  aggregate `json:"aggregate"`
	// omitempty: trace_id rỗng (sự kiện sinh ngoài ngữ cảnh request, ví dụ job
	// nền) thì không gửi khóa này, thay vì gửi một chuỗi rỗng mà consumer lại
	// phải phân biệt với "có khóa nhưng không đọc được".
	TraceID string `json:"trace_id,omitempty"`
	// Payload là JSON THÔ lấy nguyên từ cột JSONB.
	//
	// ⚠️ Phải là json.RawMessage, không được là []byte hay string. []byte bị
	// encoding/json mã hóa base64, còn string thì thành một CHUỖI chứa JSON —
	// cả hai đều "chạy được", message vẫn đi tới nơi, và consumer nhận rác.
	Payload json.RawMessage `json:"payload"`
}

type aggregate struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func buildEnvelope(rec outbox.Record) ([]byte, error) {
	body, err := json.Marshal(envelope{
		EventID:   rec.ID.String(),
		EventType: rec.EventType,
		// UTC() trước khi format: RFC3339 của một mốc giờ địa phương kết thúc
		// bằng "+07:00" chứ không phải "Z". Cùng một khoảnh khắc, nhưng
		// consumer nào so sánh chuỗi (và sẽ có) thì thấy hai định dạng khác
		// nhau tùy múi giờ của máy chạy relay.
		OccurredAt: rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		Aggregate:  aggregate{Type: rec.AggregateType, ID: rec.AggregateID.String()},
		TraceID:    rec.TraceID,
		Payload:    rec.Payload,
	})
	if err != nil {
		return nil, fmt.Errorf("dựng envelope cho sự kiện %s: %w", rec.ID, err)
	}
	return body, nil
}

// runCleanup dọn dòng cũ: chạy ngay một lần rồi lặp lại mỗi cleanupInterval.
//
// # Vì sao chạy NGAY lúc khởi động chứ không đợi hết 24 giờ đầu
//
// Vì "đợi 24 giờ" trên thực tế nghĩa là KHÔNG BAO GIỜ CHẠY. Tiến trình này được
// khởi động lại mỗi lần deploy, mỗi lần đổi biến môi trường, mỗi lần node được
// thay — ở giai đoạn đầu của một sản phẩm thì khoảng cách giữa hai lần restart
// gần như luôn ngắn hơn 24 giờ. Một job dọn không bao giờ chạy là một job không
// tồn tại, và người ta chỉ phát hiện ra khi bảng outbox đã vài chục triệu dòng.
//
// Giá phải trả gần như bằng không: lần chạy lúc khởi động, nếu không có gì quá
// hạn, chỉ là hai câu DELETE dùng index partial và trả về 0 dòng. Kể cả khi
// container quay vòng liên tục (crash loop) thì mỗi vòng cũng chỉ thêm hai câu
// truy vấn rẻ — không có nguy cơ "dọn quá tay", vì ngưỡng là một mốc thời gian
// tuyệt đối chứ không phụ thuộc số lần chạy.
func (r *relay) runCleanup(ctx context.Context) {
	r.cleanupOnce(ctx)

	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.cleanupOnce(ctx)
		}
	}
}

func (r *relay) cleanupOnce(ctx context.Context) {
	before := time.Now().Add(-retention)
	start := time.Now()

	nOutbox := r.deleteInBatches(ctx, "outbox", func(ctx context.Context) (int64, error) {
		return r.repo.DeletePublishedBefore(ctx, before, cleanupBatch)
	})
	// processed_events dùng CÙNG ngưỡng, và ngưỡng đó phải lớn hơn nhiều thời
	// gian một message còn có thể quay lại (TTL retry 30 giây, thời gian nằm ở
	// DLQ). Bảy ngày thì thoải mái. Dọn sớm hơn thì dấu vết "đã xử lý" biến mất
	// trước bản sao cuối cùng và consumer xử lý lại lần nữa.
	nProcessed := r.deleteInBatches(ctx, "processed_events", func(ctx context.Context) (int64, error) {
		return r.repo.DeleteProcessedBefore(ctx, before, cleanupBatch)
	})

	r.log.Info("dọn dữ liệu cũ xong",
		"nguong", before.UTC().Format(time.RFC3339),
		"outbox_da_xoa", nOutbox, "processed_events_da_xoa", nProcessed,
		"mat", time.Since(start).String())
}

// deleteInBatches gọi del tới khi không còn gì để xóa, nghỉ giữa các lô.
func (r *relay) deleteInBatches(ctx context.Context, table string, del func(context.Context) (int64, error)) int64 {
	var total int64
	for {
		n, err := del(ctx)
		if err != nil {
			// Không thoát tiến trình: dọn thất bại chỉ làm bảng to thêm, còn
			// relay chết thì event không được gửi. Lần sau dọn tiếp.
			r.log.Warn("dọn dữ liệu cũ thất bại", "bang", table, "da_xoa", total, "err", err)
			return total
		}
		total += n
		if n == 0 {
			return total
		}
		// Nghỉ giữa hai lô để job dọn không chiếm hết I/O của database.
		select {
		case <-time.After(cleanupPause):
		case <-ctx.Done():
			return total
		}
	}
}
