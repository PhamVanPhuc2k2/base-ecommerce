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
