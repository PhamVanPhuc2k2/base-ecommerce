package app

import (
	"context"

	"base-ecommerce/api/internal/catalog/domain"

	"github.com/google/uuid"
)

type CreateProductInput struct {
	SKU              string
	Name             string
	ShortDescription string
	CategoryID       uuid.UUID
	BrandID          uuid.UUID
	Price            domain.Money
	Attributes       map[string]string
	Images           []string
}

type CreateProduct struct {
	tx     TxManager
	repo   ProductRepository
	events EventPublisher
	cache  Cache
}

func NewCreateProduct(tx TxManager, repo ProductRepository, events EventPublisher, cache Cache) *CreateProduct {
	return &CreateProduct{tx: tx, repo: repo, events: events, cache: cache}
}

func (uc *CreateProduct) Execute(ctx context.Context, in CreateProductInput) (*domain.Product, error) {
	// Validate và sinh ID xảy ra TRƯỚC transaction: không giữ kết nối database
	// trong lúc làm việc không cần database.
	p, err := domain.NewProduct(in.SKU, in.Name, in.ShortDescription,
		in.CategoryID, in.BrandID, in.Price, in.Attributes, in.Images)
	if err != nil {
		return nil, err
	}

	events := p.PullEvents()

	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		if err := uc.repo.Save(ctx, p); err != nil {
			return err
		}
		// CÙNG transaction với repo.Save. Từ P0.3 đây là ghi vào bảng outbox,
		// nên đã commit nghĩa là sự kiện chắc chắn tồn tại.
		return uc.events.Publish(ctx, events...)
	}); err != nil {
		return nil, err
	}

	// Xóa cache SAU khi commit. Xóa trước rồi commit lỗi thì cache có thể được
	// nạp lại bằng dữ liệu chưa commit; xóa sau mà rollback thì chỉ tốn một
	// lần đọc lại.
	//
	// WithoutCancel: transaction đã commit rồi, việc xóa cache KHÔNG được hủy
	// theo. Client ngắt kết nối giữa chừng mà cache không xóa thì dữ liệu cũ
	// nằm lại tới hết TTL.
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyProductSlug(p.Slug))

	return p, nil
}
