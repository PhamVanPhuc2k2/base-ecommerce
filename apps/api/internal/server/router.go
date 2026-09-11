// Package server ráp các module vào router. Đây là nơi DUY NHẤT biết toàn bộ
// danh sách module của hệ thống.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"base-ecommerce/api/internal/platform/errs"
	"base-ecommerce/api/internal/platform/health"
	"base-ecommerce/api/internal/platform/httpx"
	"base-ecommerce/api/internal/platform/observability"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Module là thứ router gắn vào. Mỗi module nghiệp vụ tự khai báo route của mình.
type Module interface {
	Mount(r chi.Router)
}

// New dựng router với chuỗi middleware chuẩn.
//
// Thứ tự middleware quan trọng:
//   - RequestID trước RequestLogger, nếu không log sẽ không có request_id.
//   - Recoverer sau RequestLogger, để panic vẫn được ghi thành một dòng log request.
//   - Timeout cuối cùng, chỉ bao quanh handler nghiệp vụ.
func New(log *slog.Logger, h *health.Handler, handlerTimeout time.Duration, modules ...Module) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)

	// ClientIPFromRemoteAddr lấy IP từ socket TCP và KHÔNG tin bất kỳ header nào.
	//
	// Không dùng middleware.RealIP: nó đã bị deprecated vì ghi đè r.RemoteAddr
	// bằng X-Forwarded-For / True-Client-IP / X-Real-IP mà không kiểm tra ai gửi
	// (GHSA-3fxj-6jh8-hvhx). Nghĩa là client tự bịa header là đổi được IP —
	// hỏng cả log lẫn rate limit theo IP ở P0.2.
	//
	// Khi lên production sau Caddy/Cloudflare, đổi dòng này sang
	// middleware.ClientIPFromXFFTrustedProxies(n) với n = số proxy thật sự đứng
	// trước. Đọc IP luôn qua middleware.GetClientIP(ctx), đừng đọc r.RemoteAddr.
	r.Use(middleware.ClientIPFromRemoteAddr)

	r.Use(observability.RequestLogger(log))
	r.Use(middleware.Recoverer)

	// Health check nằm NGOÀI timeout: khi hệ thống quá tải, đây chính là lúc
	// cần chúng trả lời được nhất.
	r.Get("/healthz", h.Live)
	r.Get("/readyz", h.Ready)

	// Không khai báo thì Chi trả text/plain "404 page not found" và 405 với
	// body rỗng — cả hai đều không có `code` lẫn `request_id`. Client sinh từ
	// openapi.yaml parse body thành Problem sẽ nổ khi ai đó gõ sai URL.
	r.NotFound(httpx.Wrap(func(http.ResponseWriter, *http.Request) error {
		return errs.ErrRouteNotFound
	}))
	r.MethodNotAllowed(httpx.Wrap(func(http.ResponseWriter, *http.Request) error {
		return errs.ErrMethodNotAllowed
	}))

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Timeout(handlerTimeout))
		for _, m := range modules {
			m.Mount(r)
		}
	})

	return r
}
