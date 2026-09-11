package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"base-ecommerce/api/internal/platform/errs"
	"base-ecommerce/api/internal/platform/httpx"
)

var errUnauthenticated = errs.New(errs.KindUnauthenticated, "UNAUTHENTICATED",
	"Thiếu hoặc sai khóa quản trị")

// RequireAdminKey bảo vệ các endpoint ghi cho tới khi P2 có JWT + RBAC.
//
// XÓA MIDDLEWARE NÀY khi P2 xong — đây là giải pháp tạm, không phải mô hình
// xác thực của hệ thống.
//
// So sánh qua SHA-256 rồi mới ConstantTimeCompare: ConstantTimeCompare trả 0
// ngay khi độ dài khác nhau, nên so trực tiếp sẽ làm lộ độ dài khóa qua thời
// gian phản hồi. Băm trước làm hai vế luôn dài 32 byte.
func RequireAdminKey(key string) func(http.Handler) http.Handler {
	want := sha256.Sum256([]byte(key))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := sha256.Sum256([]byte(r.Header.Get("X-Admin-Key")))
			if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
				httpx.WriteError(w, r, errUnauthenticated)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
