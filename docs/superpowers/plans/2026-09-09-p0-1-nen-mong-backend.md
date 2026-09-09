# P0.1 — Nền móng backend: Implementation Plan

**Goal:** Dựng khung kỹ thuật backend chạy được — server Chi có health check, config validate lúc khởi động, pool Postgres, TxManager cho hexagonal, log JSON, graceful shutdown, hạ tầng dev bằng Docker Compose và CI.

**Architecture:** Modular monolith + Hexagonal. Kế hoạch này chỉ dựng `internal/platform/*` (hạ tầng dùng chung) và `internal/server` — chưa có module nghiệp vụ nào. Chiều phụ thuộc `adapter → app → domain` được CI kiểm bằng máy.

**Tech Stack:** Go 1.24 · Chi v5 · pgx/v5 (pgxpool) · goose · log/slog · go-task · Docker Compose · GitHub Actions

**Tài liệu thiết kế:** [01-transaction-outbox](../../design/01-transaction-outbox.md) · [02-api-contract](../../design/02-api-contract.md) · [04-kiem-chung](../../design/04-kiem-chung.md) · [05-deployment](../../design/05-deployment.md)

---

## ⚠️ Dự án này KHÔNG dùng unit test

Quyết định của chủ dự án. Hệ quả cần biết rõ để làm việc cho đúng:

- Không có file `_test.go` nào. Không testify, không testcontainers.
- Lưới an toàn tự động duy nhất là **`task check`** = `build` + `vet` + `lint` + `arch`.
  Nó bắt lỗi biên dịch, lỗi cú pháp và vi phạm kiến trúc — **không** bắt lỗi logic.
- Mọi hành vi phải **kiểm chứng thủ công**. Mỗi task dưới đây có mục "Kiểm chứng"
  ghi rõ lệnh phải chạy và kết quả phải thấy. Không được bỏ qua mục đó — nó là
  thứ duy nhất còn lại chứng minh code chạy đúng.
- Khi sửa code cũ, **không có gì báo cho bạn biết đã làm hỏng chỗ khác**. Phải tự
  chạy lại phần kiểm chứng của các task liên quan.

Rủi ro đã chấp nhận: ghi tại [04-kiem-chung](../../design/04-kiem-chung.md).

---

## Cấu trúc file khi hoàn thành

```
base-ecommerce/
├── Taskfile.yml                                  ✅ T1
├── .gitignore .gitattributes .editorconfig       ✅ T1
├── .env.example                                  ✅ T2
├── deploy/compose.dev.yml                        ✅ T2
├── scripts/check-arch.sh                         ⬜ T11
├── .github/workflows/ci.yml                      ⬜ T11
└── apps/api/
    ├── go.mod                                    ✅ T1
    ├── .golangci.yml                             ⬜ T11
    ├── db/migrations/00001_extensions.sql        ✅ T3
    ├── cmd/api/main.go                           ⬜ T10
    └── internal/
        ├── platform/
        │   ├── errs/errs.go                      ✅ T4
        │   ├── httpx/{handler,json,problem}.go   ✅ T5
        │   ├── config/config.go                  ✅ T6
        │   ├── postgres/{dbtx,pool,tx,health}.go ⬜ T7
        │   ├── observability/{log,middleware}.go ⬜ T8
        │   └── health/health.go                  ⬜ T9
        └── server/router.go                      ⬜ T9
```

---

## Task 1–6 ✅ Đã hoàn thành

Chi tiết đầy đủ nằm trong lịch sử git; đây là bản tóm tắt để tra cứu.

| Task | Nội dung | Commit |
|---|---|---|
| 1 | Repo, nhánh `feat/p0-1-nen-mong-backend`, `.gitignore`, `.gitattributes`, `.editorconfig`, `Taskfile.yml`, `go.mod` (module `base-ecommerce/api`, `go 1.24`) | `7f23f9a`, `e7a4a22` |
| 2 | `deploy/compose.dev.yml` — Postgres 5432, Redis **6380**, RabbitMQ 5672/15672, tất cả bind `127.0.0.1`; `.env.example` | `77b0468`, `04bd37f` |
| 3 | `db/migrations/00001_extensions.sql` — `pg_trgm`, `btree_gin` | `80615af` |
| 4 | `internal/platform/errs` — `Kind`, `Error`, `New`/`Wrap`/`Validation`/`From`, `Is` so cả Code lẫn Kind | `62d0f83` |
| 5 | `internal/platform/httpx` — `Handler` trả error, `Wrap` với `committedWriter`, `JSON` trả error, `Decode`, `Problem` RFC 7807 | `02d5345`, `a88fac3` |
| 6 | `internal/platform/config` — `Load()` gom lỗi, `String()` che mật khẩu | `5f7c3c6` |

**Năm quyết định các task sau phải tôn trọng:**

1. **`go 1.24`** trong `go.mod`. Sau mỗi `go get`, kiểm `git diff apps/api/go.mod` —
   `go get` tự nâng directive nếu thư viện đòi bản cao hơn, mà CI pin 1.24.
2. **Migration luôn tạo bằng `task migrate-create`** (version timestamp). Tự đặt tên
   `00002_...` sẽ khiến goose panic khi hai nhánh trùng số.
3. **`errs` không được import `net/http`** — đó là điều kiện để `domain` import nó.
4. **Không nội suy lỗi gốc vào `Message`** — nó ra thẳng `title` của response, làm
   lộ tên bảng và tên ràng buộc.
5. **Handler luôn `return err`**, không tự ghi lỗi. `httpx.Wrap` lo phần còn lại.

---

## Task 7: `platform/postgres` — pool, DBTX, TxManager

**Files:** tạo `apps/api/internal/platform/postgres/{dbtx,pool,tx,health}.go`

- [ ] **Step 1: Cài pgx**

```bash
cd apps/api && go get github.com/jackc/pgx/v5@latest
git diff go.mod          # go directive phải vẫn là 1.24
```

- [ ] **Step 2: `dbtx.go`**

```go
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX là tập lệnh chung của *pgxpool.Pool và pgx.Tx.
//
// Repository nhận DBTX chứ không nhận *pgxpool.Pool, nhờ vậy cùng một repository
// chạy được cả trong lẫn ngoài transaction mà không phải viết hai bản.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

- [ ] **Step 3: `pool.go`**

```go
package postgres

import (
	"context"
	"fmt"
	"time"

	"base-ecommerce/api/internal/platform/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool tạo connection pool và ping ngay để lỗi cấu hình lộ ra lúc khởi động
// chứ không phải lúc có request đầu tiên.
//
// Lưu ý về kích thước pool: pool to hơn KHÔNG nhanh hơn. Mỗi kết nối Postgres là
// một process riêng; vượt quá số core của máy database thì thông lượng giảm.
// Tổng (pool × số bản chạy) của mọi tiến trình phải nhỏ hơn max_connections.
func NewPool(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("phân tích DSN: %w", err)
	}

	pc.MaxConns = cfg.MaxConns
	pc.MinConns = cfg.MinConns
	pc.MaxConnLifetime = cfg.MaxConnLifetime
	pc.MaxConnIdleTime = 30 * time.Minute
	pc.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("tạo pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
```

- [ ] **Step 4: `tx.go`**

```go
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
```

- [ ] **Step 5: `health.go`**

```go
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthChecker cài đặt health.Checker cho Postgres.
type HealthChecker struct{ pool *pgxpool.Pool }

func NewHealthChecker(pool *pgxpool.Pool) *HealthChecker { return &HealthChecker{pool: pool} }

func (c *HealthChecker) Name() string { return "postgres" }

func (c *HealthChecker) Check(ctx context.Context) error { return c.pool.Ping(ctx) }
```

- [ ] **Step 6: `task check`** — phải sạch cả bốn bước.

- [ ] **Step 7: KIỂM CHỨNG THỦ CÔNG — phần quan trọng nhất của task này**

`TxManager` là đoạn logic tinh vi nhất trong P0.1: transaction ngầm qua `context`,
retry, và rollback khi context đã hủy. Không kiểm chứng thì không có cách nào biết
nó đúng. Tạo file **tạm** `apps/api/cmd/scratch/main.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"base-ecommerce/api/internal/platform/config"
	"base-ecommerce/api/internal/platform/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	must(err)

	pool, err := postgres.NewPool(ctx, cfg.DB)
	must(err)
	defer pool.Close()

	m := postgres.NewManager(pool)

	_, err = pool.Exec(ctx, `DROP TABLE IF EXISTS tx_scratch`)
	must(err)
	_, err = pool.Exec(ctx, `CREATE TABLE tx_scratch (id INT PRIMARY KEY, note TEXT NOT NULL)`)
	must(err)

	// 1. Commit
	must(m.Run(ctx, func(ctx context.Context) error {
		_, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_scratch VALUES (1,'a')`)
		return err
	}))
	fmt.Println("1. sau commit, số dòng =", count(ctx, pool), "(phải là 1)")

	// 2. Lỗi thì rollback toàn bộ
	boom := errors.New("boom")
	err = m.Run(ctx, func(ctx context.Context) error {
		if _, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_scratch VALUES (2,'b')`); err != nil {
			return err
		}
		return boom
	})
	fmt.Println("2. lỗi trả về đúng boom:", errors.Is(err, boom), "(phải là true)")
	fmt.Println("   số dòng =", count(ctx, pool), "(vẫn phải là 1)")

	// 3. Trong transaction, DB(ctx) KHÔNG được là pool
	must(m.Run(ctx, func(ctx context.Context) error {
		_, isPool := m.DB(ctx).(*pgxpool.Pool)
		fmt.Println("3. trong tx, DB(ctx) là pool:", isPool, "(phải là false)")
		return nil
	}))

	// 4. Run lồng nhau dùng chung một transaction: lỗi ở ngoài hủy cả phần trong
	err = m.Run(ctx, func(ctx context.Context) error {
		if _, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_scratch VALUES (3,'ngoai')`); err != nil {
			return err
		}
		if err := m.Run(ctx, func(ctx context.Context) error {
			_, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_scratch VALUES (4,'trong')`)
			return err
		}); err != nil {
			return err
		}
		return boom
	})
	fmt.Println("4. lồng nhau rồi lỗi, số dòng =", count(ctx, pool), "(vẫn phải là 1)")

	// 5. Context đã hủy vẫn rollback được, không rò rỉ kết nối
	cctx, cancel := context.WithCancel(ctx)
	_ = m.Run(cctx, func(ctx context.Context) error {
		_, _ = m.DB(ctx).Exec(ctx, `INSERT INTO tx_scratch VALUES (5,'huy')`)
		cancel()
		return errors.New("client bỏ đi")
	})
	time.Sleep(200 * time.Millisecond)
	fmt.Println("5. sau khi ctx bị hủy, số dòng =", count(ctx, pool), "(vẫn phải là 1)")
	fmt.Println("   kết nối đang giữ =", pool.Stat().AcquiredConns(), "(phải là 0)")

	_, _ = pool.Exec(ctx, `DROP TABLE tx_scratch`)
	fmt.Println("\nXong. Mọi dòng khớp ghi chú trong ngoặc thì TxManager đúng.")
}

func count(ctx context.Context, pool *pgxpool.Pool) int {
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tx_scratch`).Scan(&n); err != nil {
		panic(err)
	}
	return n
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "LỖI:", err)
		os.Exit(1)
	}
}
```

Chạy:
```bash
task up
cd apps/api && go run ./cmd/scratch
```

Kết quả phải đúng từng dòng:
```
1. sau commit, số dòng = 1 (phải là 1)
2. lỗi trả về đúng boom: true (phải là true)
   số dòng = 1 (vẫn phải là 1)
3. trong tx, DB(ctx) là pool: false (phải là false)
4. lồng nhau rồi lỗi, số dòng = 1 (vẫn phải là 1)
5. sau khi ctx bị hủy, số dòng = 1 (vẫn phải là 1)
   kết nối đang giữ = 0 (phải là 0)
```

Dòng cuối quan trọng nhất: `AcquiredConns() == 0` chứng minh
`context.WithoutCancel` trong `defer tx.Rollback` đang làm đúng việc. Nếu nó lớn
hơn 0 thì đang rò rỉ kết nối — pool sẽ cạn dần và mọi request treo.

- [ ] **Step 8: Xóa file tạm rồi commit**

```bash
rm -rf apps/api/cmd/scratch
git add apps/api/internal/platform/postgres apps/api/go.mod apps/api/go.sum
git commit -m "feat(postgres): pool, DBTX và TxManager truyền transaction qua context

Cho phép tầng app mở transaction mà không import pgx. Kèm retry lỗi
tuần tự hóa và rollback an toàn khi context đã bị hủy.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `platform/observability` — log JSON và middleware

**Files:** tạo `apps/api/internal/platform/observability/{log,middleware}.go`

- [ ] **Step 1: `log.go`**

```go
// Package observability cung cấp log có cấu trúc và middleware quan sát.
package observability

import (
	"io"
	"log/slog"
	"strings"
)

// NewLogger tạo logger ghi JSON ra w.
//
// JSON ra stdout là yêu cầu của 12-factor: tiến trình không tự quản lý file log,
// hạ tầng thu gom. Nhờ vậy chuyển sang Kubernetes sau này không phải sửa gì.
func NewLogger(w io.Writer, level, env, version string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: ParseLevel(level)})
	return slog.New(h).With("env", env, "version", version)
}

func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

- [ ] **Step 2: `middleware.go`**

```go
package observability

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// RequestLogger ghi một dòng log cho mỗi request đã hoàn tất.
//
// Phải đặt SAU middleware.RequestID trong chuỗi middleware, nếu không
// request_id sẽ rỗng.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			log.LogAttrs(r.Context(), levelFor(ww.Status()), "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("ip", r.RemoteAddr),
			)
		})
	}
}

func levelFor(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
```

- [ ] **Step 3: `task check`** — phải sạch.

- [ ] **Step 4: Kiểm chứng** — thực hiện ở Task 10 khi đã có server chạy. Ở bước
này chỉ cần biên dịch được.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/platform/observability
git commit -m "feat(observability): log JSON và middleware ghi request

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Health check và router

**Files:** tạo `apps/api/internal/platform/health/health.go`, `apps/api/internal/server/router.go`

- [ ] **Step 1: `health.go`**

```go
// Package health cung cấp hai endpoint kiểm tra sức khỏe khác nhau.
//
// /healthz (liveness): tiến trình còn sống không. Hỏng thì phải khởi động lại.
// /readyz  (readiness): có sẵn sàng nhận request không (ping được DB, cache...).
//
// Hai cái này KHÁC nhau. Gộp lại thì Redis chập chờn sẽ khiến hạ tầng khởi động
// lại một API vốn vẫn phục vụ tốt.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// Checker là một phụ thuộc bên ngoài cần kiểm tra ở /readyz.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type Handler struct {
	version  string
	checkers []Checker
	down     atomic.Bool // bật lên khi bắt đầu tắt máy
}

func New(version string, checkers ...Checker) *Handler {
	return &Handler{version: version, checkers: checkers}
}

// Shutdown đánh dấu tiến trình đang tắt. Gọi ngay khi nhận SIGTERM, TRƯỚC khi
// đóng server, để proxy kịp ngừng gửi request mới tới.
func (h *Handler) Shutdown() { h.down.Store(true) }

func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	if h.down.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "shutting_down", "version": h.version,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "version": h.version,
	})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.down.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "shutting_down"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := make(map[string]string, len(h.checkers))
	status, code := "ok", http.StatusOK
	for _, c := range h.checkers {
		if err := c.Check(ctx); err != nil {
			checks[c.Name()] = "fail"
			status, code = "degraded", http.StatusServiceUnavailable
			continue
		}
		checks[c.Name()] = "ok"
	}

	writeJSON(w, code, map[string]any{
		"status": status, "version": h.version, "checks": checks,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 2: `router.go`**

```go
// Package server ráp các module vào router. Đây là nơi DUY NHẤT biết toàn bộ
// danh sách module của hệ thống.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"base-ecommerce/api/internal/platform/health"
	"base-ecommerce/api/internal/platform/observability"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// New dựng router với chuỗi middleware chuẩn.
//
// Thứ tự middleware quan trọng:
//   - RequestID trước RequestLogger, nếu không log sẽ không có request_id.
//   - Recoverer sau RequestLogger, để panic vẫn được ghi thành một dòng log request.
//   - Timeout cuối cùng, chỉ bao quanh handler nghiệp vụ.
func New(log *slog.Logger, h *health.Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(observability.RequestLogger(log))
	r.Use(middleware.Recoverer)

	// Health check nằm NGOÀI timeout: khi hệ thống quá tải, đây chính là lúc
	// cần chúng trả lời được nhất.
	r.Get("/healthz", h.Live)
	r.Get("/readyz", h.Ready)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		// Các module nghiệp vụ gắn vào đây từ kế hoạch P0.2 trở đi.
	})

	return r
}
```

- [ ] **Step 3: `task check`** — phải sạch.

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/platform/health apps/api/internal/server
git commit -m "feat(health): tách liveness và readiness, dựng router Chi

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: `cmd/api` — khởi động và graceful shutdown

**Files:** tạo `apps/api/cmd/api/main.go`

- [ ] **Step 1: `main.go`**

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"base-ecommerce/api/internal/platform/config"
	"base-ecommerce/api/internal/platform/health"
	"base-ecommerce/api/internal/platform/observability"
	"base-ecommerce/api/internal/platform/postgres"
	"base-ecommerce/api/internal/server"
)

// version được nhúng lúc build: -ldflags="-X main.version=$GIT_SHA"
var version = "dev"

func main() {
	// Subcommand healthcheck để Docker HEALTHCHECK gọi được — image distroless
	// không có curl hay wget.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

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

	// BẮT BUỘC. httpx.WriteError ghi log lỗi 5xx qua slog mặc định của package
	// (nó được gọi từ Wrap, không có chỗ nào truyền logger vào). Không đặt dòng
	// này thì đúng những dòng log quan trọng nhất — 5xx kèm nguyên nhân gốc —
	// sẽ ra stderr dạng text, không có env/version và bỏ qua LOG_LEVEL, trong
	// khi log request lại là JSON ra stdout.
	slog.SetDefault(log)

	log.Info("đang khởi động", "config", cfg.String())

	// Nhận tín hiệu tắt trước khi mở tài nguyên, để Ctrl+C lúc đang kết nối
	// database cũng thoát được.
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

	h := health.New(cfg.Version, postgres.NewHealthChecker(pool))

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           server.New(log, h),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server đang lắng nghe", "addr", cfg.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server dừng bất thường: %w", err)
	case <-ctx.Done():
		log.Info("nhận tín hiệu tắt, bắt đầu dừng êm")
	}

	// Bước 1: báo chưa sẵn sàng để proxy ngừng gửi request mới tới.
	h.Shutdown()

	// Bước 2: chờ proxy nhận ra. Bỏ bước này thì khách sẽ nhận 502 ngay giữa
	// lúc đang thanh toán.
	drain := 5 * time.Second
	if cfg.Env != "production" {
		drain = 0 // môi trường dev không có proxy, không cần chờ
	}
	time.Sleep(drain)

	// Bước 3: xử lý nốt request đang dở.
	shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("dừng server không sạch", "err", err)
	}

	// Bước 4: đóng tài nguyên theo chiều ngược lúc khởi tạo (defer pool.Close).
	log.Info("đã dừng")
	return nil
}

// healthcheck gọi /healthz của chính tiến trình đang chạy trong container.
func healthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "HTTP_ADDR không hợp lệ: %v\n", err)
		return 1
	}
	if host == "" {
		host = "127.0.0.1"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Get(fmt.Sprintf("http://%s:%s/healthz", host, port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck thất bại: %v\n", err)
		return 1
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck trả về %d\n", res.StatusCode)
		return 1
	}
	return 0
}
```

- [ ] **Step 2: `task check`** — phải sạch.

- [ ] **Step 3: KIỂM CHỨNG THỦ CÔNG — 8 mục, làm hết**

```bash
task up
task migrate
task run
```

Mở terminal thứ hai:

| # | Lệnh | Kết quả phải thấy |
|---|---|---|
| 1 | (nhìn terminal chạy `task run`) | Log là **JSON một dòng**, có `"env"` và `"version"`, và dòng `"server đang lắng nghe"` |
| 2 | `curl -i localhost:8080/healthz` | `200`, body `{"status":"ok","version":"dev"}` |
| 3 | `curl -i localhost:8080/readyz` | `200`, có `"checks":{"postgres":"ok"}` |
| 4 | `curl -i localhost:8080/khong-ton-tai` | `404`. Terminal server in một dòng JSON có `"status":404`, `"path"`, `"duration_ms"`, và `"request_id"` khác rỗng |
| 5 | `docker compose -f deploy/compose.dev.yml stop postgres` rồi `curl -i localhost:8080/readyz` | `503`, `"checks":{"postgres":"fail"}` |
| 6 | ngay sau đó `curl -i localhost:8080/healthz` | **Vẫn `200`** — điểm mấu chốt của việc tách hai endpoint. Nếu nó cũng 503 là đã cài sai |
| 7 | `docker compose -f deploy/compose.dev.yml start postgres`, rồi `Ctrl+C` ở terminal server | Log lần lượt `"nhận tín hiệu tắt, bắt đầu dừng êm"` rồi `"đã dừng"`, thoát mã 0 |
| 8 | `cd apps/api && DATABASE_URL= go run ./cmd/api; echo "exit=$?"` | In `khởi động thất bại: cấu hình không hợp lệ: thiếu biến môi trường bắt buộc DATABASE_URL` và `exit=1` |

- [ ] **Step 4: Commit**

```bash
git add apps/api/cmd
git commit -m "feat(api): entrypoint với graceful shutdown và subcommand healthcheck

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: CI và kiểm tra kiến trúc bằng máy

**Files:** tạo `scripts/check-arch.sh`, `apps/api/.golangci.yml`, `.github/workflows/ci.yml`

- [ ] **Step 1: `scripts/check-arch.sh`**

```bash
#!/usr/bin/env bash
# Kiểm tra chiều phụ thuộc của kiến trúc hexagonal bằng máy, không bằng tự giác.
# Quy tắc đầy đủ ở README mục 3.
#
# Dự án không có unit test, nên script này cùng golangci-lint là toàn bộ lưới an
# toàn tự động. Đừng làm yếu nó đi.
set -euo pipefail

cd "$(dirname "$0")/../apps/api"
fail=0

# 1. domain không được chạm hạ tầng.
#    Danh sách trắng: stdlib, platform/errs, google/uuid, shopspring/decimal.
for pkg in $(go list ./internal/*/domain/... 2>/dev/null || true); do
  if go list -deps "$pkg" | grep -Eq 'go-chi|jackc/pgx|redis|amqp|net/http$'; then
    echo "LỖI KIẾN TRÚC: $pkg import package hạ tầng"
    go list -deps "$pkg" | grep -E 'go-chi|jackc/pgx|redis|amqp|net/http$' | sed 's/^/    /'
    fail=1
  fi
done

# 2. app không được import net/http hay adapter.
for pkg in $(go list ./internal/*/app/... 2>/dev/null || true); do
  if go list -f '{{join .Imports "\n"}}' "$pkg" | grep -Eq 'net/http|/adapter/'; then
    echo "LỖI KIẾN TRÚC: $pkg import net/http hoặc adapter"
    fail=1
  fi
done

# 3. repository phải dùng DBTX, không được giữ pool trực tiếp.
if grep -rn 'pgxpool\.Pool' internal/*/adapter/ 2>/dev/null; then
  echo "LỖI KIẾN TRÚC: adapter giữ *pgxpool.Pool — phải nhận DBTX qua Manager.DB(ctx)"
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "Kiểm tra kiến trúc: OK"
fi
exit "$fail"
```

Chạy thử: `chmod +x scripts/check-arch.sh && task arch` → `Kiểm tra kiến trúc: OK`

- [ ] **Step 2: `apps/api/.golangci.yml`**

Không có unit test thì linter phải gánh nhiều hơn, nên bật rộng:

```yaml
version: "2"

linters:
  enable:
    - errcheck        # không được bỏ qua lỗi trả về
    - govet
    - ineffassign
    - staticcheck
    - unused
    - bodyclose       # response body phải được đóng
    - rowserrcheck    # rows.Err() phải được kiểm tra
    - sqlclosecheck
    - contextcheck    # context phải được truyền xuống
    - errorlint       # dùng errors.Is/As thay vì so sánh trực tiếp
    - noctx           # không gọi HTTP mà thiếu context
    - nilerr          # return nil trong khi err != nil
    - copyloopvar

  settings:
    errcheck:
      check-type-assertions: true

formatters:
  enable:
    - gofmt
    - goimports
```

Chạy `task lint` → `0 issues`. Có cảnh báo thì sửa trước khi đi tiếp.

- [ ] **Step 3: `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

env:
  GO_VERSION: '1.24'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: apps/api/go.sum
      - name: Build
        working-directory: apps/api
        run: go build ./...
      - name: Vet
        working-directory: apps/api
        run: go vet ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: apps/api/go.sum
      # Phải là v7 trở lên: action v6 cài golangci-lint v1, không đọc được
      # file cấu hình schema version "2".
      - uses: golangci/golangci-lint-action@v7
        with:
          version: v2.13.2      # pin cứng để CI và máy dev dùng cùng một bản
          working-directory: apps/api

  arch:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: apps/api/go.sum
      - run: bash scripts/check-arch.sh
```

- [ ] **Step 4: Chạy toàn bộ ở máy** — `task check`, cả bốn bước phải xanh.

- [ ] **Step 5: Commit**

```bash
git add scripts .github apps/api/.golangci.yml
git commit -m "ci: lint, kiểm tra kiến trúc và build tự động

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tiêu chí hoàn thành P0.1

```bash
task up && task migrate
task check                  # build + vet + lint + arch, tất cả xanh
task run
```

Rồi kiểm tra bằng tay — đây là phần thay cho test, **không được bỏ**:

| Kiểm tra | Kỳ vọng |
|---|---|
| `curl localhost:8080/healthz` | 200, có `version` |
| `curl localhost:8080/readyz` | 200, `checks.postgres = ok` |
| `curl localhost:8080/khong-co` | 404, server in log JSON kèm `request_id` |
| Dừng Postgres → `/readyz` | 503 |
| Dừng Postgres → `/healthz` | **vẫn 200** |
| `Ctrl+C` | dừng êm, thoát mã 0 |
| `DATABASE_URL= go run ./cmd/api` | báo lỗi rõ ràng, thoát mã 1 |
| Kịch bản TxManager ở Task 7 | mọi dòng khớp ghi chú, `AcquiredConns() == 0` |

---

## Kế hoạch tiếp theo

| | Nội dung | Phụ thuộc |
|---|---|---|
| **P0.2** | Module `catalog`: domain → app → pgstore → httpapi, OpenAPI, cache Redis | P0.1 |
| **P0.3** | Outbox + `outboxrelay` + `worker`, consumer idempotent | P0.1 |
| **P0.4** | Next.js: 2 trang thật gọi API, sinh type từ OpenAPI | P0.2 |

P0.2 và P0.3 làm song song được sau khi P0.1 xong.
