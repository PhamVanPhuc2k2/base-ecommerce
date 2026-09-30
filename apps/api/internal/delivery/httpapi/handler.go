package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Usecases gom mọi use case mà handler gọi. Struct có tên trường thay cho một
// hàm dựng nhận 13 tham số cùng kiểu con trỏ — đổi chỗ hai tham số cùng kiểu
// thì trình biên dịch không bắt được, còn đổi chỗ hai trường có tên thì có.
type Usecases struct {
	CreateProduct  *usecase.CreateProduct
	UpdateProduct  *usecase.UpdateProduct
	PublishProduct *usecase.PublishProduct
	GetProduct     *usecase.GetProduct
	ListProducts   *usecase.ListProducts
	CategoryTree   *usecase.GetCategoryTree
	CreateCategory *usecase.CreateCategory
	UpdateCategory *usecase.UpdateCategory
	DeleteCategory *usecase.DeleteCategory
	AddVariant     *usecase.AddVariant
	ListFacets     *usecase.ListFacets
	ListAttributes *usecase.ListAttributes
	CategoryAttrs  *usecase.CategoryAttributes
	AttributeAdmin *usecase.AttributeAdmin
	Media          *usecase.MediaUploads
	Sitemap        *usecase.SitemapProducts
	Auth           *usecase.Auth
	Verification   *usecase.Verification
	AddressBook    *usecase.AddressBook
	Authorizer     *usecase.Authorizer
	RoleAdmin      *usecase.RoleAdmin
	UpdateVariant  *usecase.UpdateVariant
	ListBrands     *usecase.ListBrands
	CreateBrand    *usecase.CreateBrand
	UpdateBrand    *usecase.UpdateBrand
	DeleteBrand    *usecase.DeleteBrand
}

type Handler struct {
	verifier TokenVerifier
	uc       Usecases
}

func NewHandler(verifier TokenVerifier, uc Usecases) *Handler {
	return &Handler{verifier: verifier, uc: uc}
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) error {
	p, err := h.uc.GetProduct.BySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toProductDTO(p, publicVariants))
}

// parseListInput đọc bộ lọc từ query string — dùng chung cho /products và
// /products/facets, để facet đếm trên ĐÚNG tập kết quả mà danh sách trả về.
func parseListInput(r *http.Request) (usecase.ListProductsInput, error) {
	q := r.URL.Query()

	in := usecase.ListProductsInput{
		CategorySlug: q.Get("category"),
		BrandSlug:    q.Get("brand"),
		Sort:         usecase.SortOption(q.Get("sort")),
		Attributes:   map[string]string{},
	}
	if v := q.Get("price_min"); v != "" {
		in.PriceMin = &v
	}
	if v := q.Get("price_max"); v != "" {
		in.PriceMax = &v
	}
	// Giá trị không phải số phải báo lỗi chứ không im lặng dùng mặc định —
	// cùng chính sách với `sort`. Bỏ qua âm thầm che mất lỗi phía client: họ
	// gửi page=abc, nhận trang 1, và tưởng danh sách chỉ có bấy nhiêu.
	if raw := q.Get("page"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return in, domain.ErrInvalidPagination
		}
		in.Page = v
	}
	if raw := q.Get("limit"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return in, domain.ErrInvalidPagination
		}
		in.Limit = v
	}
	// Lọc thuộc tính qua tiền tố attr.: ?attr.ram=16GB&attr.socket=AM5
	for k, vals := range q {
		if after, ok := strings.CutPrefix(k, "attr."); ok && len(vals) > 0 && after != "" {
			in.Attributes[after] = vals[0]
		}
	}

	return in, nil
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) error {
	in, err := parseListInput(r)
	if err != nil {
		return err
	}
	res, err := h.uc.ListProducts.Execute(r.Context(), in)
	if err != nil {
		return err
	}

	items := make([]productDTO, 0, len(res.Items))
	for _, p := range res.Items {
		items = append(items, toProductDTO(p, publicVariants))
	}

	totalPages := (res.Total + res.Limit - 1) / res.Limit

	// has_next phải tôn trọng trần MaxPage, nếu không hợp đồng dẫn client vào
	// tường: ở trang 200 với 8334 trang, has_next là true, client bấm "trang
	// sau" và nhận 400 PAGE_TOO_DEEP. total_pages vẫn báo con số thật để client
	// biết còn bao nhiêu dữ liệu — nhưng cách đi tới phần còn lại là lọc hẹp
	// lại, không phải lật tiếp.
	hasNext := res.Page < totalPages && res.Page < usecase.MaxPage

	return httpx.JSON(w, http.StatusOK, listResponse{
		Data: items,
		Meta: listMeta{
			Page: res.Page, Limit: res.Limit, Total: res.Total,
			TotalPages: totalPages,
			MaxPage:    usecase.MaxPage,
			HasNext:    hasNext,
			HasPrev:    res.Page > 1,
		},
	})
}

func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) error {
	t, err := h.uc.CategoryTree.Tree(r.Context())
	if err != nil {
		return err
	}
	roots := t.Roots()
	out := make([]categoryDTO, 0, len(roots))
	for _, c := range roots {
		out = append(out, toCategoryDTO(t, c))
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[createProductRequest](w, r)
	if err != nil {
		return err
	}
	variants := make([]domain.VariantInput, 0, len(req.Variants))
	for _, v := range req.Variants {
		in, err := v.toInput()
		if err != nil {
			return err
		}
		variants = append(variants, in)
	}

	p, err := h.uc.CreateProduct.Execute(r.Context(), usecase.CreateProductInput{
		Name: req.Name, ShortDescription: req.ShortDescription,
		CategoryID: req.CategoryID, BrandID: req.BrandID, Variants: variants,
		Attributes: req.Attributes, Images: req.Images,
	})
	if err != nil {
		return err
	}

	w.Header().Set("Location", "/api/v1/products/"+p.Slug)
	return httpx.JSON(w, http.StatusCreated, toProductDTO(p, adminVariants))
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrProductNotFound
	}
	req, err := httpx.Decode[updateProductRequest](w, r)
	if err != nil {
		return err
	}
	p, err := h.uc.UpdateProduct.Execute(r.Context(), usecase.UpdateProductInput{
		ID: id, Name: req.Name, ShortDescription: req.ShortDescription,
		Attributes: req.Attributes, Images: req.Images,
		CategoryID: req.CategoryID, BrandID: req.BrandID,
	})
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toProductDTO(p, adminVariants))
}

func (h *Handler) PublishProduct(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrProductNotFound
	}
	p, err := h.uc.PublishProduct.Execute(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toProductDTO(p, adminVariants))
}
