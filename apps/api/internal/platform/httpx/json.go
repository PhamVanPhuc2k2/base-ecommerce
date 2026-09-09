package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"base-ecommerce/api/internal/platform/errs"
)

// MaxBodyBytes giới hạn kích thước body. Upload file đi đường riêng qua
// presigned URL của object storage, không qua endpoint JSON.
const MaxBodyBytes = 1 << 20 // 1 MB

// JSON ghi response JSON. Trả về error để handler `return httpx.JSON(...)`.
//
// Mã hóa TRƯỚC khi ghi header: nếu ghi 200 rồi mới phát hiện không mã hóa được
// thì client nhận một body rỗng, không phải JSON, mà vẫn tưởng thành công.
func JSON(w http.ResponseWriter, status int, v any) error {
	if v == nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		return nil
	}

	b, err := json.Marshal(v)
	if err != nil {
		return errs.Wrap(err, errs.KindInternal, "INTERNAL_ERROR", "Đã có lỗi xảy ra")
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(b); err != nil {
		// Client đã ngắt kết nối. Không sửa được gì nữa, chỉ ghi log.
		slog.Error("không ghi được response", "err", err)
	}
	return nil
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
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return v, errs.Wrap(err, errs.KindTooLarge, "PAYLOAD_TOO_LARGE",
				"Dữ liệu gửi lên vượt quá giới hạn cho phép")
		}
		return v, errs.Wrap(err, errs.KindInvalid, "MALFORMED_REQUEST",
			"Dữ liệu gửi lên không hợp lệ")
	}
	return v, nil
}
