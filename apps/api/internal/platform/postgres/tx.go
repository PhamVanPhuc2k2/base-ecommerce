package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

// TxOptions điều chỉnh hành vi của một transaction.
type TxOptions struct {
	// Serializable bật mức cô lập cao nhất. Chỉ dùng khi thật sự cần —
	// đổi lại là tỉ lệ phải thử lại cao hơn.
	Serializable bool
	// MaxRetries số lần thử lại khi gặp lỗi tuần tự hóa. 0 = mặc định 3.
	MaxRetries int
}

func (o TxOptions) isoLevel() pgx.TxIsoLevel {
	if o.Serializable {
		return pgx.Serializable
	}
	return pgx.ReadCommitted
}

func (o TxOptions) maxRetries() int {
	if o.MaxRetries <= 0 {
		return 3
	}
	return o.MaxRetries
}

// Manager sở hữu pool và cung cấp transaction cho tầng app.
//
// Manager cài đặt interface TxManager mà tầng app khai báo, nhờ vậy app mở được
// transaction mà không cần biết pgx là gì.
type Manager struct{ pool *pgxpool.Pool }

func NewManager(pool *pgxpool.Pool) *Manager { return &Manager{pool: pool} }

// DB trả về transaction đang gắn với ctx nếu có, ngược lại trả về pool.
//
// MỌI repository phải lấy kết nối qua hàm này. Repository giữ *pgxpool.Pool
// trực tiếp sẽ chạy ngoài transaction mà không ai nhận ra.
func (m *Manager) DB(ctx context.Context) DBTX {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return m.pool
}

// InTx cho biết ctx có đang mang một transaction không.
//
// Có hàm này để những câu lệnh chỉ đúng khi ở trong transaction tự chặn được
// mình. Ví dụ thật: SELECT ... FOR UPDATE SKIP LOCKED chạy ngoài transaction
// vẫn trả về dữ liệu và không lỗi gì — nhưng khóa nhả ngay khi câu lệnh kết
// thúc, nên SKIP LOCKED mất tác dụng và hai tiến trình sẽ xử lý trùng nhau.
// Hỏng kiểu đó không có triệu chứng cho tới khi chạy hai bản cùng lúc.
func (m *Manager) InTx(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(pgx.Tx)
	return ok
}

// Run chạy fn trong một transaction với tùy chọn mặc định.
func (m *Manager) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.RunWith(ctx, TxOptions{}, fn)
}

// RunWith chạy fn trong một transaction.
//
// Quy tắc khi dùng:
//   - Chỉ tầng app được gọi. Không gọi trong handler, không gọi trong repository.
//   - Không làm I/O bên ngoài trong fn (HTTP, RabbitMQ, gửi mail) — transaction
//     đang giữ một kết nối Postgres, mà kết nối là tài nguyên khan hiếm.
//   - fn phải chạy lại được, vì có thể bị thử lại khi gặp lỗi tuần tự hóa.
func (m *Manager) RunWith(ctx context.Context, opt TxOptions, fn func(ctx context.Context) error) error {
	// Đã ở trong transaction thì tái sử dụng, không mở transaction lồng nhau.
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	var lastErr error
	for attempt := 0; attempt < opt.maxRetries(); attempt++ {
		if attempt > 0 {
			// Backoff có nhiễu ngẫu nhiên để nhiều tiến trình không cùng thử lại một lúc.
			delay := time.Duration(1<<attempt)*10*time.Millisecond +
				time.Duration(rand.Intn(10))*time.Millisecond
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		lastErr = m.once(ctx, opt, fn)
		if lastErr == nil {
			return nil
		}
		if !IsRetryable(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("transaction thất bại sau %d lần thử: %w", opt.maxRetries(), lastErr)
}

func (m *Manager) once(ctx context.Context, opt TxOptions, fn func(context.Context) error) error {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: opt.isoLevel()})
	if err != nil {
		return fmt.Errorf("mở transaction: %w", err)
	}

	// Rollback là no-op nếu đã commit. Dùng defer để kết nối được trả lại pool
	// ngay cả khi fn panic.
	//
	// WithoutCancel là bắt buộc: nếu client ngắt kết nối thì ctx đã bị hủy và
	// Rollback(ctx) sẽ không chạy được, làm kết nối treo tới khi pool thu hồi.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IsRetryable cho biết lỗi có phải loại nên thử lại cả transaction hay không.
//
// 40001 và 40P01 là hành vi bình thường của Postgres khi có tranh chấp, không
// phải bug. Cách xử lý đúng là chạy lại nguyên transaction.
func IsRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Code {
	case "40001", "40P01":
		return true
	default:
		return false
	}
}
