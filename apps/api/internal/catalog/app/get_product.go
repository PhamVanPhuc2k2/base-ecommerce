package app

import (
	"context"

	"base-ecommerce/api/internal/catalog/domain"
)

type GetProduct struct {
	repo  ProductRepository
	cache Cache
}

func NewGetProduct(repo ProductRepository, cache Cache) *GetProduct {
	return &GetProduct{repo: repo, cache: cache}
}

// BySlug đọc theo cache-aside. Không mở transaction: đọc một bản ghi không cần.
func (uc *GetProduct) BySlug(ctx context.Context, slug string) (*domain.Product, error) {
	return uc.cache.ProductBySlug(ctx, slug, TTLProduct,
		func(ctx context.Context) (*domain.Product, error) {
			return uc.repo.BySlug(ctx, slug)
		})
}
