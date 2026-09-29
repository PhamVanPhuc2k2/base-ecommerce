package httpapi

import (
	"net/http"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler cho danh mục và thương hiệu (P1.1). Cùng khuôn với handler sản phẩm:
// parse → gọi use case → ghi response, không logic nghiệp vụ.

func (h *Handler) ListBrands(w http.ResponseWriter, r *http.Request) error {
	bs, err := h.uc.ListBrands.Execute(r.Context())
	if err != nil {
		return err
	}
	out := make([]brandDTO, 0, len(bs))
	for _, b := range bs {
		out = append(out, toBrandDTO(b))
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[createCategoryRequest](w, r)
	if err != nil {
		return err
	}
	c, err := h.uc.CreateCategory.Execute(r.Context(), usecase.CreateCategoryInput{
		Name: req.Name, Slug: req.Slug, ParentID: req.ParentID, Position: req.Position,
	})
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusCreated, toCategoryNodeDTO(c))
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		// id không phải uuid thì chắc chắn không có danh mục nào mang nó — 404
		// như mọi id lạ khác, không phải 400: client không cần biết định dạng id.
		return domain.ErrUnknownCategory
	}
	req, err := httpx.Decode[updateCategoryRequest](w, r)
	if err != nil {
		return err
	}
	c, err := h.uc.UpdateCategory.Execute(r.Context(), usecase.UpdateCategoryInput{
		ID: id, Name: req.Name, Slug: req.Slug, Position: req.Position,
		Parent: usecase.ParentChange{Set: req.ParentID.Set, ID: req.ParentID.Value},
	})
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toCategoryNodeDTO(c))
}

func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownCategory
	}
	if err := h.uc.DeleteCategory.Execute(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) CreateBrand(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[createBrandRequest](w, r)
	if err != nil {
		return err
	}
	b, err := h.uc.CreateBrand.Execute(r.Context(), req.Name, req.Slug)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusCreated, toBrandDTO(b))
}

func (h *Handler) UpdateBrand(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownBrand
	}
	req, err := httpx.Decode[updateBrandRequest](w, r)
	if err != nil {
		return err
	}
	b, err := h.uc.UpdateBrand.Execute(r.Context(), usecase.UpdateBrandInput{
		ID: id, Name: req.Name, Slug: req.Slug,
	})
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toBrandDTO(b))
}

func (h *Handler) DeleteBrand(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownBrand
	}
	if err := h.uc.DeleteBrand.Execute(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
