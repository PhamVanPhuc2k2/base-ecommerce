// Package rediscache cài đặt port usecase.Cache bằng Redis.
package rediscache

import (
	"context"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	platformredis "base-ecommerce/api/pkg/redis"
)

type Cache struct{ c *platformredis.Cache }

func New(c *platformredis.Cache) *Cache { return &Cache{c: c} }

func (a *Cache) ProductBySlug(ctx context.Context, slug string, ttl time.Duration,
	load func(context.Context) (*domain.Product, error)) (*domain.Product, error) {
	return platformredis.GetOrLoad(ctx, a.c, usecase.KeyProductSlug(slug), ttl, load)
}

// CategoryTree cache kiểu domain.Categories chứ không phải []*domain.Category:
// kiểu có Validate() nên GetOrLoad từ chối được `[null]` — phần tử nil sẽ làm
// NewTree panic. Lỗ này có từ P0.2.
func (a *Cache) CategoryTree(ctx context.Context, ttl time.Duration,
	load func(context.Context) (domain.Categories, error)) (domain.Categories, error) {
	return platformredis.GetOrLoad(ctx, a.c, usecase.KeyCategoryTree(), ttl, load)
}

func (a *Cache) Brands(ctx context.Context, ttl time.Duration,
	load func(context.Context) (domain.Brands, error)) (domain.Brands, error) {
	return platformredis.GetOrLoad(ctx, a.c, usecase.KeyBrands(), ttl, load)
}

// ProductCount cache số đếm dưới dạng *int, KHÔNG phải int.
//
// GetOrLoad từ chối `null` bằng cách kiểm con trỏ nil. Với int thì
// json.Unmarshal("null") để nguyên 0 và không báo lỗi — một khóa hỏng thành
// null sẽ ra total 0, has_next false, và phân trang biến mất trong khi dữ liệu
// vẫn còn. Không lỗi, không log. Đặc tả P1.1 mục 2.5.
func (a *Cache) ProductCount(ctx context.Context, key string, ttl time.Duration,
	load func(context.Context) (int, error)) (int, error) {
	n, err := platformredis.GetOrLoad(ctx, a.c, key, ttl, func(ctx context.Context) (*int, error) {
		v, err := load(ctx)
		if err != nil {
			return nil, err
		}
		return &v, nil
	})
	if err != nil {
		return 0, err
	}
	return *n, nil
}

func (a *Cache) AttributeCatalog(ctx context.Context, ttl time.Duration,
	load func(context.Context) (*domain.AttributeCatalog, error)) (*domain.AttributeCatalog, error) {
	return platformredis.GetOrLoad(ctx, a.c, usecase.KeyAttributeCatalog(), ttl, load)
}

func (a *Cache) ProductFacets(ctx context.Context, key string, ttl time.Duration,
	load func(context.Context) (domain.FacetCounts, error)) (domain.FacetCounts, error) {
	return platformredis.GetOrLoad(ctx, a.c, key, ttl, load)
}

func (a *Cache) Invalidate(ctx context.Context, keys ...string) {
	a.c.Delete(ctx, keys...)
}
