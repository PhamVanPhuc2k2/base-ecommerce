package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"base-ecommerce/api/internal/platform/config"
	"base-ecommerce/api/internal/platform/health"
	"base-ecommerce/api/internal/platform/observability"
	"base-ecommerce/api/internal/platform/postgres"
	"base-ecommerce/api/internal/server"
)

// version được nhúng lúc build: -ldflags="-X main.version=$GIT_SHA"
var version = "dev"

func main() {
	// Subcommand healthcheck để Docker HEALTHCHECK gọi được — image distroless
	// không có curl hay wget.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "khởi động thất bại: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err // config sai thì chết ngay, không chạy tiếp
	}
	if cfg.Version == "dev" {
		cfg.Version = version
	}

	log := observability.NewLogger(os.Stdout, cfg.LogLevel, cfg.Env, cfg.Version)

	// BẮT BUỘC. httpx.WriteError ghi log lỗi 5xx qua slog mặc định của package
	// (nó được gọi từ Wrap, không có chỗ nào truyền logger vào). Không đặt dòng
	// này thì đúng những dòng log quan trọng nhất — 5xx kèm nguyên nhân gốc —
	// sẽ ra stderr dạng text, không có env/version và bỏ qua LOG_LEVEL, trong
	// khi log request lại là JSON ra stdout.
	slog.SetDefault(log)

	log.Info("đang khởi động", "config", cfg.String())

	// Nhận tín hiệu tắt trước khi mở tài nguyên, để Ctrl+C lúc đang kết nối
	// database cũng thoát được.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(startCtx, cfg.DB)
	if err != nil {
		return fmt.Errorf("kết nối database: %w", err)
	}
	defer pool.Close()
	log.Info("đã kết nối database")

	h := health.New(cfg.Version, postgres.NewHealthChecker(pool))

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           server.New(log, h),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server đang lắng nghe", "addr", cfg.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server dừng bất thường: %w", err)
	case <-ctx.Done():
		log.Info("nhận tín hiệu tắt, bắt đầu dừng êm")
	}

	// Bước 1: báo chưa sẵn sàng để proxy ngừng gửi request mới tới.
	h.Shutdown()

	// Bước 2: chờ proxy nhận ra. Bỏ bước này thì khách sẽ nhận 502 ngay giữa
	// lúc đang thanh toán.
	drain := 5 * time.Second
	if cfg.Env != "production" {
		drain = 0 // môi trường dev không có proxy, không cần chờ
	}
	time.Sleep(drain)

	// Bước 3: xử lý nốt request đang dở.
	shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("dừng server không sạch", "err", err)
	}

	// Bước 4: đóng tài nguyên theo chiều ngược lúc khởi tạo (defer pool.Close).
	log.Info("đã dừng")
	return nil
}

// healthcheck gọi /healthz của chính tiến trình đang chạy trong container.
func healthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "HTTP_ADDR không hợp lệ: %v\n", err)
		return 1
	}
	if host == "" {
		host = "127.0.0.1"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Get(fmt.Sprintf("http://%s:%s/healthz", host, port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck thất bại: %v\n", err)
		return 1
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck trả về %d\n", res.StatusCode)
		return 1
	}
	return 0
}
