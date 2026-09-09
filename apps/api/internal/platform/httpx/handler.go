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
