// Package usecase chứa use case của catalog — tầng Use Cases của Clean
// Architecture.
//
// usecase KHÔNG được import net/http, pgx, redis, delivery hay repository. Nó chỉ
// biết các interface khai báo trong file này — đó là điều kiện để đổi hạ tầng
// mà không phải sửa nghiệp vụ. scripts/check-arch.sh kiểm điều này.
package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"base-ecommerce/api/internal/domain"

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
	// Attributes lọc products.attributes; VariantOptions lọc options của
	// variant active. Use case chia attr.* vào hai map theo định nghĩa.
	Attributes     map[string]string
	VariantOptions map[string]string
	Sort           SortOption
	Page           int
	Limit          int
}

type ProductRepository interface {
	Save(ctx context.Context, p *domain.Product) error
	ByID(ctx context.Context, id uuid.UUID) (*domain.Product, error)
	BySlug(ctx context.Context, slug string) (*domain.Product, error)
	// List chỉ lấy một trang. Số đếm tách ra Count để use case cache được nó —
	// count(*) là 99,7% chi phí của trang danh sách (đo ở P0.2, 200k dòng).
	List(ctx context.Context, f ListFilter) ([]*domain.Product, error)
	Count(ctx context.Context, f ListFilter) (int, error)
	Facets(ctx context.Context, f ListFilter, productCodes, variantCodes []string) (domain.FacetCounts, error)
	// Sitemap trả slug + updated_at của sản phẩm live, sắp theo id.
	Sitemap(ctx context.Context, offset, limit int) ([]SitemapEntry, error)
	CountLive(ctx context.Context) (int, error)
}

// SitemapEntry là đúng hai thứ một dòng sitemap cần — không nạp cả sản phẩm.
type SitemapEntry struct {
	Slug      string
	UpdatedAt time.Time
}

type MediaRepository interface {
	Insert(ctx context.Context, m *domain.Media) error
	// ByID khóa dòng (FOR UPDATE) — chỉ dùng trong use case ghi.
	ByID(ctx context.Context, id uuid.UUID) (*domain.Media, error)
	Update(ctx context.Context, m *domain.Media) error
	ReadyKeys(ctx context.Context, keys []string) (map[string]bool, error)
}

// ObjectStorage là kho file (MinIO/S3). Chỉ kiểu stdlib trong chữ ký: use
// case không được biết SDK nào đứng sau.
type ObjectStorage interface {
	PresignPost(ctx context.Context, key, contentType string, maxBytes int64, ttl time.Duration) (string, map[string]string, error)
	Stat(ctx context.Context, key string) (exists bool, size int64, err error)
	ReadHead(ctx context.Context, key string, n int64) ([]byte, error)
	Remove(ctx context.Context, key string) error
}

type AttributeRepository interface {
	Catalog(ctx context.Context) (*domain.AttributeCatalog, error)
	// ByID khóa dòng (FOR UPDATE) — chỉ dùng trong use case ghi.
	ByID(ctx context.Context, id uuid.UUID) (*domain.AttributeDefinition, error)
	Insert(ctx context.Context, d *domain.AttributeDefinition) error
	Update(ctx context.Context, d *domain.AttributeDefinition) error
	Delete(ctx context.Context, id uuid.UUID) error
	ReplaceAssignments(ctx context.Context, categoryID uuid.UUID, assigns []domain.CategoryAttribute) error
}

type CategoryRepository interface {
	All(ctx context.Context) (domain.Categories, error)
	// LockForWrite xếp hàng mọi lần ghi danh mục cho tới hết transaction. Phải
	// gọi TRƯỚC khi đọc cây để kiểm vòng lặp — xem đặc tả P1.1 mục 2.1.
	LockForWrite(ctx context.Context) error
	ByID(ctx context.Context, id uuid.UUID) (*domain.Category, error)
	Insert(ctx context.Context, c *domain.Category) error
	Update(ctx context.Context, c *domain.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type BrandRepository interface {
	All(ctx context.Context) (domain.Brands, error)
	// ByID khóa dòng (FOR UPDATE) — chỉ dùng trong use case ghi.
	ByID(ctx context.Context, id uuid.UUID) (*domain.Brand, error)
	Insert(ctx context.Context, b *domain.Brand) error
	Update(ctx context.Context, b *domain.Brand) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// Cache là cổng ra cache. Cài đặt phải NUỐT mọi lỗi hạ tầng: cache hỏng thì
// đọc thẳng nguồn, không bao giờ trả lỗi cho người dùng vì cache.
type Cache interface {
	ProductBySlug(ctx context.Context, slug string, ttl time.Duration,
		load func(context.Context) (*domain.Product, error)) (*domain.Product, error)
	CategoryTree(ctx context.Context, ttl time.Duration,
		load func(context.Context) (domain.Categories, error)) (domain.Categories, error)
	Brands(ctx context.Context, ttl time.Duration,
		load func(context.Context) (domain.Brands, error)) (domain.Brands, error)
	ProductCount(ctx context.Context, key string, ttl time.Duration,
		load func(context.Context) (int, error)) (int, error)
	AttributeCatalog(ctx context.Context, ttl time.Duration,
		load func(context.Context) (*domain.AttributeCatalog, error)) (*domain.AttributeCatalog, error)
	ProductFacets(ctx context.Context, key string, ttl time.Duration,
		load func(context.Context) (domain.FacetCounts, error)) (domain.FacetCounts, error)
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
func KeyBrands() string                 { return "brand:all" }
func KeyAttributeCatalog() string       { return "attribute:catalog" }

// KeyProductFacets dùng chung phần băm với số đếm: cùng bộ lọc thì cùng tập
// kết quả, chỉ khác thứ được tính trên nó.
func KeyProductFacets(f ListFilter) string {
	return "product:facets:" + filterHash(f)
}

// KeyProductCount băm bộ lọc ĐÃ CHUẨN HÓA thành khóa cache số đếm.
//
// Chỉ gồm những gì đổi số đếm: page, limit, sort không có mặt — trang 1 và
// trang 7 của cùng bộ lọc dùng chung một số đếm. Mọi tập hợp đều được sắp xếp
// trước khi băm, vì map trong Go duyệt ngẫu nhiên: không sắp thì cùng một bộ
// lọc ra khóa khác nhau mỗi lần và tỉ lệ trúng cache về 0 mà không ai hay.
func KeyProductCount(f ListFilter) string {
	return "product:count:" + filterHash(f)
}

func filterHash(f ListFilter) string {
	ids := make([]string, len(f.CategoryIDs))
	for i, id := range f.CategoryIDs {
		ids[i] = id.String()
	}
	sort.Strings(ids)
	attrs := make([]string, 0, len(f.Attributes))
	for k, v := range f.Attributes {
		attrs = append(attrs, k+"="+v)
	}
	sort.Strings(attrs)
	// Tùy chọn biến thể tách riêng: attr.ram=16 lọc ở cấp sản phẩm và ở cấp
	// biến thể cho ra tập KHÁC nhau, không được chung khóa cache.
	vopts := make([]string, 0, len(f.VariantOptions))
	for k, v := range f.VariantOptions {
		vopts = append(vopts, k+"="+v)
	}
	sort.Strings(vopts)
	dec := func(d *decimal.Decimal) string {
		if d == nil {
			return ""
		}
		return d.String()
	}
	norm, _ := json.Marshal([]any{ids, f.BrandSlug, dec(f.PriceMin), dec(f.PriceMax), attrs, vopts})
	sum := sha256.Sum256(norm)
	return hex.EncodeToString(sum[:16])
}

const (
	TTLProduct      = 30 * time.Minute
	TTLCategoryTree = 6 * time.Hour
	TTLBrands       = 6 * time.Hour
	TTLAttributes   = 6 * time.Hour
	TTLFacets       = 60 * time.Second
	// TTLProductCount ngắn và KHÔNG vô hiệu hóa khi ghi: lệch tối đa một phút
	// chỉ làm trang cuối thiếu/thừa sản phẩm vừa đăng. Theo dõi mọi bộ lọc mà
	// một lần ghi ảnh hưởng tới thì đắt hơn nhiều. Xem đặc tả P1.1 mục 2.5.
	TTLProductCount = 60 * time.Second
)

// ---- Xác thực (P2.1) ----

type UserRepository interface {
	Insert(ctx context.Context, u *domain.User) error
	// ByEmail / ByID trả (nil, nil) khi không có.
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	ByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// RefreshToken là một dòng refresh_tokens. Hash là SHA-256 của token — token
// thô không bao giờ rời khỏi use case trừ trong response cho chính chủ.
type RefreshToken struct {
	ID        uuid.UUID
	FamilyID  uuid.UUID
	UserID    uuid.UUID
	Hash      []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

type RefreshTokenRepository interface {
	Insert(ctx context.Context, t RefreshToken) error
	// ByHashForUpdate khóa dòng (FOR UPDATE); (nil, nil) khi không có.
	ByHashForUpdate(ctx context.Context, hash []byte) (*RefreshToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error
}

type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(plain, encoded string) (bool, error)
}

type TokenIssuer interface {
	Issue(userID, sessionID uuid.UUID, now time.Time) (token string, expiresAt time.Time, err error)
}

// RateLimiter: ok=false khi vượt giới hạn, kèm thời gian phải chờ. Cài đặt
// PHẢI fail-open (hỏng thì cho qua) — thiết kế 03 mục 7.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (ok bool, wait time.Duration)
}
