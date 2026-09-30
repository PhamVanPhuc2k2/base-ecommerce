package httpapi

import (
	"net/http"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type attributeDTO struct {
	ID         uuid.UUID `json:"id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Unit       string    `json:"unit"`
	Options    []string  `json:"options"`
	Filterable bool      `json:"filterable"`
	Variant    bool      `json:"variant"`
}

func toAttributeDTO(d *domain.AttributeDefinition) attributeDTO {
	return attributeDTO{ID: d.ID, Code: d.Code, Name: d.Name, Type: string(d.Type), Unit: d.Unit,
		Options: d.Options, Filterable: d.Filterable, Variant: d.Variant}
}

// categoryAttributeDTO là một thuộc tính trong tập HIỆU LỰC của danh mục — định
// nghĩa phẳng ra cùng cấu hình gán, để client không phải ghép hai danh sách.
type categoryAttributeDTO struct {
	attributeDTO
	Required  bool `json:"required"`
	Position  int  `json:"position"`
	Inherited bool `json:"inherited"`
}

type facetValueDTO struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type facetDTO struct {
	Code    string          `json:"code"`
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Unit    string          `json:"unit"`
	Variant bool            `json:"variant"`
	Values  []facetValueDTO `json:"values"`
}

type createAttributeRequest struct {
	Code       string   `json:"code"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Unit       string   `json:"unit"`
	Options    []string `json:"options"`
	Filterable bool     `json:"filterable"`
	Variant    bool     `json:"variant"`
}

// updateAttributeRequest KHÔNG có code, type, variant: chúng bất biến (đặc tả
// P1.3 mục 2.5). Gửi chúng lên trả 400 MALFORMED_REQUEST nhờ
// DisallowUnknownFields — báo to thay vì âm thầm bỏ qua.
type updateAttributeRequest struct {
	Name       *string  `json:"name"`
	Unit       *string  `json:"unit"`
	Options    []string `json:"options"`
	Filterable *bool    `json:"filterable"`
}

type setCategoryAttributesRequest struct {
	Attributes []struct {
		AttributeID uuid.UUID `json:"attribute_id"`
		Required    bool      `json:"required"`
		Position    int       `json:"position"`
	} `json:"attributes"`
}

func (h *Handler) ListAttributes(w http.ResponseWriter, r *http.Request) error {
	defs, err := h.uc.ListAttributes.Execute(r.Context())
	if err != nil {
		return err
	}
	out := make([]attributeDTO, 0, len(defs))
	for _, d := range defs {
		out = append(out, toAttributeDTO(d))
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) GetCategoryAttributes(w http.ResponseWriter, r *http.Request) error {
	entries, err := h.uc.CategoryAttrs.Execute(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		return err
	}
	out := make([]categoryAttributeDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, categoryAttributeDTO{attributeDTO: toAttributeDTO(e.Def),
			Required: e.Required, Position: e.Position, Inherited: e.Inherited})
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) ListFacets(w http.ResponseWriter, r *http.Request) error {
	in, err := parseListInput(r)
	if err != nil {
		return err
	}
	groups, err := h.uc.ListFacets.Execute(r.Context(), in)
	if err != nil {
		return err
	}
	out := make([]facetDTO, 0, len(groups))
	for _, g := range groups {
		vals := make([]facetValueDTO, 0, len(g.Values))
		for _, v := range g.Values {
			vals = append(vals, facetValueDTO{Value: v.Value, Count: v.Count})
		}
		d := g.Entry.Def
		out = append(out, facetDTO{Code: d.Code, Name: d.Name, Type: string(d.Type),
			Unit: d.Unit, Variant: d.Variant, Values: vals})
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) CreateAttribute(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[createAttributeRequest](w, r)
	if err != nil {
		return err
	}
	d, err := h.uc.AttributeAdmin.Create(r.Context(), usecase.CreateAttributeInput{
		Code: req.Code, Name: req.Name, Type: domain.AttributeType(req.Type), Unit: req.Unit,
		Options: req.Options, Filterable: req.Filterable, Variant: req.Variant,
	})
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusCreated, toAttributeDTO(d))
}

func (h *Handler) UpdateAttribute(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownAttribute
	}
	req, err := httpx.Decode[updateAttributeRequest](w, r)
	if err != nil {
		return err
	}
	d, err := h.uc.AttributeAdmin.Update(r.Context(), usecase.UpdateAttributeInput{
		ID: id, Name: req.Name, Unit: req.Unit, Options: req.Options, Filterable: req.Filterable,
	})
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toAttributeDTO(d))
}

func (h *Handler) DeleteAttribute(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownAttribute
	}
	if err := h.uc.AttributeAdmin.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) SetCategoryAttributes(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownCategory
	}
	req, err := httpx.Decode[setCategoryAttributesRequest](w, r)
	if err != nil {
		return err
	}
	assigns := make([]domain.CategoryAttribute, 0, len(req.Attributes))
	for _, a := range req.Attributes {
		assigns = append(assigns, domain.CategoryAttribute{
			AttributeID: a.AttributeID, Required: a.Required, Position: a.Position,
		})
	}
	if err := h.uc.AttributeAdmin.SetCategoryAttributes(r.Context(), id, assigns); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
