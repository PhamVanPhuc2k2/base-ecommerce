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

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		// Các module nghiệp vụ gắn vào đây từ kế hoạch P0.2 trở đi.
	})

	return r
}
