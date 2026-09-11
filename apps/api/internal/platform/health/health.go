// Package health cung cấp hai endpoint kiểm tra sức khỏe khác nhau.
//
// /healthz (liveness): tiến trình còn sống không. Hỏng thì phải khởi động lại.
// /readyz  (readiness): có sẵn sàng nhận request không (ping được DB, cache...).
//
// Hai cái này KHÁC nhau. Gộp lại thì Redis chập chờn sẽ khiến hạ tầng khởi động
// lại một API vốn vẫn phục vụ tốt.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// Checker là một phụ thuộc bên ngoài cần kiểm tra ở /readyz.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// Optional đánh dấu một phụ thuộc mà thiếu nó API vẫn phục vụ đúng, chỉ chậm
// hơn. Phụ thuộc kiểu này hỏng thì /readyz vẫn trả 200.
//
// Vì sao phải phân biệt: /readyz trả lời đúng MỘT câu — "instance này có phục
// vụ được request không?". Redis chết thì câu trả lời vẫn là CÓ, vì mọi đường
// đọc đều rơi xuống Postgres. Cho Redis làm 503 nghĩa là biến một sự cố cache
// thành sự cố toàn hệ thống: load balancer rút hết instance ra khỏi vòng quay
// trong khi từng instance vẫn đang trả 200 cho mọi endpoint thật.
type Optional interface {
	Optional() bool
}

func isOptional(c Checker) bool {
	o, ok := c.(Optional)
	return ok && o.Optional()
}

type Handler struct {
	version  string
	checkers []Checker
	down     atomic.Bool // bật lên khi bắt đầu tắt máy
}

func New(version string, checkers ...Checker) *Handler {
	return &Handler{version: version, checkers: checkers}
}

// Shutdown đánh dấu tiến trình đang tắt. Gọi ngay khi nhận SIGTERM, TRƯỚC khi
// đóng server, để proxy kịp ngừng gửi request mới tới.
func (h *Handler) Shutdown() { h.down.Store(true) }

func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	if h.down.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "shutting_down", "version": h.version,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "version": h.version,
	})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.down.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "shutting_down"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := make(map[string]string, len(h.checkers))
	status, code := "ok", http.StatusOK
	for _, c := range h.checkers {
		if err := c.Check(ctx); err != nil {
			checks[c.Name()] = "fail"
			if isOptional(c) {
				// Vẫn báo "degraded" để cảnh báo nổ, nhưng giữ 200 để instance
				// không bị rút khỏi load balancer.
				if status == "ok" {
					status = "degraded"
				}
				continue
			}
			status, code = "degraded", http.StatusServiceUnavailable
			continue
		}
		checks[c.Name()] = "ok"
	}

	writeJSON(w, code, map[string]any{
		"status": status, "version": h.version, "checks": checks,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
