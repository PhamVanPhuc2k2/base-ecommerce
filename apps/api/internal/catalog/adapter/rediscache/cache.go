// Package rediscache cài đặt port app.Cache bằng Redis.
package rediscache

import (
	"context"
	"time"

	"base-ecommerce/api/internal/catalog/app"
	"base-ecommerce/api/internal/catalog/domain"
	platformredis "base-ecommerce/api/internal/platform/redis"
)

type Cache struct{ c *platformredis.Cache }

func New(c *platformredis.Cache) *Cache { return &Cache{c: c} }

func (a *Cache) ProductBySlug(ctx context.Context, slug string, ttl time.Duration,
	load func(context.Context) (*domain.Product, error)) (*domain.Product, error) {
	return platformredis.GetOrLoad(ctx, a.c, app.KeyProductSlug(slug), ttl, load)
}

func (a *Cache) CategoryTree(ctx context.Context, ttl time.Duration,
	load func(context.Context) ([]*domain.Category, error)) ([]*domain.Category, error) {
	return platformredis.GetOrLoad(ctx, a.c, app.KeyCategoryTree(), ttl, load)
}

func (a *Cache) Invalidate(ctx context.Context, keys ...string) {
	a.c.Delete(ctx, keys...)
}
