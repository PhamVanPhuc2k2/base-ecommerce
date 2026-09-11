package app

import (
	"context"

	"base-ecommerce/api/internal/catalog/domain"

	"github.com/google/uuid"
)

type PublishProduct struct {
	tx     TxManager
	repo   ProductRepository
	events EventPublisher
	cache  Cache
}

func NewPublishProduct(tx TxManager, repo ProductRepository, events EventPublisher, cache Cache) *PublishProduct {
	return &PublishProduct{tx: tx, repo: repo, events: events, cache: cache}
}

func (uc *PublishProduct) Execute(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	var published *domain.Product

	// repo.ByID PHẢI nằm trong closure. TxManager chạy lại closure khi gặp lỗi
	// tuần tự hóa, và cả sự kiện lẫn oldSlug đều phụ thuộc vào việc đọc lại
	// aggregate ở mỗi lần thử. Nhấc nó ra ngoài thì lần thử thứ hai phát ra
	// không sự kiện nào và xóa nhầm khóa cache.
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		p, err := uc.repo.ByID(ctx, id)
		if err != nil {
			return err
		}
		// Quy tắc nghiệp vụ nằm ở domain, không ở đây.
		if err := p.Publish(); err != nil {
			return err
		}
		if err := uc.repo.Save(ctx, p); err != nil {
			return err
		}
		published = p
		return uc.events.Publish(ctx, p.PullEvents()...)
	}); err != nil {
		return nil, err
	}

	// WithoutCancel: transaction đã commit rồi, việc xóa cache KHÔNG được hủy
	// theo. Client ngắt kết nối giữa chừng mà cache không xóa thì dữ liệu cũ
	// nằm lại tới hết TTL.
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyProductSlug(published.Slug))
	return published, nil
}
