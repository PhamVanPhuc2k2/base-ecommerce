package app

import (
	"context"

	"base-ecommerce/api/internal/catalog/domain"
)

const (
	DefaultLimit = 24
	MaxLimit     = 100
	MaxPage      = 200
)

type ListProductsInput struct {
	CategorySlug string
	BrandSlug    string
	PriceMin     *string
	PriceMax     *string
	Attributes   map[string]string
	Sort         SortOption
	Page         int
	Limit        int
}

type ListProductsResult struct {
	Items []*domain.Product
	Total int
	Page  int
	Limit int
}

type ListProducts struct {
	repo ProductRepository
	tree *GetCategoryTree
}

func NewListProducts(repo ProductRepository, tree *GetCategoryTree) *ListProducts {
	return &ListProducts{repo: repo, tree: tree}
}

func (uc *ListProducts) Execute(ctx context.Context, in ListProductsInput) (*ListProductsResult, error) {
	page, limit := in.Page, in.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	// Chặn offset sâu: Postgres phải quét và vứt bỏ offset dòng, nên trang 5000
	// sẽ giết database. Google cũng không index sâu vậy — bot cào giá thì có.
	if page > MaxPage {
		return nil, domain.ErrPageTooDeep
	}

	// sort là enum đóng. Bỏ qua giá trị lạ và im lặng dùng mặc định sẽ che mất
	// lỗi phía client; trả 400 để họ sửa.
	switch in.Sort {
	case "":
		in.Sort = SortNewest
	case SortNewest, SortPriceAsc, SortPriceDesc:
	default:
		return nil, domain.ErrInvalidSort
	}

	f := ListFilter{
		BrandSlug:  in.BrandSlug,
		Attributes: in.Attributes,
		Sort:       in.Sort,
		Page:       page,
		Limit:      limit,
	}

	if in.CategorySlug != "" {
		tree, err := uc.tree.Tree(ctx)
		if err != nil {
			return nil, err
		}
		cat, ok := tree.BySlug(in.CategorySlug)
		if !ok {
			return nil, domain.ErrCategoryNotFound
		}
		// Giải ID con cháu TRONG GO từ cây đã cache — đây là lý do chọn
		// adjacency list thay vì ltree hay closure table.
		f.CategoryIDs = tree.DescendantIDs(cat.ID)
	}

	if in.PriceMin != nil {
		m, err := domain.NewMoney(*in.PriceMin, "VND")
		if err != nil {
			return nil, err
		}
		d := m.Decimal()
		f.PriceMin = &d
	}
	if in.PriceMax != nil {
		m, err := domain.NewMoney(*in.PriceMax, "VND")
		if err != nil {
			return nil, err
		}
		d := m.Decimal()
		f.PriceMax = &d
	}

	items, total, err := uc.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return &ListProductsResult{Items: items, Total: total, Page: page, Limit: limit}, nil
}
