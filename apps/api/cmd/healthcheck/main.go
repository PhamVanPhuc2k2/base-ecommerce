// Lệnh healthcheck gọi một endpoint HTTP và thoát 0 nếu nhận 200, 1 nếu không.
//
// Vì sao cần cả một binary cho việc này: image chạy là distroless/static — không
// có shell, không có curl, không có wget. `HEALTHCHECK CMD curl ...` sẽ luôn
// thất bại vì trong container không tồn tại curl, và Docker sẽ báo container
// unhealthy mãi mãi trong khi ứng dụng vẫn chạy tốt.
//
// Đổi sang image có shell chỉ để healthcheck là đánh đổi sai: kéo theo cả một
// userland và bề mặt tấn công đi kèm. Một binary tĩnh vài MB rẻ hơn nhiều.
//
// Dùng:
//
//	healthcheck                       # mặc định http://127.0.0.1:8080/healthz
//	healthcheck http://127.0.0.1:9000/readyz
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

const defaultURL = "http://127.0.0.1:8080/healthz"

func main() {
	url := defaultURL
	if len(os.Args) > 1 && os.Args[1] != "" {
		url = os.Args[1]
	}

	// Timeout ngắn hơn `timeout` của HEALTHCHECK trong compose, để lỗi ở đây là
	// "ứng dụng không trả lời" chứ không phải "Docker giết mất tiến trình kiểm tra".
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck: url không hợp lệ:", err)
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck: không gọi được:", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: mã trạng thái", resp.StatusCode)
		os.Exit(1)
	}
}
