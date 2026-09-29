package httpx

import (
	"log/slog"
	"net/http"
)

// Handler giống http.HandlerFunc nhưng trả error, nhờ vậy handler không phải
// tự xử lý response lỗi và việc map lỗi được dồn về một chỗ duy nhất.
type Handler func(w http.ResponseWriter, r *http.Request) error

// committedWriter ghi nhớ response đã bắt đầu được gửi đi hay chưa.
type committedWriter struct {
	http.ResponseWriter
	committed bool
}

func (w *committedWriter) WriteHeader(code int) {
	w.committed = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *committedWriter) Write(b []byte) (int, error) {
	w.committed = true
	return w.ResponseWriter.Write(b)
}

// Wrap chuyển Handler thành http.HandlerFunc để gắn vào router.
func Wrap(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cw := &committedWriter{ResponseWriter: w}

		err := h(cw, r)
		if err == nil {
			return
		}

		// Header đã gửi đi rồi thì không sửa status được nữa. Ghi đè sẽ tạo ra
		// body chứa hai JSON document nối nhau, client đọc document đầu và
		// tưởng request thành công — nguy hiểm hơn hẳn việc mất thông tin lỗi.
		if cw.committed {
			slog.ErrorContext(r.Context(), "handler lỗi sau khi đã ghi response",
				"err", err, "path", r.URL.Path)
			return
		}

		WriteError(cw, r, err)
	}
}
