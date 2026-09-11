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
//
// Lọc status NGAY TRONG callback, không phải sau khi GetOrLoad trả về: nếu lọc
// bên ngoài thì sản phẩm nháp vẫn kịp được ghi vào cache, và nằm đó 30 phút kể
// cả sau khi đã sửa hoặc đã đăng bán.
func (uc *GetProduct) BySlug(ctx context.Context, slug string) (*domain.Product, error) {
	return uc.cache.ProductBySlug(ctx, slug, TTLProduct,
		func(ctx context.Context) (*domain.Product, error) {
			p, err := uc.repo.BySlug(ctx, slug)
			if err != nil {
				return nil, err
			}
			if p.Status != domain.StatusLive {
				return nil, domain.ErrProductNotFound
			}
			return p, nil
		})
}
