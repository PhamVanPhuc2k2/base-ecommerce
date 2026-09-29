package usecase

import (
	"context"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// Bốn use case thương hiệu chung một file: mỗi cái vài dòng, cùng một khuôn
// (ghi trong transaction → xóa cache sau commit), và tách ra bốn file chỉ làm
// người đọc phải nhảy qua lại để thấy chúng giống hệt nhau.

type ListBrands struct {
	repo  BrandRepository
	cache Cache
}

func NewListBrands(repo BrandRepository, cache Cache) *ListBrands {
	return &ListBrands{repo: repo, cache: cache}
}

func (uc *ListBrands) Execute(ctx context.Context) (domain.Brands, error) {
	return uc.cache.Brands(ctx, TTLBrands, uc.repo.All)
}

type CreateBrand struct {
	tx    TxManager
	repo  BrandRepository
	cache Cache
}

func NewCreateBrand(tx TxManager, repo BrandRepository, cache Cache) *CreateBrand {
	return &CreateBrand{tx: tx, repo: repo, cache: cache}
}

func (uc *CreateBrand) Execute(ctx context.Context, name, slug string) (*domain.Brand, error) {
	b, err := domain.NewBrand(name, slug)
	if err != nil {
		return nil, err
	}
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		return uc.repo.Insert(ctx, b)
	}); err != nil {
		return nil, err
	}
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyBrands())
	return b, nil
}

type UpdateBrandInput struct {
	ID   uuid.UUID
	Name *string
	Slug *string
}

type UpdateBrand struct {
	tx    TxManager
	repo  BrandRepository
	cache Cache
}

func NewUpdateBrand(tx TxManager, repo BrandRepository, cache Cache) *UpdateBrand {
	return &UpdateBrand{tx: tx, repo: repo, cache: cache}
}

func (uc *UpdateBrand) Execute(ctx context.Context, in UpdateBrandInput) (*domain.Brand, error) {
	var updated *domain.Brand
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		b, err := uc.repo.ByID(ctx, in.ID) // FOR UPDATE
		if err != nil {
			return err
		}
		if in.Name != nil {
			if err := b.Rename(*in.Name); err != nil {
				return err
			}
		}
		if in.Slug != nil {
			if err := b.SetSlug(*in.Slug); err != nil {
				return err
			}
		}
		if err := uc.repo.Update(ctx, b); err != nil {
			return err
		}
		updated = b
		return nil
	}); err != nil {
		return nil, err
	}
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyBrands())
	return updated, nil
}

type DeleteBrand struct {
	tx    TxManager
	repo  BrandRepository
	cache Cache
}

func NewDeleteBrand(tx TxManager, repo BrandRepository, cache Cache) *DeleteBrand {
	return &DeleteBrand{tx: tx, repo: repo, cache: cache}
}

func (uc *DeleteBrand) Execute(ctx context.Context, id uuid.UUID) error {
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		return uc.repo.Delete(ctx, id)
	}); err != nil {
		return err
	}
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyBrands())
	return nil
}
