package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"base-ecommerce/api/internal/catalog/app"
	"base-ecommerce/api/internal/catalog/domain"
	"base-ecommerce/api/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	create  *app.CreateProduct
	update  *app.UpdateProduct
	publish *app.PublishProduct
	get     *app.GetProduct
	list    *app.ListProducts
	tree    *app.GetCategoryTree
}

func NewHandler(create *app.CreateProduct, update *app.UpdateProduct,
	publish *app.PublishProduct, get *app.GetProduct,
	list *app.ListProducts, tree *app.GetCategoryTree) *Handler {
	return &Handler{create: create, update: update, publish: publish,
		get: get, list: list, tree: tree}
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) error {
	p, err := h.get.BySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toProductDTO(p))
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()

	in := app.ListProductsInput{
		CategorySlug: q.Get("category"),
		BrandSlug:    q.Get("brand"),
		Sort:         app.SortOption(q.Get("sort")),
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
			return domain.ErrInvalidPagination
		}
		in.Page = v
	}
	if raw := q.Get("limit"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return domain.ErrInvalidPagination
		}
		in.Limit = v
	}
	// Lọc thuộc tính qua tiền tố attr.: ?attr.ram=16GB&attr.socket=AM5
	for k, vals := range q {
		if after, ok := strings.CutPrefix(k, "attr."); ok && len(vals) > 0 && after != "" {
			in.Attributes[after] = vals[0]
		}
	}

	res, err := h.list.Execute(r.Context(), in)
	if err != nil {
		return err
	}

	items := make([]productDTO, 0, len(res.Items))
	for _, p := range res.Items {
		items = append(items, toProductDTO(p))
	}

	totalPages := (res.Total + res.Limit - 1) / res.Limit

	// has_next phải tôn trọng trần MaxPage, nếu không hợp đồng dẫn client vào
	// tường: ở trang 200 với 8334 trang, has_next là true, client bấm "trang
	// sau" và nhận 400 PAGE_TOO_DEEP. total_pages vẫn báo con số thật để client
	// biết còn bao nhiêu dữ liệu — nhưng cách đi tới phần còn lại là lọc hẹp
	// lại, không phải lật tiếp.
	hasNext := res.Page < totalPages && res.Page < app.MaxPage

	return httpx.JSON(w, http.StatusOK, listResponse{
		Data: items,
		Meta: listMeta{
			Page: res.Page, Limit: res.Limit, Total: res.Total,
			TotalPages: totalPages,
			MaxPage:    app.MaxPage,
			HasNext:    hasNext,
			HasPrev:    res.Page > 1,
		},
	})
}

func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) error {
	t, err := h.tree.Tree(r.Context())
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
	price, err := domain.NewMoney(req.Price, req.Currency)
	if err != nil {
		return err
	}

	p, err := h.create.Execute(r.Context(), app.CreateProductInput{
		SKU: req.SKU, Name: req.Name, ShortDescription: req.ShortDescription,
		CategoryID: req.CategoryID, BrandID: req.BrandID, Price: price,
		Attributes: req.Attributes, Images: req.Images,
	})
	if err != nil {
		return err
	}

	w.Header().Set("Location", "/api/v1/products/"+p.Slug)
	return httpx.JSON(w, http.StatusCreated, toProductDTO(p))
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
	// Chỉ dựng Money khi client thật sự gửi giá. Bản trước gọi NewMoney vô
	// điều kiện, nên một lệnh PATCH chỉ đổi tên cũng ăn 422 INVALID_PRICE —
	// tức là không PATCH nổi một trường nào nếu không gửi kèm giá.
	var price *domain.Money
	if req.Price != nil {
		currency := domain.SupportedCurrency
		if req.Currency != nil {
			currency = *req.Currency
		}
		m, mErr := domain.NewMoney(*req.Price, currency)
		if mErr != nil {
			return mErr
		}
		price = &m
	}

	p, err := h.update.Execute(r.Context(), app.UpdateProductInput{
		ID: id, Name: req.Name, ShortDescription: req.ShortDescription,
		Price: price, Attributes: req.Attributes, Images: req.Images,
	})
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toProductDTO(p))
}

func (h *Handler) PublishProduct(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrProductNotFound
	}
	p, err := h.publish.Execute(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toProductDTO(p))
}
