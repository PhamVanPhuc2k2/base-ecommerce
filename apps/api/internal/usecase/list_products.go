package usecase

import (
	"context"

	"base-ecommerce/api/internal/domain"
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
	// Attributes là mọi attr.<code>=<v> client gửi. Use case tự chia vào thuộc
	// tính sản phẩm hay tùy chọn biến thể theo định nghĩa của danh mục.
	Attributes map[string]string
	Sort       SortOption
	Page       int
	Limit      int
}

type ListProductsResult struct {
	Items []*domain.Product
	Total int
	Page  int
	Limit int
}

type ListProducts struct {
	repo    ProductRepository
	tree    *GetCategoryTree
	cache   Cache
	schemas *AttributeSchemas
}

func NewListProducts(repo ProductRepository, tree *GetCategoryTree, cache Cache, schemas *AttributeSchemas) *ListProducts {
	return &ListProducts{repo: repo, tree: tree, cache: cache, schemas: schemas}
}

func (uc *ListProducts) Execute(ctx context.Context, in ListProductsInput) (*ListProductsResult, error) {
	f, _, err := buildFilter(ctx, uc.tree, uc.schemas, in)
	if err != nil {
		return nil, err
	}

	items, err := uc.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}

	// Bỏ hẳn bước đếm khi trang 1 chưa đầy: số sản phẩm trên trang CHÍNH LÀ số
	// đếm. Trường hợp phổ biến nhất (lọc hẹp) nhờ vậy không tốn câu đếm nào.
	total := len(items)
	if f.Page > 1 || len(items) == f.Limit {
		total, err = uc.cache.ProductCount(ctx, KeyProductCount(f), TTLProductCount,
			func(ctx context.Context) (int, error) { return uc.repo.Count(ctx, f) })
		if err != nil {
			return nil, err
		}
	}
	return &ListProductsResult{Items: items, Total: total, Page: f.Page, Limit: f.Limit}, nil
}

// FacetGroup là một thuộc tính lọc được cùng số sản phẩm theo từng giá trị.
type FacetGroup struct {
	Entry  domain.SchemaEntry
	Values []domain.FacetCount
}

type ListFacets struct {
	repo    ProductRepository
	tree    *GetCategoryTree
	cache   Cache
	schemas *AttributeSchemas
}

func NewListFacets(repo ProductRepository, tree *GetCategoryTree, cache Cache, schemas *AttributeSchemas) *ListFacets {
	return &ListFacets{repo: repo, tree: tree, cache: cache, schemas: schemas}
}

// Execute nhận ĐÚNG bộ lọc của ListProducts, nên số cạnh mỗi giá trị là số sản
// phẩm sẽ thấy khi bấm thêm giá trị đó (đếm "conjunctive", đặc tả P1.3 mục 2.7).
// Không có danh mục thì trả rỗng: thuộc tính thuộc về danh mục.
func (uc *ListFacets) Execute(ctx context.Context, in ListProductsInput) ([]FacetGroup, error) {
	f, schema, err := buildFilter(ctx, uc.tree, uc.schemas, in)
	if err != nil || schema == nil {
		return nil, err
	}

	var productCodes, variantCodes []string
	var groups []FacetGroup
	for _, e := range schema.Entries() {
		if !e.Def.Filterable {
			continue
		}
		groups = append(groups, FacetGroup{Entry: e})
		if e.Def.Variant {
			variantCodes = append(variantCodes, e.Def.Code)
		} else {
			productCodes = append(productCodes, e.Def.Code)
		}
	}
	if len(groups) == 0 {
		return nil, nil
	}

	counts, err := uc.cache.ProductFacets(ctx, KeyProductFacets(f), TTLFacets,
		func(ctx context.Context) (domain.FacetCounts, error) {
			return uc.repo.Facets(ctx, f, productCodes, variantCodes)
		})
	if err != nil {
		return nil, err
	}
	byCode := map[string][]domain.FacetCount{}
	for _, c := range counts {
		byCode[c.Code] = append(byCode[c.Code], c)
	}
	for i := range groups {
		groups[i].Values = byCode[groups[i].Entry.Def.Code]
	}
	return groups, nil
}

// buildFilter chuẩn hóa tham số thành ListFilter — dùng chung cho danh sách và
// facet, để hai bên KHÔNG BAO GIỜ hiểu attr.* khác nhau. Trả kèm schema của
// danh mục (nil khi không lọc theo danh mục).
func buildFilter(ctx context.Context, trees *GetCategoryTree, schemas *AttributeSchemas,
	in ListProductsInput) (ListFilter, *domain.Schema, error) {

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
		return ListFilter{}, nil, domain.ErrPageTooDeep
	}

	// sort là enum đóng. Bỏ qua giá trị lạ và im lặng dùng mặc định sẽ che mất
	// lỗi phía client; trả 400 để họ sửa.
	switch in.Sort {
	case "":
		in.Sort = SortNewest
	case SortNewest, SortPriceAsc, SortPriceDesc:
	default:
		return ListFilter{}, nil, domain.ErrInvalidSort
	}

	f := ListFilter{
		BrandSlug:      in.BrandSlug,
		Attributes:     map[string]string{},
		VariantOptions: map[string]string{},
		Sort:           in.Sort,
		Page:           page,
		Limit:          limit,
	}

	var schema *domain.Schema
	if in.CategorySlug != "" {
		tree, err := trees.Tree(ctx)
		if err != nil {
			return ListFilter{}, nil, err
		}
		cat, ok := tree.BySlug(in.CategorySlug)
		if !ok {
			return ListFilter{}, nil, domain.ErrCategoryNotFound
		}
		// Giải ID con cháu TRONG GO từ cây đã cache — đây là lý do chọn
		// adjacency list thay vì ltree hay closure table.
		f.CategoryIDs = tree.DescendantIDs(cat.ID)
		schema, err = schemas.For(ctx, cat.ID)
		if err != nil {
			return ListFilter{}, nil, err
		}
	}

	// attr.<code> là thuộc tính biến thể → lọc trong variant active; còn lại
	// (kể cả khóa không có định nghĩa, và khi không lọc theo danh mục) → lọc
	// products.attributes như P0. Khóa lạ KHÔNG bị từ chối ở đây: link cũ đã
	// được index vẫn phải mở được, chỉ là ra ít kết quả hơn.
	for k, v := range in.Attributes {
		if schema != nil {
			if e, ok := schema.Lookup(k); ok && e.Def.Variant {
				f.VariantOptions[k] = v
				continue
			}
		}
		f.Attributes[k] = v
	}

	if in.PriceMin != nil {
		m, err := domain.NewMoney(*in.PriceMin, "VND")
		if err != nil {
			return ListFilter{}, nil, err
		}
		d := m.Decimal()
		f.PriceMin = &d
	}
	if in.PriceMax != nil {
		m, err := domain.NewMoney(*in.PriceMax, "VND")
		if err != nil {
			return ListFilter{}, nil, err
		}
		d := m.Decimal()
		f.PriceMax = &d
	}
	return f, schema, nil
}
