package app

import (
	"context"

	"base-ecommerce/api/internal/catalog/domain"

	"github.com/google/uuid"
)

type UpdateProductInput struct {
	ID               uuid.UUID
	Name             string
	ShortDescription string
	Price            domain.Money
	Attributes       map[string]string
	Images           []string
}

type UpdateProduct struct {
	tx     TxManager
	repo   ProductRepository
	events EventPublisher
	cache  Cache
}

func NewUpdateProduct(tx TxManager, repo ProductRepository, events EventPublisher, cache Cache) *UpdateProduct {
	return &UpdateProduct{tx: tx, repo: repo, events: events, cache: cache}
}

func (uc *UpdateProduct) Execute(ctx context.Context, in UpdateProductInput) (*domain.Product, error) {
	var (
		updated *domain.Product
		oldSlug string
	)

	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		p, err := uc.repo.ByID(ctx, in.ID)
		if err != nil {
			return err
		}
		oldSlug = p.Slug

		if err := p.Update(in.Name, in.ShortDescription, in.Price, in.Attributes, in.Images); err != nil {
			return err
		}
		if err := uc.repo.Save(ctx, p); err != nil {
			return err
		}
		updated = p
		return uc.events.Publish(ctx, p.PullEvents()...)
	}); err != nil {
		return nil, err
	}

	// Xóa cả slug cũ lẫn mới: đổi tên sản phẩm là đổi slug, để sót slug cũ thì
	// đường dẫn cũ vẫn trả dữ liệu cũ cho tới khi hết TTL.
	uc.cache.Invalidate(ctx, KeyProductSlug(oldSlug), KeyProductSlug(updated.Slug))

	return updated, nil
}
