// Package app chứa use case của catalog.
//
// app KHÔNG được import net/http, pgx, redis hay bất kỳ adapter nào. Nó chỉ
// biết các interface khai báo trong file này — đó là điều kiện để đổi hạ tầng
// mà không phải sửa nghiệp vụ. scripts/check-arch.sh kiểm điều này.
package app

import (
	"context"
	"time"

	"base-ecommerce/api/internal/catalog/domain"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type SortOption string

const (
	SortNewest    SortOption = "newest"
	SortPriceAsc  SortOption = "price_asc"
	SortPriceDesc SortOption = "price_desc"
)

// ListFilter là bộ lọc đã được chuẩn hóa. CategoryIDs đã giải sẵn từ cây danh
// mục, nên repository chỉ cần một câu SQL phẳng.
type ListFilter struct {
	CategoryIDs []uuid.UUID
	BrandSlug   string
	PriceMin    *decimal.Decimal
	PriceMax    *decimal.Decimal
	Attributes  map[string]string
	Sort        SortOption
	Page        int
	Limit       int
}

type ProductRepository interface {
	Save(ctx context.Context, p *domain.Product) error
	ByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	BySlug(ctx context.Context, slug string) (*domain.Product, error)
	List(ctx context.Context, f ListFilter) (items []*domain.Product, total int, err error)
}

type CategoryRepository interface {
	All(ctx context.Context) ([]*domain.Category, error)
}

// Cache là cổng ra cache. Cài đặt phải NUỐT mọi lỗi hạ tầng: cache hỏng thì
// đọc thẳng nguồn, không bao giờ trả lỗi cho người dùng vì cache.
type Cache interface {
	ProductBySlug(ctx context.Context, slug string, ttl time.Duration,
		load func(context.Context) (*domain.Product, error)) (*domain.Product, error)
	CategoryTree(ctx context.Context, ttl time.Duration,
		load func(context.Context) ([]*domain.Category, error)) ([]*domain.Category, error)
	Invalidate(ctx context.Context, keys ...string)
}

// EventPublisher phát sự kiện nghiệp vụ.
//
// P0.2 cài bản ghi log. P0.3 thay bằng adapter outbox ghi vào database trong
// CÙNG transaction — và không phải sửa file này hay bất kỳ use case nào.
type EventPublisher interface {
	Publish(ctx context.Context, events ...domain.Event) error
}

type TxManager interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}

// Khóa cache — tập trung một chỗ để use case và adapter không lệch nhau.
func KeyProductSlug(slug string) string { return "product:slug:" + slug }
func KeyCategoryTree() string           { return "category:tree" }

const (
	TTLProduct      = 30 * time.Minute
	TTLCategoryTree = 6 * time.Hour
)
