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
	Code      string            `json:"code"`
	RequestID string            `json:"request_id"`
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
	case errs.KindTooLarge:
		return http.StatusRequestEntityTooLarge
	case errs.KindMethodNotAllowed:
		return http.StatusMethodNotAllowed
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
	//
	// Trừ KindUnavailable: hết giờ và client tự hủy không phải lỗi của server.
	// Ghi chúng ở mức ERROR làm nhiễu cảnh báo đúng lúc hệ thống đang tải cao,
	// tức là lúc cần đọc log nhất.
	switch {
	case e.Kind == errs.KindUnavailable:
		slog.WarnContext(r.Context(), "request không hoàn tất",
			"err", err, "request_id", reqID, "path", r.URL.Path)
	case status >= http.StatusInternalServerError:
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
