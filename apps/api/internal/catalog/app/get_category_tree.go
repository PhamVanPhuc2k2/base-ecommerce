package app

import (
	"context"

	"base-ecommerce/api/internal/catalog/domain"
)

type GetCategoryTree struct {
	repo  CategoryRepository
	cache Cache
}

func NewGetCategoryTree(repo CategoryRepository, cache Cache) *GetCategoryTree {
	return &GetCategoryTree{repo: repo, cache: cache}
}

// Tree trả cây danh mục đã dựng. Danh sách phẳng được cache; việc dựng cây làm
// lại mỗi lần vì nó chỉ là vài trăm phần tử trong bộ nhớ, rẻ hơn nhiều so với
// việc serialize cả cây có con trỏ.
func (uc *GetCategoryTree) Tree(ctx context.Context) (*domain.Tree, error) {
	cats, err := uc.cache.CategoryTree(ctx, TTLCategoryTree,
		func(ctx context.Context) ([]*domain.Category, error) {
			return uc.repo.All(ctx)
		})
	if err != nil {
		return nil, err
	}
	return domain.NewTree(cats), nil
}
