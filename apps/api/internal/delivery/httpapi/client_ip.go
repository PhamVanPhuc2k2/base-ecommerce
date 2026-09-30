package httpapi

import (
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5/middleware"
)

// clientIPFromTrustedProxy lấy IP khách từ X-Forwarded-For — CHỈ khi socket
// đến từ một proxy tin cậy (đặc tả P2.4 mục 2.3).
//
// Vì sao cần: storefront là BFF, mọi request đăng nhập tới API đều đến từ IP
// của server Next. Chỉ nhìn socket thì rate limit 5/phút theo IP (P2.1) thành
// 5/phút cho TOÀN BỘ khách.
//
// Vì sao không dùng thẳng middleware.ClientIPFromXFF của chi: nó đọc header
// bất kể ai gửi. Ai nối thẳng vào API, tự ghi "X-Forwarded-For: 1.2.3.4" là
// đổi được IP — mỗi request một IP mới, rate limit vô nghĩa. Ở đây header chỉ
// được đọc khi socket đã nằm trong danh sách tin cậy; còn lại giữ nguyên IP
// socket mà ClientIPFromRemoteAddr đã ghi.
//
// Trong header, chi đi từ PHẢI sang TRÁI, bỏ qua IP tin cậy, lấy IP đầu tiên
// không tin cậy — phần bên trái đó là thứ client tự khai, không đáng tin.
func clientIPFromTrustedProxy(trusted []netip.Prefix) func(http.Handler) http.Handler {
	if len(trusted) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	strs := make([]string, len(trusted))
	for i, p := range trusted {
		strs[i] = p.String()
	}
	fromXFF := middleware.ClientIPFromXFF(strs...)
	return func(next http.Handler) http.Handler {
		viaXFF := fromXFF(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer := middleware.GetClientIPAddr(r.Context()) // IP socket
			for _, p := range trusted {
				if peer.IsValid() && p.Contains(peer) {
					viaXFF.ServeHTTP(w, r)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
