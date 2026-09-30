// Package httpapi là tầng delivery HTTP: router go-chi, chuỗi middleware chuẩn
// và handler gọi xuống usecase. Không bao giờ gọi thẳng repository.
package httpapi

import (
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"base-ecommerce/api/pkg/errs"
	"base-ecommerce/api/pkg/health"
	"base-ecommerce/api/pkg/httpx"
	"base-ecommerce/api/pkg/observability"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Module là thứ router gắn vào. Mỗi nhóm handler tự khai báo route của mình.
type Module interface {
	Mount(r chi.Router)
}

// NewRouter dựng router go-chi với chuỗi middleware chuẩn.
//
// Thứ tự middleware quan trọng:
//   - RequestID trước RequestLogger, nếu không log sẽ không có request_id.
//   - Recoverer sau RequestLogger, để panic vẫn được ghi thành một dòng log request.
//   - Timeout cuối cùng, chỉ bao quanh handler nghiệp vụ.
func NewRouter(log *slog.Logger, h *health.Handler, handlerTimeout time.Duration,
	trustedProxies []netip.Prefix, modules ...Module) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)

	// ClientIPFromRemoteAddr lấy IP từ socket TCP và KHÔNG tin bất kỳ header nào.
	//
	// Không dùng middleware.RealIP: nó đã bị deprecated vì ghi đè r.RemoteAddr
	// bằng X-Forwarded-For / True-Client-IP / X-Real-IP mà không kiểm tra ai gửi
	// (GHSA-3fxj-6jh8-hvhx). Nghĩa là client tự bịa header là đổi được IP —
	// hỏng cả log lẫn rate limit theo IP ở P0.2.
	//
	// Đọc IP luôn qua middleware.GetClientIP(ctx), đừng đọc r.RemoteAddr.
	r.Use(middleware.ClientIPFromRemoteAddr)
	// ...rồi GHI ĐÈ bằng X-Forwarded-For, nhưng chỉ khi socket là proxy tin cậy.
	r.Use(clientIPFromTrustedProxy(trustedProxies))

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
