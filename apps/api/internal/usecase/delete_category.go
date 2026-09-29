package usecase

import (
	"context"

	"github.com/google/uuid"
)

type DeleteCategory struct {
	tx    TxManager
	repo  CategoryRepository
	cache Cache
}

func NewDeleteCategory(tx TxManager, repo CategoryRepository, cache Cache) *DeleteCategory {
	return &DeleteCategory{tx: tx, repo: repo, cache: cache}
}

// Execute xóa danh mục rỗng. Còn con hay còn sản phẩm thì khóa ngoại RESTRICT
// chặn và repository map ra CATEGORY_HAS_CHILDREN / CATEGORY_HAS_PRODUCTS —
// không kiểm trước bằng SELECT, vì kiểm-rồi-xóa là hai bước và khóa ngoại là
// thứ duy nhất đúng kể cả khi có ai chèn con vào giữa hai bước đó.
func (uc *DeleteCategory) Execute(ctx context.Context, id uuid.UUID) error {
	if err := uc.tx.Run(ctx, func(ctx context.Context) error {
		if err := uc.repo.LockForWrite(ctx); err != nil {
			return err
		}
		return uc.repo.Delete(ctx, id)
	}); err != nil {
		return err
	}
	uc.cache.Invalidate(context.WithoutCancel(ctx), KeyCategoryTree())
	return nil
}
