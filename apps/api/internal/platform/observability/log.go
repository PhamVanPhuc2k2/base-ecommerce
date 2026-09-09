// Package observability cung cấp log có cấu trúc và middleware quan sát.
package observability

import (
	"io"
	"log/slog"
	"strings"
)

// NewLogger tạo logger ghi JSON ra w.
//
// JSON ra stdout là yêu cầu của 12-factor: tiến trình không tự quản lý file log,
// hạ tầng thu gom. Nhờ vậy chuyển sang Kubernetes sau này không phải sửa gì.
func NewLogger(w io.Writer, level, env, version string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: ParseLevel(level)})
	return slog.New(h).With("env", env, "version", version)
}

func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
