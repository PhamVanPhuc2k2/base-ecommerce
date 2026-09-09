# P0.1 — Nền móng backend: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dựng khung kỹ thuật backend chạy được — server Chi có health check, config validate lúc khởi động, pool Postgres, TxManager cho hexagonal, log JSON, graceful shutdown, hạ tầng dev bằng Docker Compose và CI đầy đủ.

**Architecture:** Modular monolith + Hexagonal. Kế hoạch này chỉ dựng `internal/platform/*` (hạ tầng dùng chung) và `internal/server` — chưa có module nghiệp vụ nào. Chiều phụ thuộc `adapter → app → domain` được CI kiểm bằng máy ngay từ đầu.

**Tech Stack:** Go 1.24 · Chi v5 · pgx/v5 (pgxpool) · goose · testcontainers-go · testify · log/slog · go-task · Docker Compose · GitHub Actions

**Tài liệu thiết kế liên quan:** [01-transaction-outbox](../../design/01-transaction-outbox.md) · [02-api-contract](../../design/02-api-contract.md) · [04-testing](../../design/04-testing.md) · [05-deployment](../../design/05-deployment.md)

## Ngoài phạm vi kế hoạch này

Những hạng mục sau nằm trong checklist P0 của README nhưng **không** thuộc P0.1,
vì chúng cần thành phần chưa có ở bước này:

| Hạng mục | Thuộc kế hoạch |
|---|---|
| `platform/redis`, cache, rate limit, CORS | P0.2 (cần Redis) |
| `platform/rabbitmq`, outbox, worker, relay | P0.3 |
| Sentry | P0.3 (gắn cùng lúc với xử lý lỗi bất đồng bộ) |
| Dockerfile, compose.prod, deploy | Sau P0.4 — chưa có gì để đóng gói cho tới khi có nghiệp vụ |
| OpenAPI, sqlc, sinh client TS | P0.2 |

Kết thúc P0.1, hệ thống chưa phục vụ nghiệp vụ nào — đó là chủ đích. Nó chứng minh
khung kỹ thuật đứng vững trước khi đổ nghiệp vụ lên trên.

---

## Lưu ý về công cụ chạy lệnh

README ban đầu ghi `Makefile`. Máy phát triển là Windows, nơi `make` không có sẵn.
Kế hoạch này dùng **[go-task](https://taskfile.dev)** (`Taskfile.yml`) thay thế:
cài bằng `go install`, chạy giống nhau trên Windows/macOS/Linux, không cần thêm
công cụ hệ thống nào. Task 1 cập nhật lại README cho khớp.

---

## Cấu trúc file sau khi hoàn thành

```
base-ecommerce/
├── Taskfile.yml                                  # T1
├── .gitignore  .editorconfig  .env.example       # T1, T2
├── deploy/compose.dev.yml                        # T2
├── .github/workflows/ci.yml                      # T13
├── scripts/check-arch.sh                         # T13
└── apps/api/
    ├── go.mod  .golangci.yml                     # T1, T13
    ├── db/migrations/00001_extensions.sql        # T3
    ├── cmd/api/main.go                           # T12
    └── internal/
        ├── platform/
        │   ├── errs/errs.go                      # T4  — kernel, domain được import
        │   ├── httpx/{handler,json,problem}.go   # T5
        │   ├── config/config.go                  # T6
        │   ├── testdb/testdb.go                  # T7
        │   ├── postgres/{pool,dbtx}.go           # T8
        │   ├── postgres/tx.go                    # T9
        │   ├── observability/{log,middleware}.go # T10
        │   └── health/health.go                  # T11
        └── server/router.go                      # T11
```

Mỗi file một trách nhiệm. `errs` không import `net/http` — đó là điều kiện để
`domain` được phép import nó mà không phá vỡ quy tắc 3.1.

---

## Task 1: Khởi tạo repo và Taskfile

**Files:**
- Create: `.gitignore`, `.editorconfig`, `Taskfile.yml`, `apps/api/go.mod`
- Modify: `README.md`

- [ ] **Step 1: Khởi tạo git và Go module**

```bash
cd /d/Projects/base-ecommerce
git init
git checkout -b feat/p0-1-nen-mong-backend      # không commit thẳng lên main
mkdir -p apps/api/cmd/api apps/api/internal/platform apps/api/db/migrations scripts .github/workflows
cd apps/api
go mod init base-ecommerce/api
go mod edit -go=1.24                            # xem giải thích bên dưới
cd ../..
```

Module path `base-ecommerce/api` là đường dẫn cục bộ, không phải URL. Hợp lệ với Go
vì repo này không được `go get` từ nơi khác. Đổi sang `github.com/<org>/...` sau khi
có remote nếu muốn.

⚠️ **Phải chỉnh `go` directive.** `go mod init` ghi phiên bản của toolchain đang cài
trên máy, kèm cả số patch (ví dụ `go 1.26.4`). Directive này là **mức tối thiểu**,
nên CI pin Go 1.24 (Task 13) sẽ hỏng ngay với `go.mod requires go >= 1.26.4`. Ngoài
ra pin tới số patch buộc mọi người phải dùng đúng bản đó mà không có lý do. Đặt
`go 1.24` — máy bạn dùng toolchain mới hơn vẫn build được bình thường.

⚠️ **Và kiểm lại sau mỗi lần `go get`.** `go get` sẽ tự nâng `go` directive nếu một
thư viện khai mức tối thiểu cao hơn. Điều này rất dễ xảy ra ở Task 4, 5 và 7
(testify, chi, pgx, testcontainers). Sau mỗi lần cài thư viện, chạy
`git diff apps/api/go.mod` — nếu directive bị nâng lên quá `1.24` thì hoặc hạ lại,
hoặc nâng `GO_VERSION` trong CI ở Task 13 cho khớp. Hai chỗ này phải luôn đi cùng nhau.

- [ ] **Step 2: Tạo `.gitignore`**

```gitignore
# Go
/apps/api/bin/
/apps/api/api
*.exe
*.log
coverage.out
go.work
go.work.sum
__debug_bin*

# Node
node_modules/
.next/
out/
dist/
.turbo/

# Env & secrets
.env
.env.*
!.env.example
.vercel/
*.enc.yaml.dec

# IDE / OS
.idea/
.vscode/
.DS_Store
Thumbs.db
```

Mẫu `.env.*` kèm ngoại lệ `!.env.example` là bắt buộc: Next.js đặt credential
production ở `.env.production.local`, mà mẫu `.env.local` đơn lẻ không bắt được
file đó.

- [ ] **Step 3: Tạo `.editorconfig`**

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.go]
indent_style = tab
indent_size = 4

[*.md]
trim_trailing_whitespace = false
```

`[*.md]` tắt cắt khoảng trắng cuối dòng vì trong Markdown hai dấu cách cuối dòng
là ngắt dòng cứng — repo này đã có vài nghìn dòng Markdown. Không có section
`[Makefile]` vì dự án dùng go-task, không có Makefile.

- [ ] **Step 3b: Tạo `.gitattributes`**

```gitattributes
* text=auto eol=lf
*.sh text eol=lf
*.png -text
```

Máy Windows thường bật `core.autocrlf=true`, làm bản clone mới có CRLF trong khi
`.editorconfig` khai `end_of_line = lf`. File này ép LF trong repo, quan trọng cho
script shell chạy trong Docker và WSL.

- [ ] **Step 4: Tạo `Taskfile.yml`**

```yaml
version: '3'

env:
  DATABASE_URL: postgres://app:app@localhost:5432/base_ecommerce?sslmode=disable

tasks:
  up:
    desc: Khởi động hạ tầng dev
    cmd: docker compose -f deploy/compose.dev.yml up -d

  down:
    desc: Dừng hạ tầng dev
    cmd: docker compose -f deploy/compose.dev.yml down

  migrate:
    desc: Chạy migration
    dir: apps/api
    cmd: goose -dir db/migrations postgres "$DATABASE_URL" up

  migrate-down:
    desc: Lùi một migration
    dir: apps/api
    cmd: goose -dir db/migrations postgres "$DATABASE_URL" down

  test-unit:
    desc: Test nhanh, không cần Docker
    dir: apps/api
    cmd: go test ./internal/platform/errs/... ./internal/platform/httpx/... ./internal/platform/config/... -race

  test:
    desc: Toàn bộ test (cần Docker)
    dir: apps/api
    cmd: go test ./... -race

  lint:
    desc: Chạy linter
    dir: apps/api
    cmd: golangci-lint run

  arch:
    desc: Kiểm tra chiều phụ thuộc kiến trúc
    cmd: bash scripts/check-arch.sh

  run:
    desc: Chạy API server
    dir: apps/api
    cmd: go run ./cmd/api
```

Mọi task đều phải có `desc` — `task --list` chỉ hiện những task có mô tả, và `run`
là task dùng nhiều nhất.

- [ ] **Step 5: Cài công cụ**

```bash
go install github.com/go-task/task/v3/cmd/task@latest
go install github.com/pressly/goose/v3/cmd/goose@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

⚠️ **Chú ý đường dẫn `/v2` của golangci-lint.** Đường dẫn không có `/v2` chỉ giải
ra bản v1.x, mà file `.golangci.yml` ở Task 13 dùng schema `version: "2"` — v1 sẽ
từ chối thẳng với `you are using a configuration file for golangci-lint v2 with
golangci-lint v1`.

Kiểm tra: `task --version` in ra số phiên bản, và `golangci-lint --version` phải
báo bản **2.x**.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: khởi tạo monorepo, Go module và Taskfile

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Hạ tầng dev bằng Docker Compose

**Files:**
- Create: `deploy/compose.dev.yml`, `.env.example`

- [ ] **Step 1: Tạo `deploy/compose.dev.yml`**

```yaml
name: base-ecommerce-dev

services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: app
      POSTGRES_DB: base_ecommerce
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app -d base_ecommerce"]
      interval: 5s
      timeout: 3s
      retries: 10

  redis:
    image: redis:7-alpine
    command: ["redis-server", "--appendonly", "yes"]
    ports: ["6379:6379"]
    volumes: ["redisdata:/data"]
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

  rabbitmq:
    image: rabbitmq:4-management-alpine
    environment:
      RABBITMQ_DEFAULT_USER: app
      RABBITMQ_DEFAULT_PASS: app
    ports: ["5672:5672", "15672:15672"]
    volumes: ["rabbitdata:/var/lib/rabbitmq"]
    healthcheck:
      test: ["CMD", "rabbitmq-diagnostics", "-q", "ping"]
      interval: 10s
      timeout: 5s
      retries: 10

volumes:
  pgdata:
  redisdata:
  rabbitdata:
```

- [ ] **Step 2: Tạo `.env.example`**

```bash
# Ứng dụng
APP_ENV=development
APP_VERSION=dev
LOG_LEVEL=debug

# HTTP
HTTP_ADDR=:8080
HTTP_READ_TIMEOUT=15s
HTTP_WRITE_TIMEOUT=30s
HTTP_SHUTDOWN_TIMEOUT=30s

# PostgreSQL
DATABASE_URL=postgres://app:app@localhost:5432/base_ecommerce?sslmode=disable
DB_MAX_CONNS=20
DB_MIN_CONNS=2
DB_MAX_CONN_LIFETIME=1h
```

- [ ] **Step 3: Khởi động và kiểm tra**

```bash
cp .env.example .env
task up
docker compose -f deploy/compose.dev.yml ps
```

Kết quả mong đợi: cả 3 service ở trạng thái `running (healthy)`. RabbitMQ UI mở
được ở http://localhost:15672 (app/app).

- [ ] **Step 4: Commit**

```bash
git add deploy/compose.dev.yml .env.example
git commit -m "chore: docker-compose cho Postgres, Redis, RabbitMQ

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Migration đầu tiên

**Files:**
- Create: `apps/api/db/migrations/00001_extensions.sql`

- [ ] **Step 1: Tạo migration**

```sql
-- +goose Up
-- pg_trgm: tìm kiếm gần đúng tên sản phẩm (dùng từ P1)
CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- btree_gin: index kết hợp cột thường với cột JSONB trong bộ lọc catalog
CREATE EXTENSION IF NOT EXISTS btree_gin;

-- +goose Down
DROP EXTENSION IF EXISTS btree_gin;
DROP EXTENSION IF EXISTS pg_trgm;
```

Không cài `pgcrypto`: UUID v7 sinh ở tầng `domain` bằng Go, không dùng hàm sinh
UUID của Postgres (xem tài liệu thiết kế 02 mục 1.1).

- [ ] **Step 2: Chạy migration**

Run: `task migrate`
Expected: `OK   00001_extensions.sql` và `goose: successfully migrated database to version: 1`

- [ ] **Step 3: Kiểm tra**

```bash
docker compose -f deploy/compose.dev.yml exec postgres \
  psql -U app -d base_ecommerce -c "\dx"
```
Expected: bảng liệt kê có `btree_gin`, `pg_trgm`, `plpgsql`.

- [ ] **Step 4: Commit**

```bash
git add apps/api/db/migrations
git commit -m "feat(db): migration khởi tạo extension

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `platform/errs` — kiểu lỗi kernel

**Files:**
- Create: `apps/api/internal/platform/errs/errs.go`
- Test: `apps/api/internal/platform/errs/errs_test.go`

`errs` **không được import `net/http`**. Việc map sang mã HTTP nằm ở `httpx`.
Đó là điều kiện để `domain` import `errs` mà vẫn giữ được quy tắc 3.1.

- [ ] **Step 1: Cài testify**

```bash
cd apps/api && go get github.com/stretchr/testify@latest
```

- [ ] **Step 2: Viết test thất bại**

`apps/api/internal/platform/errs/errs_test.go`:

```go
package errs_test

import (
	"errors"
	"fmt"
	"testing"

	"base-ecommerce/api/internal/platform/errs"
	"github.com/stretchr/testify/require"
)

var errProductNotFound = errs.New(errs.KindNotFound, "PRODUCT_NOT_FOUND", "Không tìm thấy sản phẩm")

func TestError_ErrorsIs_KhopTheoCode(t *testing.T) {
	// Lỗi đi qua nhiều tầng và bị bọc lại vẫn phải nhận diện được.
	wrapped := fmt.Errorf("pgstore: %w", errProductNotFound)
	require.ErrorIs(t, wrapped, errProductNotFound)
}

func TestError_ErrorsIs_KhacCodeThiKhongKhop(t *testing.T) {
	other := errs.New(errs.KindNotFound, "BRAND_NOT_FOUND", "Không tìm thấy thương hiệu")
	require.NotErrorIs(t, other, errProductNotFound)
}

func TestError_Unwrap_GiuLoiGoc(t *testing.T) {
	cause := errors.New("connection refused")
	e := errs.Wrap(cause, errs.KindUnavailable, "SERVICE_UNAVAILABLE", "Dịch vụ tạm thời gián đoạn")
	require.ErrorIs(t, e, cause)
	require.Contains(t, e.Error(), "connection refused")
}

func TestFrom_LoiLaThiTraVeInternal(t *testing.T) {
	e := errs.From(errors.New("boom"))
	require.Equal(t, errs.KindInternal, e.Kind)
	require.Equal(t, "INTERNAL_ERROR", e.Code)
	// Thông điệp trả cho người dùng KHÔNG được chứa nội dung lỗi gốc.
	require.NotContains(t, e.Message, "boom")
}

func TestFrom_TrichDuocErrorDaBoc(t *testing.T) {
	wrapped := fmt.Errorf("app: %w", errProductNotFound)
	e := errs.From(wrapped)
	require.Equal(t, "PRODUCT_NOT_FOUND", e.Code)
}

func TestValidation_GomNhieuLoiTruong(t *testing.T) {
	e := errs.Validation(
		errs.FieldError{Field: "price", Code: "REQUIRED", Message: "Giá là bắt buộc"},
		errs.FieldError{Field: "sku", Code: "TOO_LONG", Message: "SKU tối đa 64 ký tự"},
	)
	require.Equal(t, errs.KindValidation, e.Kind)
	require.Equal(t, "VALIDATION_FAILED", e.Code)
	require.Len(t, e.Fields, 2)
}

func TestKindInternal_LaGiaTriKhong(t *testing.T) {
	// Quên gán Kind thì phải mặc định thành lỗi nội bộ (500), không phải 200/404.
	var e errs.Error
	require.Equal(t, errs.KindInternal, e.Kind)
}
```

- [ ] **Step 3: Chạy test để chắc chắn nó thất bại**

Run: `cd apps/api && go test ./internal/platform/errs/... -v`
Expected: FAIL — `no required module provides package base-ecommerce/api/internal/platform/errs`

- [ ] **Step 4: Viết implementation**

`apps/api/internal/platform/errs/errs.go`:

```go
// Package errs định nghĩa kiểu lỗi dùng chung cho toàn hệ thống.
//
// Đây là package kernel: chỉ phụ thuộc thư viện chuẩn và KHÔNG import net/http.
// Nhờ vậy tầng domain được phép import nó mà không phá vỡ quy tắc chiều phụ thuộc.
package errs

import (
	"errors"
	"fmt"
)

// Kind phân loại lỗi theo ngữ nghĩa, độc lập với giao thức truyền tải.
// Việc map Kind sang mã HTTP là trách nhiệm của tầng httpx.
type Kind uint8

const (
	// KindInternal cố ý là giá trị 0: quên gán Kind thì mặc định thành 500.
	KindInternal Kind = iota
	KindInvalid
	KindUnauthenticated
	KindForbidden
	KindNotFound
	KindConflict
	KindValidation
	KindRateLimited
	KindUnavailable
)

// FieldError mô tả một lỗi ở cấp trường dữ liệu.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error là kiểu lỗi chuẩn của hệ thống.
//
// Code là hợp đồng với client, một khi công bố thì không đổi.
// Message là tiếng Việt, hiển thị được cho người dùng cuối.
// cause là lỗi gốc, chỉ dùng để ghi log — không bao giờ lộ ra response.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Fields  []FieldError
	cause   error
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Wrap(cause error, kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message, cause: cause}
}

func Validation(fields ...FieldError) *Error {
	return &Error{
		Kind:    KindValidation,
		Code:    "VALIDATION_FAILED",
		Message: "Dữ liệu không hợp lệ",
		Fields:  fields,
	}
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// Is so sánh theo Code thay vì theo con trỏ, nhờ vậy errors.Is vẫn đúng
// khi lỗi được tạo lại ở tầng khác hoặc đã bị bọc nhiều lần.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// From trích *Error ra khỏi chuỗi lỗi.
// Lỗi không xác định được quy về lỗi nội bộ với thông điệp trung tính —
// không bao giờ để nội dung lỗi gốc lọt ra ngoài.
func From(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return New(KindInternal, "INTERNAL_ERROR", "Đã có lỗi xảy ra")
}
```

- [ ] **Step 5: Chạy test để xác nhận đã xanh**

Run: `cd apps/api && go test ./internal/platform/errs/... -v`
Expected: PASS — 7 test đều `--- PASS`

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/platform/errs apps/api/go.mod apps/api/go.sum
git commit -m "feat(errs): kiểu lỗi kernel dùng chung

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `platform/httpx` — helper HTTP và mô hình lỗi RFC 7807

**Files:**
- Create: `apps/api/internal/platform/httpx/handler.go`, `json.go`, `problem.go`
- Test: `apps/api/internal/platform/httpx/httpx_test.go`

- [ ] **Step 1: Cài Chi**

```bash
cd apps/api && go get github.com/go-chi/chi/v5@latest
```

- [ ] **Step 2: Viết test thất bại**

`apps/api/internal/platform/httpx/httpx_test.go`:

```go
package httpx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"base-ecommerce/api/internal/platform/errs"
	"base-ecommerce/api/internal/platform/httpx"
	"github.com/stretchr/testify/require"
)

type problem struct {
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Status    int               `json:"status"`
	Code      string            `json:"code"`
	RequestID string            `json:"request_id"`
	Errors    []errs.FieldError `json:"errors"`
}

func doRequest(t *testing.T, h http.HandlerFunc, body string) (*httptest.ResponseRecorder, problem) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	var p problem
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
	}
	return rec, p
}

func TestWrap_KhongLoiThiKhongDungToiResponse(t *testing.T) {
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		httpx.JSON(w, http.StatusOK, map[string]string{"ok": "yes"})
		return nil
	})
	rec, _ := doRequest(t, h, "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestWrap_MapKindSangMaHTTP(t *testing.T) {
	tests := []struct {
		name   string
		kind   errs.Kind
		status int
	}{
		{"internal", errs.KindInternal, 500},
		{"invalid", errs.KindInvalid, 400},
		{"unauthenticated", errs.KindUnauthenticated, 401},
		{"forbidden", errs.KindForbidden, 403},
		{"not found", errs.KindNotFound, 404},
		{"conflict", errs.KindConflict, 409},
		{"validation", errs.KindValidation, 422},
		{"rate limited", errs.KindRateLimited, 429},
		{"unavailable", errs.KindUnavailable, 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
				return errs.New(tt.kind, "SOME_CODE", "Thông điệp")
			})
			rec, p := doRequest(t, h, "")
			require.Equal(t, tt.status, rec.Code)
			require.Equal(t, tt.status, p.Status)
			require.Equal(t, "SOME_CODE", p.Code)
			require.Equal(t, "application/problem+json; charset=utf-8",
				rec.Header().Get("Content-Type"))
		})
	}
}

func TestWrap_LoiNoiBoKhongLamLoThongTinBenTrong(t *testing.T) {
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errors.New(`pq: duplicate key value violates unique constraint "products_sku_key"`)
	})
	rec, p := doRequest(t, h, "")

	require.Equal(t, 500, rec.Code)
	require.Equal(t, "INTERNAL_ERROR", p.Code)
	body := rec.Body.String()
	require.NotContains(t, body, "products_sku_key")
	require.NotContains(t, body, "duplicate key")
}

func TestWrap_LoiValidateTraVeDanhSachTruong(t *testing.T) {
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errs.Validation(errs.FieldError{
			Field: "price", Code: "REQUIRED", Message: "Giá là bắt buộc",
		})
	})
	rec, p := doRequest(t, h, "")
	require.Equal(t, 422, rec.Code)
	require.Len(t, p.Errors, 1)
	require.Equal(t, "price", p.Errors[0].Field)
}

func TestProblemType_SinhTuCode(t *testing.T) {
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return errs.New(errs.KindNotFound, "PRODUCT_NOT_FOUND", "Không tìm thấy")
	})
	_, p := doRequest(t, h, "")
	require.Equal(t, "/errors/product-not-found", p.Type)
}

func TestDecode_JSONHongThiTraMalformed(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		_, err := httpx.Decode[input](w, r)
		return err
	})
	rec, p := doRequest(t, h, `{"name": `)
	require.Equal(t, 400, rec.Code)
	require.Equal(t, "MALFORMED_REQUEST", p.Code)
}

func TestDecode_TruongLaThiTuChoi(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		_, err := httpx.Decode[input](w, r)
		return err
	})
	rec, p := doRequest(t, h, `{"name":"a","khong_ton_tai":1}`)
	require.Equal(t, 400, rec.Code)
	require.Equal(t, "MALFORMED_REQUEST", p.Code)
}

func TestDecode_HopLe(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	var got input
	h := httpx.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		v, err := httpx.Decode[input](w, r)
		got = v
		return err
	})
	rec, _ := doRequest(t, h, `{"name":"Asus ROG"}`)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "Asus ROG", got.Name)
}
```

- [ ] **Step 3: Chạy test để chắc chắn nó thất bại**

Run: `cd apps/api && go test ./internal/platform/httpx/... -v`
Expected: FAIL — package `httpx` chưa tồn tại

- [ ] **Step 4: Viết `json.go`**

`apps/api/internal/platform/httpx/json.go`:

```go
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"base-ecommerce/api/internal/platform/errs"
)

// MaxBodyBytes giới hạn kích thước body. Upload file đi đường riêng qua
// presigned URL của object storage, không qua endpoint JSON.
const MaxBodyBytes = 1 << 20 // 1 MB

// JSON ghi response JSON. Gọi sau khi đã ghi header, trước khi return nil.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Header đã gửi đi rồi, không sửa được status nữa — chỉ còn cách ghi log.
		slog.Error("không mã hóa được response", "err", err)
	}
}

// NoContent trả 204 không body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Decode đọc body JSON thành T.
// Trường lạ bị từ chối để lỗi đánh máy ở client lộ ra ngay thay vì bị bỏ qua âm thầm.
func Decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, errs.Wrap(err, errs.KindInvalid, "MALFORMED_REQUEST",
			"Dữ liệu gửi lên không hợp lệ")
	}
	return v, nil
}
```

- [ ] **Step 5: Viết `problem.go`**

`apps/api/internal/platform/httpx/problem.go`:

```go
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"base-ecommerce/api/internal/platform/errs"
	"github.com/go-chi/chi/v5/middleware"
)

// Problem là body lỗi theo RFC 7807, thêm hai trường ngoài chuẩn:
// code (hợp đồng ổn định cho client) và request_id (để tra log).
type Problem struct {
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Status    int               `json:"status"`
	Detail    string            `json:"detail,omitempty"`
	Code      string            `json:"code"`
	RequestID string            `json:"request_id,omitempty"`
	Errors    []errs.FieldError `json:"errors,omitempty"`
}

func statusOf(k errs.Kind) int {
	switch k {
	case errs.KindInvalid:
		return http.StatusBadRequest
	case errs.KindUnauthenticated:
		return http.StatusUnauthorized
	case errs.KindForbidden:
		return http.StatusForbidden
	case errs.KindNotFound:
		return http.StatusNotFound
	case errs.KindConflict:
		return http.StatusConflict
	case errs.KindValidation:
		return http.StatusUnprocessableEntity
	case errs.KindRateLimited:
		return http.StatusTooManyRequests
	case errs.KindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// WriteError là nơi DUY NHẤT map lỗi sang HTTP response.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	e := errs.From(err)
	status := statusOf(e.Kind)
	reqID := middleware.GetReqID(r.Context())

	// Lỗi từ 500 trở lên là lỗi của mình — phải ghi log kèm nguyên nhân gốc.
	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request thất bại",
			"err", err, "request_id", reqID, "path", r.URL.Path)
	}

	// Chỉ những trường dưới đây được ra ngoài. e.cause KHÔNG bao giờ có mặt.
	p := Problem{
		Type:      "/errors/" + strings.ToLower(strings.ReplaceAll(e.Code, "_", "-")),
		Title:     e.Message,
		Status:    status,
		Code:      e.Code,
		RequestID: reqID,
		Errors:    e.Fields,
	}

	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.Error("không mã hóa được problem response", "err", err)
	}
}
```

- [ ] **Step 6: Viết `handler.go`**

`apps/api/internal/platform/httpx/handler.go`:

```go
package httpx

import "net/http"

// Handler giống http.HandlerFunc nhưng trả error, nhờ vậy handler không phải
// tự xử lý response lỗi và việc map lỗi được dồn về một chỗ duy nhất.
type Handler func(w http.ResponseWriter, r *http.Request) error

// Wrap chuyển Handler thành http.HandlerFunc để gắn vào router.
func Wrap(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			WriteError(w, r, err)
		}
	}
}
```

- [ ] **Step 7: Chạy test để xác nhận đã xanh**

Run: `cd apps/api && go test ./internal/platform/httpx/... -v`
Expected: PASS — gồm cả 9 sub-test của `TestWrap_MapKindSangMaHTTP`

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/platform/httpx apps/api/go.mod apps/api/go.sum
git commit -m "feat(httpx): handler trả error và mô hình lỗi RFC 7807

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: `platform/config` — đọc và validate biến môi trường

**Files:**
- Create: `apps/api/internal/platform/config/config.go`
- Test: `apps/api/internal/platform/config/config_test.go`

- [ ] **Step 1: Viết test thất bại**

`apps/api/internal/platform/config/config_test.go`:

```go
package config_test

import (
	"testing"
	"time"

	"base-ecommerce/api/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestLoad_DayDuBienThiThanhCong(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/db?sslmode=disable")
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("DB_MAX_CONNS", "30")
	t.Setenv("HTTP_READ_TIMEOUT", "20s")

	c, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, "production", c.Env)
	require.Equal(t, ":9000", c.HTTP.Addr)
	require.EqualValues(t, 30, c.DB.MaxConns)
	require.Equal(t, 20*time.Second, c.HTTP.ReadTimeout)
}

func TestLoad_DungGiaTriMacDinhKhiThieuBienKhongBatBuoc(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")

	c, err := config.Load()

	require.NoError(t, err)
	require.Equal(t, "development", c.Env)
	require.Equal(t, ":8080", c.HTTP.Addr)
	require.EqualValues(t, 20, c.DB.MaxConns)
	require.Equal(t, time.Hour, c.DB.MaxConnLifetime)
}

func TestLoad_ThieuBienBatBuocThiLoi(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := config.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_GomTatCaLoiTrongMotLan(t *testing.T) {
	// Sai nhiều biến thì phải báo hết trong một lần, không phải sửa từng cái một.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_MAX_CONNS", "khong-phai-so")
	t.Setenv("HTTP_READ_TIMEOUT", "khong-phai-thoi-gian")

	_, err := config.Load()

	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "DATABASE_URL")
	require.Contains(t, msg, "DB_MAX_CONNS")
	require.Contains(t, msg, "HTTP_READ_TIMEOUT")
}

func TestConfig_StringCheGiaTriNhayCam(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:matkhausieubimat@localhost:5432/db")

	c, err := config.Load()
	require.NoError(t, err)
	require.NotContains(t, c.String(), "matkhausieubimat")
}

func TestConfig_IsProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("APP_ENV", "production")

	c, err := config.Load()
	require.NoError(t, err)
	require.True(t, c.IsProduction())
}
```

- [ ] **Step 2: Chạy test để chắc chắn nó thất bại**

Run: `cd apps/api && go test ./internal/platform/config/... -v`
Expected: FAIL — package chưa tồn tại

- [ ] **Step 3: Viết implementation**

`apps/api/internal/platform/config/config.go`:

```go
// Package config đọc cấu hình từ biến môi trường và validate ngay lúc khởi động.
//
// Nguyên tắc: thiếu hoặc sai biến thì tiến trình thoát ngay với thông báo rõ ràng,
// thay vì chạy được rồi mới hỏng lúc có request đầu tiên chạm tới.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env     string
	Version string
	LogLevel string
	HTTP    HTTP
	DB      DB
}

type HTTP struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type DB struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// String che các giá trị nhạy cảm để an toàn khi ghi log toàn bộ config.
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{Env:%s Version:%s HTTP.Addr:%s DB.DSN:%s DB.MaxConns:%d}",
		c.Env, c.Version, c.HTTP.Addr, redactDSN(c.DSN()), c.DB.MaxConns,
	)
}

func (c *Config) DSN() string { return c.DB.DSN }

// redactDSN thay mật khẩu trong DSN bằng ***.
func redactDSN(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	scheme := strings.Index(dsn, "://")
	if at < 0 || scheme < 0 || at < scheme {
		return dsn
	}
	creds := dsn[scheme+3 : at]
	if colon := strings.Index(creds, ":"); colon >= 0 {
		return dsn[:scheme+3] + creds[:colon] + ":***" + dsn[at:]
	}
	return dsn
}

func Load() (*Config, error) {
	l := &loader{}

	c := &Config{
		Env:      l.str("APP_ENV", "development"),
		Version:  l.str("APP_VERSION", "dev"),
		LogLevel: l.str("LOG_LEVEL", "info"),
		HTTP: HTTP{
			Addr:            l.str("HTTP_ADDR", ":8080"),
			ReadTimeout:     l.dur("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    l.dur("HTTP_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: l.dur("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		DB: DB{
			DSN:             l.required("DATABASE_URL"),
			MaxConns:        int32(l.num("DB_MAX_CONNS", 20)),
			MinConns:        int32(l.num("DB_MIN_CONNS", 2)),
			MaxConnLifetime: l.dur("DB_MAX_CONN_LIFETIME", time.Hour),
		},
	}

	if err := l.err(); err != nil {
		return nil, err
	}
	return c, nil
}

// loader gom lỗi lại thay vì dừng ở lỗi đầu tiên, để một lần chạy báo hết
// mọi biến sai — sửa một lượt thay vì sửa từng cái.
type loader struct{ errs []error }

func (l *loader) str(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (l *loader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		l.errs = append(l.errs, fmt.Errorf("thiếu biến môi trường bắt buộc %s", key))
	}
	return v
}

func (l *loader) num(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s phải là số nguyên, nhận được %q", key, raw))
		return def
	}
	return v
}

func (l *loader) dur(key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s phải là khoảng thời gian (ví dụ 15s, 1h), nhận được %q", key, raw))
		return def
	}
	return v
}

func (l *loader) err() error {
	if len(l.errs) == 0 {
		return nil
	}
	return fmt.Errorf("cấu hình không hợp lệ: %w", errors.Join(l.errs...))
}
```

- [ ] **Step 4: Chạy test để xác nhận đã xanh**

Run: `cd apps/api && go test ./internal/platform/config/... -v`
Expected: PASS — 6 test

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/platform/config
git commit -m "feat(config): đọc env và validate lúc khởi động

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: `platform/testdb` — Postgres thật cho test

**Files:**
- Create: `apps/api/internal/platform/testdb/testdb.go`

Mẫu: một container cho mỗi package test, một database riêng cho mỗi test tạo từ
template. Nhờ vậy test chạy song song được và không phụ thuộc thứ tự.

- [ ] **Step 1: Cài thư viện**

```bash
cd apps/api
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/postgres@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/pressly/goose/v3@latest
go get github.com/google/uuid@latest
```

- [ ] **Step 2: Viết `testdb.go`**

`apps/api/internal/platform/testdb/testdb.go`:

```go
// Package testdb cung cấp Postgres thật cho integration test.
//
// Không mock database: phần dễ sai nhất là SQL, mà mock thì không kiểm được SQL.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // driver database/sql, CHỈ dùng cho goose
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const templateDB = "template_test"

var (
	adminPool *pgxpool.Pool // trỏ tới database quản trị, dùng để CREATE/DROP DATABASE
	adminDSN  string
)

// Setup khởi động container, chạy migration một lần vào database mẫu.
// Gọi từ TestMain của package cần database:
//
//	func TestMain(m *testing.M) { os.Exit(testdb.Setup(m, "../../../db/migrations")) }
func Setup(m *testing.M, migrationsDir string) int {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase(templateDB),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		log.Printf("không khởi động được container postgres: %v", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(container) }()

	adminDSN, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Printf("không lấy được DSN: %v", err)
		return 1
	}

	// Chạy migration MỘT lần vào database mẫu.
	sqlDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		log.Printf("không mở được kết nối cho goose: %v", err)
		return 1
	}
	if err := goose.SetDialect("postgres"); err != nil {
		log.Printf("goose dialect: %v", err)
		return 1
	}
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		log.Printf("chạy migration thất bại: %v", err)
		return 1
	}
	_ = sqlDB.Close()

	adminPool, err = pgxpool.New(ctx, adminDSN)
	if err != nil {
		log.Printf("không tạo được admin pool: %v", err)
		return 1
	}
	defer adminPool.Close()

	return m.Run()
}

// New tạo một database sạch riêng cho test này và trả về pool trỏ tới nó.
// Database được xóa tự động khi test kết thúc.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	require.NotNil(t, adminPool, "chưa gọi testdb.Setup trong TestMain")

	ctx := context.Background()
	name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20]

	// CREATE DATABASE ... TEMPLATE chỉ mất vài chục ms vì migration đã chạy sẵn.
	_, err := adminPool.Exec(ctx,
		fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateDB))
	require.NoError(t, err, "không tạo được database cho test")

	dsn := strings.Replace(adminDSN, "/"+templateDB+"?", "/"+name+"?", 1)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		// Ngắt mọi kết nối còn sót trước khi xóa, nếu không DROP sẽ bị từ chối.
		_, _ = adminPool.Exec(context.Background(),
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, name)
		_, _ = adminPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name)
	})

	return pool
}
```

- [ ] **Step 3: Kiểm tra biên dịch**

Run: `cd apps/api && go build ./...`
Expected: không có output (thành công)

Nếu `tcpostgres.Run` báo không tồn tại, phiên bản testcontainers đang dùng là bản
cũ — đổi thành `tcpostgres.RunContainer(ctx, opts...)` và bỏ tham số image (chuyển
sang `testcontainers.WithImage("postgres:17-alpine")`).

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/platform/testdb apps/api/go.mod apps/api/go.sum
git commit -m "test: helper testdb dùng testcontainers và template database

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `platform/postgres` — pool và DBTX

**Files:**
- Create: `apps/api/internal/platform/postgres/dbtx.go`, `pool.go`
- Test: `apps/api/internal/platform/postgres/pool_test.go`, `main_test.go`

- [ ] **Step 1: Viết `TestMain` chung cho package**

`apps/api/internal/platform/postgres/main_test.go`:

```go
package postgres_test

import (
	"os"
	"testing"

	"base-ecommerce/api/internal/platform/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Setup(m, "../../../db/migrations"))
}
```

- [ ] **Step 2: Viết test thất bại**

`apps/api/internal/platform/postgres/pool_test.go`:

```go
package postgres_test

import (
	"context"
	"testing"
	"time"

	"base-ecommerce/api/internal/platform/config"
	"base-ecommerce/api/internal/platform/postgres"
	"base-ecommerce/api/internal/platform/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestNewPool_DSNSaiThiLoiNgayLucKhoiDong(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := postgres.NewPool(ctx, config.DB{
		DSN:      "postgres://khong:ton@127.0.0.1:1/tai?sslmode=disable&connect_timeout=1",
		MaxConns: 5, MinConns: 1, MaxConnLifetime: time.Hour,
	})

	require.Error(t, err)
}

func TestManager_DB_NgoaiTransactionThiTraPool(t *testing.T) {
	t.Parallel()
	pool := testdb.New(t)
	m := postgres.NewManager(pool)

	db := m.DB(context.Background())

	// Ngoài transaction, DB(ctx) phải chính là pool.
	_, ok := db.(*pgxpool.Pool)
	require.True(t, ok, "ngoài transaction thì DB(ctx) phải trả về pool")
}

func TestDBTX_PoolVaTxDeuThoaManInterface(t *testing.T) {
	t.Parallel()
	pool := testdb.New(t)

	var _ postgres.DBTX = pool

	tx, err := pool.Begin(context.Background())
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()

	var _ postgres.DBTX = tx
}
```

- [ ] **Step 3: Chạy test để chắc chắn nó thất bại**

Run: `cd apps/api && go test ./internal/platform/postgres/... -v`
Expected: FAIL — `undefined: postgres.NewPool`

- [ ] **Step 4: Viết `dbtx.go`**

`apps/api/internal/platform/postgres/dbtx.go`:

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

- [ ] **Step 5: Viết `pool.go`**

`apps/api/internal/platform/postgres/pool.go`:

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

- [ ] **Step 6: Viết `Manager` tối thiểu để test chạy được**

`apps/api/internal/platform/postgres/tx.go` (bản đầu, mở rộng ở Task 9):

```go
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

// Manager sở hữu pool và cung cấp transaction cho tầng app.
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
```

- [ ] **Step 7: Chạy test để xác nhận đã xanh**

Run: `cd apps/api && go test ./internal/platform/postgres/... -v`
Expected: PASS — 3 test. Lần chạy đầu mất ~30 giây do phải tải image Postgres.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/platform/postgres
git commit -m "feat(postgres): pgxpool và interface DBTX

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: `platform/postgres` — TxManager

**Files:**
- Modify: `apps/api/internal/platform/postgres/tx.go`
- Test: `apps/api/internal/platform/postgres/tx_test.go`

- [ ] **Step 1: Viết test thất bại**

`apps/api/internal/platform/postgres/tx_test.go`:

```go
package postgres_test

import (
	"context"
	"errors"
	"testing"

	"base-ecommerce/api/internal/platform/postgres"
	"base-ecommerce/api/internal/platform/testdb"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// setupTxTest tạo bảng tạm để thử transaction. Mỗi test có database riêng
// nên không cần dọn dẹp giữa các test.
func setupTxTest(t *testing.T) (*postgres.Manager, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	_, err := pool.Exec(context.Background(),
		`CREATE TABLE tx_test (id INT PRIMARY KEY, note TEXT NOT NULL)`)
	require.NoError(t, err)
	return postgres.NewManager(pool), pool
}

func countRows(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM tx_test`).Scan(&n))
	return n
}

func TestRun_ThanhCongThiCommit(t *testing.T) {
	t.Parallel()
	m, pool := setupTxTest(t)

	err := m.Run(context.Background(), func(ctx context.Context) error {
		_, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_test VALUES (1, 'a')`)
		return err
	})

	require.NoError(t, err)
	require.Equal(t, 1, countRows(t, pool))
}

func TestRun_LoiThiRollbackToanBo(t *testing.T) {
	t.Parallel()
	m, pool := setupTxTest(t)
	boom := errors.New("boom")

	err := m.Run(context.Background(), func(ctx context.Context) error {
		if _, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_test VALUES (1, 'a')`); err != nil {
			return err
		}
		if _, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_test VALUES (2, 'b')`); err != nil {
			return err
		}
		return boom
	})

	require.ErrorIs(t, err, boom)
	require.Equal(t, 0, countRows(t, pool), "rollback phải xóa sạch cả hai dòng")
}

func TestRun_TrongTransactionThiDBTraVeTx(t *testing.T) {
	t.Parallel()
	m, _ := setupTxTest(t)

	err := m.Run(context.Background(), func(ctx context.Context) error {
		_, isPool := m.DB(ctx).(*pgxpool.Pool)
		require.False(t, isPool, "trong transaction thì DB(ctx) không được trả về pool")
		return nil
	})
	require.NoError(t, err)
}

func TestRun_LongNhauThiDungChungMotTransaction(t *testing.T) {
	t.Parallel()
	m, pool := setupTxTest(t)
	boom := errors.New("boom")

	err := m.Run(context.Background(), func(ctx context.Context) error {
		if _, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_test VALUES (1, 'ngoai')`); err != nil {
			return err
		}
		// Run lồng nhau phải tái sử dụng transaction hiện tại, không mở cái mới.
		if err := m.Run(ctx, func(ctx context.Context) error {
			_, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_test VALUES (2, 'trong')`)
			return err
		}); err != nil {
			return err
		}
		return boom // lỗi ở ngoài phải rollback CẢ phần ghi bên trong
	})

	require.ErrorIs(t, err, boom)
	require.Equal(t, 0, countRows(t, pool))
}

func TestRun_PanicThiKhongRoRiKetNoi(t *testing.T) {
	t.Parallel()
	m, pool := setupTxTest(t)

	require.Panics(t, func() {
		_ = m.Run(context.Background(), func(ctx context.Context) error {
			_, _ = m.DB(ctx).Exec(ctx, `INSERT INTO tx_test VALUES (1, 'a')`)
			panic("nổ giữa chừng")
		})
	})

	// Kết nối phải được trả lại pool, nếu không câu lệnh dưới đây sẽ treo.
	require.Equal(t, 0, countRows(t, pool))
}

func TestRun_ContextDaHuyVanRollbackDuoc(t *testing.T) {
	t.Parallel()
	m, pool := setupTxTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	err := m.Run(ctx, func(ctx context.Context) error {
		if _, err := m.DB(ctx).Exec(ctx, `INSERT INTO tx_test VALUES (1, 'a')`); err != nil {
			return err
		}
		cancel() // client ngắt kết nối giữa chừng
		return errors.New("client bỏ đi")
	})

	require.Error(t, err)
	require.Equal(t, 0, countRows(t, pool))
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"serialization failure", &pgconn.PgError{Code: "40001"}, true},
		{"deadlock detected", &pgconn.PgError{Code: "40P01"}, true},
		{"unique violation", &pgconn.PgError{Code: "23505"}, false},
		{"lỗi thường", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, postgres.IsRetryable(tt.err))
		})
	}
}
```

- [ ] **Step 2: Chạy test để chắc chắn nó thất bại**

Run: `cd apps/api && go test ./internal/platform/postgres/... -run TestRun -v`
Expected: FAIL — `m.Run undefined`

- [ ] **Step 3: Viết implementation đầy đủ cho `tx.go`**

Thay toàn bộ `apps/api/internal/platform/postgres/tx.go`:

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
	// MaxRetries số lần thử lại khi gặp lỗi tuần tự hóa. 0 = dùng mặc định 3.
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

- [ ] **Step 4: Chạy test để xác nhận đã xanh**

Run: `cd apps/api && go test ./internal/platform/postgres/... -race -v`
Expected: PASS — 11 test (gồm 4 sub-test của `TestIsRetryable`)

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/platform/postgres
git commit -m "feat(postgres): TxManager truyền transaction qua context

Cho phép tầng app mở transaction mà không import pgx. Kèm retry lỗi
tuần tự hóa và rollback an toàn khi context đã bị hủy.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: `platform/observability` — log JSON và middleware

**Files:**
- Create: `apps/api/internal/platform/observability/log.go`, `middleware.go`
- Test: `apps/api/internal/platform/observability/log_test.go`

- [ ] **Step 1: Viết test thất bại**

`apps/api/internal/platform/observability/log_test.go`:

```go
package observability_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"base-ecommerce/api/internal/platform/observability"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_GhiRaJSON(t *testing.T) {
	var buf bytes.Buffer
	log := observability.NewLogger(&buf, "info", "production", "v1.2.3")

	log.Info("xin chào", "khoa", "giatri")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	require.Equal(t, "xin chào", entry["msg"])
	require.Equal(t, "giatri", entry["khoa"])
	require.Equal(t, "v1.2.3", entry["version"])
	require.Equal(t, "production", entry["env"])
}

func TestNewLogger_TonTrongMucLog(t *testing.T) {
	var buf bytes.Buffer
	log := observability.NewLogger(&buf, "warn", "production", "v1")

	log.Info("không được ghi")
	require.Empty(t, buf.String())

	log.Warn("phải được ghi")
	require.Contains(t, buf.String(), "phải được ghi")
}

func TestRequestLogger_GhiPhuongThucDuongDanTrangThai(t *testing.T) {
	var buf bytes.Buffer
	log := observability.NewLogger(&buf, "info", "test", "v1")

	h := middleware.RequestID(
		observability.RequestLogger(log)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/san-pham/abc", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	require.Equal(t, "GET", entry["method"])
	require.Equal(t, "/san-pham/abc", entry["path"])
	require.EqualValues(t, 418, entry["status"])
	require.NotEmpty(t, entry["request_id"])
	require.Contains(t, entry, "duration_ms")
}

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug":     slog.LevelDebug,
		"info":      slog.LevelInfo,
		"warn":      slog.LevelWarn,
		"error":     slog.LevelError,
		"lung-tung": slog.LevelInfo, // giá trị lạ thì về mặc định
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			require.Equal(t, want, observability.ParseLevel(in))
		})
	}
}
```

- [ ] **Step 2: Chạy test để chắc chắn nó thất bại**

Run: `cd apps/api && go test ./internal/platform/observability/... -v`
Expected: FAIL — package chưa tồn tại

- [ ] **Step 3: Viết `log.go`**

`apps/api/internal/platform/observability/log.go`:

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

- [ ] **Step 4: Viết `middleware.go`**

`apps/api/internal/platform/observability/middleware.go`:

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

- [ ] **Step 5: Chạy test để xác nhận đã xanh**

Run: `cd apps/api && go test ./internal/platform/observability/... -v`
Expected: PASS — 3 test + 5 sub-test của `TestParseLevel`

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/platform/observability
git commit -m "feat(observability): log JSON và middleware ghi request

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: Health check và router

**Files:**
- Create: `apps/api/internal/platform/health/health.go`, `apps/api/internal/server/router.go`
- Test: `apps/api/internal/platform/health/health_test.go`

- [ ] **Step 1: Viết test thất bại**

`apps/api/internal/platform/health/health_test.go`:

```go
package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"base-ecommerce/api/internal/platform/health"
	"github.com/stretchr/testify/require"
)

type fakeChecker struct {
	name string
	err  error
}

func (f fakeChecker) Name() string                     { return f.name }
func (f fakeChecker) Check(context.Context) error      { return f.err }

func TestLive_LuonTraVeOKKhiChuaTat(t *testing.T) {
	h := health.New("v1.2.3")

	rec := httptest.NewRecorder()
	h.Live(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "ok", body["status"])
	require.Equal(t, "v1.2.3", body["version"])
}

func TestLive_TraVe503SauKhiDanhDauShutdown(t *testing.T) {
	h := health.New("v1")
	h.Shutdown()

	rec := httptest.NewRecorder()
	h.Live(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestReady_TatCaCheckXanhThiOK(t *testing.T) {
	h := health.New("v1", fakeChecker{name: "postgres"}, fakeChecker{name: "redis"})

	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestReady_MotCheckHongThiTraVe503VaChiRoCaiNao(t *testing.T) {
	h := health.New("v1",
		fakeChecker{name: "postgres"},
		fakeChecker{name: "redis", err: errors.New("connection refused")},
	)

	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "ok", body.Checks["postgres"])
	require.Equal(t, "fail", body.Checks["redis"])
}
```

- [ ] **Step 2: Chạy test để chắc chắn nó thất bại**

Run: `cd apps/api && go test ./internal/platform/health/... -v`
Expected: FAIL — package chưa tồn tại

- [ ] **Step 3: Viết `health.go`**

`apps/api/internal/platform/health/health.go`:

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

- [ ] **Step 4: Chạy test để xác nhận đã xanh**

Run: `cd apps/api && go test ./internal/platform/health/... -v`
Expected: PASS — 4 test

- [ ] **Step 5: Viết checker cho Postgres**

`apps/api/internal/platform/postgres/health.go`:

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

- [ ] **Step 6: Viết router**

`apps/api/internal/server/router.go`:

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

- [ ] **Step 7: Kiểm tra biên dịch**

Run: `cd apps/api && go build ./...`
Expected: không có output

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/platform/health apps/api/internal/platform/postgres/health.go apps/api/internal/server
git commit -m "feat(health): tách liveness và readiness, dựng router Chi

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: `cmd/api` — khởi động và graceful shutdown

**Files:**
- Create: `apps/api/cmd/api/main.go`

- [ ] **Step 1: Viết `main.go`**

`apps/api/cmd/api/main.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
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

- [ ] **Step 2: Chạy thử**

```bash
task up          # nếu hạ tầng chưa chạy
task migrate
task run
```
Expected: log JSON có dòng `"msg":"server đang lắng nghe","addr":":8080"`

- [ ] **Step 3: Kiểm tra hai endpoint**

Mở terminal khác:

```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
```
Expected:
- `/healthz` → `200` với `{"status":"ok","version":"dev"}`
- `/readyz` → `200` với `{"status":"ok","version":"dev","checks":{"postgres":"ok"}}`

- [ ] **Step 4: Kiểm tra readyz phát hiện được database chết**

```bash
docker compose -f deploy/compose.dev.yml stop postgres
curl -i http://localhost:8080/readyz    # kỳ vọng 503, checks.postgres = "fail"
curl -i http://localhost:8080/healthz   # kỳ vọng VẪN 200 — tiến trình còn sống
docker compose -f deploy/compose.dev.yml start postgres
```

Đây là điểm mấu chốt của việc tách hai endpoint. Nếu `/healthz` cũng trả 503 thì
đã cài sai.

- [ ] **Step 5: Kiểm tra graceful shutdown**

Nhấn `Ctrl+C` ở terminal đang chạy `task run`.
Expected: log lần lượt `nhận tín hiệu tắt, bắt đầu dừng êm` rồi `đã dừng`, tiến
trình thoát với mã 0.

- [ ] **Step 6: Kiểm tra config sai thì chết ngay**

```bash
DATABASE_URL= go run ./cmd/api
```
Expected: in ra `khởi động thất bại: cấu hình không hợp lệ: thiếu biến môi trường bắt buộc DATABASE_URL`, thoát mã 1.

- [ ] **Step 7: Commit**

```bash
git add apps/api/cmd
git commit -m "feat(api): entrypoint với graceful shutdown và subcommand healthcheck

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 13: CI và kiểm tra kiến trúc bằng máy

**Files:**
- Create: `scripts/check-arch.sh`, `apps/api/.golangci.yml`, `.github/workflows/ci.yml`

- [ ] **Step 1: Viết script kiểm tra kiến trúc**

`scripts/check-arch.sh`:

```bash
#!/usr/bin/env bash
# Kiểm tra chiều phụ thuộc của kiến trúc hexagonal bằng máy, không bằng tự giác.
# Quy tắc đầy đủ ở README mục 3.
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

- [ ] **Step 2: Chạy thử script**

```bash
chmod +x scripts/check-arch.sh
task arch
```
Expected: `Kiểm tra kiến trúc: OK` (chưa có module nghiệp vụ nên chưa có gì để vi phạm)

- [ ] **Step 3: Cấu hình golangci-lint**

`apps/api/.golangci.yml`:

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

  settings:
    errcheck:
      check-type-assertions: true

  exclusions:
    rules:
      - path: _test\.go
        linters: [errcheck, bodyclose]

formatters:
  enable:
    - gofmt
    - goimports
```

Run: `cd apps/api && golangci-lint run`
Expected: `0 issues`. Có cảnh báo thì sửa trước khi đi tiếp.

- [ ] **Step 4: Viết workflow CI**

`.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

env:
  GO_VERSION: '1.24'

jobs:
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

  test-unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: apps/api/go.sum
      - name: Test không cần Docker
        working-directory: apps/api
        run: |
          go test -race \
            ./internal/platform/errs/... \
            ./internal/platform/httpx/... \
            ./internal/platform/config/... \
            ./internal/platform/observability/... \
            ./internal/platform/health/...

  test-integration:
    runs-on: ubuntu-latest
    needs: test-unit
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: apps/api/go.sum
      - name: Test tích hợp (testcontainers)
        working-directory: apps/api
        run: go test -race ./... -coverprofile=coverage.out
      - uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: apps/api/coverage.out
```

`test-unit` chạy trước và nhanh, để lỗi thường gặp lộ ra trong ~30 giây thay vì
phải chờ hết phần tích hợp.

- [ ] **Step 5: Chạy toàn bộ ở máy để chắc chắn CI sẽ xanh**

```bash
task lint
task arch
task test
```
Expected: cả ba đều thành công. `task test` mất khoảng 1–2 phút do phải kéo image Postgres.

- [ ] **Step 6: Commit**

```bash
git add scripts .github apps/api/.golangci.yml
git commit -m "ci: lint, kiểm tra kiến trúc và test tự động

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tiêu chí hoàn thành P0.1

Chạy lần lượt và tất cả phải đúng:

```bash
task up && task migrate     # hạ tầng lên, migration chạy xong
task lint                   # 0 issue
task arch                   # Kiểm tra kiến trúc: OK
task test                   # toàn bộ test xanh
task run                    # server chạy
```

Rồi kiểm tra bằng tay:

| Kiểm tra | Kỳ vọng |
|---|---|
| `curl localhost:8080/healthz` | 200, có `version` |
| `curl localhost:8080/readyz` | 200, `checks.postgres = ok` |
| Dừng Postgres → `/readyz` | 503 |
| Dừng Postgres → `/healthz` | **vẫn 200** |
| `Ctrl+C` | dừng êm, thoát mã 0 |
| `DATABASE_URL= go run ./cmd/api` | báo lỗi rõ ràng, thoát mã 1 |

---

## Kế hoạch tiếp theo

| | Nội dung | Phụ thuộc |
|---|---|---|
| **P0.2** | Module `catalog`: domain → app → pgstore → httpapi, OpenAPI, cache Redis | P0.1 |
| **P0.3** | Outbox + `outboxrelay` + `worker`, consumer idempotent | P0.1 |
| **P0.4** | Next.js: 2 trang thật gọi API, sinh type từ OpenAPI | P0.2 |

P0.2 và P0.3 làm song song được sau khi P0.1 xong.
