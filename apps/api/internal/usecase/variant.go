package usecase

import (
	"context"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// AddVariant và UpdateVariant chung một file và chung một khuôn với
// UpdateProduct: khóa aggregate (repo.ByID là FOR UPDATE) → domain sửa → Save
// cả aggregate → phát sự kiện → sau commit xóa cache theo slug.
//
// Không có DeleteVariant: đơn hàng (P4) sẽ trỏ vào variant, nên ngừng bán là
// status = inactive. Đặc tả P1.2 mục 2.2.

type AddVariant struct {
	tx     TxManager
	repo   ProductRepository
	events EventPublisher
	cache  Cache
}

func NewAddVariant(tx TxManager, repo ProductRepository, events EventPublisher, cache Cache) *AddVariant {
	return &AddVariant{tx: tx, repo: repo, events: events, cache: cache}
}

func (uc *AddVariant) Execute(ctx context.Context, productID uuid.UUID, in domain.VariantInput) (*domain.Product, error) {
	return mutateProduct(ctx, uc.tx, uc.repo, uc.events, uc.cache, productID, func(p *domain.Product) error {
		_, err := p.AddVariant(in)
		return err
	})
}

type UpdateVariantInput struct {
	ProductID uuid.UUID
	VariantID uuid.UUID
	Price     *domain.Money
	Options   map[string]string
	Status    *domain.VariantStatus
	Position  *int
}

type UpdateVariant struct {
	tx     TxManager
	repo   ProductRepository
	events EventPublisher
	cache  Cache
}

func NewUpdateVariant(tx TxManager, repo ProductRepository, events EventPublisher, cache Cache) *UpdateVariant {
	return &UpdateVariant{tx: tx, repo: repo, events: events, cache: cache}
}

func (uc *UpdateVariant) Execute(ctx context.Context, in UpdateVariantInput) (*domain.Product, error) {
	return mutateProduct(ctx, uc.tx, uc.repo, uc.events, uc.cache, in.ProductID, func(p *domain.Product) error {
		// Variant không thuộc sản phẩm này thì domain trả UNKNOWN_VARIANT —
		// không có đường nào sửa variant của sản phẩm A qua URL của sản phẩm B.
		return p.UpdateVariant(in.VariantID, in.Price, in.Options, in.Status, in.Position)
	})
}

// mutateProduct là khuôn chung của mọi lệnh sửa variant.
//
// Đọc aggregate NẰM TRONG closure: TxManager chạy lại closure khi gặp lỗi tuần
// tự hóa, và lần chạy lại phải sửa trên dữ liệu mới, phát lại sự kiện của
// chính lần đó.
func mutateProduct(ctx context.Context, tx TxManager, repo ProductRepository,
	events EventPublisher, cache Cache, id uuid.UUID, mutate func(*domain.Product) error) (*domain.Product, error) {

	var updated *domain.Product
	if err := tx.Run(ctx, func(ctx context.Context) error {
		p, err := repo.ByID(ctx, id)
		if err != nil {
			return err
		}
		if err := mutate(p); err != nil {
			return err
		}
		if err := repo.Save(ctx, p); err != nil {
			return err
		}
		updated = p
		return events.Publish(ctx, p.PullEvents()...)
	}); err != nil {
		return nil, err
	}
	// Sau commit, WithoutCancel — xem CreateProduct. Giá "từ" vừa đổi, nên trang
	// chi tiết đang cache phải đọc lại.
	cache.Invalidate(context.WithoutCancel(ctx), KeyProductSlug(updated.Slug))
	return updated, nil
}
