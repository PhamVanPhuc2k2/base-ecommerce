package main

import (
	"context"
	"log/slog"
	"time"

	"base-ecommerce/api/internal/usecase"
)

const (
	// expiryInterval: giữ chỗ hết hạn được nhả trễ nhất ~30 giây. Hàng "kẹt"
	// thêm nửa phút sau 15 phút chờ thanh toán là không đáng kể; quét dày hơn
	// chỉ tốn truy vấn.
	expiryInterval = 30 * time.Second
	expiryBatch    = 100
)

// runExpirySweeper nhả giữ chỗ hết hạn định kỳ (đặc tả P3.2 mục 2.5).
//
// Lỗi KHÔNG làm tiến trình chết: database chớp tắt thì lượt sau quét tiếp,
// giữ chỗ hết hạn chỉ nằm thêm một nhịp. Nhiều bản worker chạy cùng lúc vẫn
// đúng — DueForUpdate dùng SKIP LOCKED.
func runExpirySweeper(ctx context.Context, uc *usecase.Reservations, log *slog.Logger) error {
	t := time.NewTicker(expiryInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		// Lặp tới khi một lô không đầy: dồn nhiều giữ chỗ hết hạn (worker vừa
		// khởi động lại) thì nhả hết trong một nhịp, không phải 100 cái mỗi 30 giây.
		for {
			n, err := uc.ExpireDue(ctx, expiryBatch)
			if err != nil {
				log.Error("nhả giữ chỗ hết hạn thất bại", "err", err)
				break
			}
			if n > 0 {
				log.Info("đã nhả giữ chỗ hết hạn", "count", n)
			}
			if n < expiryBatch {
				break
			}
		}
	}
}
