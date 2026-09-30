package httpapi

import (
	"net/http"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Lệnh ghi variant trả về CẢ sản phẩm, không chỉ variant: giá "từ" của sản
// phẩm đổi theo, và client cần thấy con số đó ngay mà không phải gọi lại.

func (h *Handler) AddVariant(w http.ResponseWriter, r *http.Request) error {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrProductNotFound
	}
	req, err := httpx.Decode[createVariantRequest](w, r)
	if err != nil {
		return err
	}
	in, err := req.toInput()
	if err != nil {
		return err
	}
	p, err := h.uc.AddVariant.Execute(r.Context(), productID, in)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusCreated, toProductDTO(p, adminVariants))
}

func (h *Handler) UpdateVariant(w http.ResponseWriter, r *http.Request) error {
	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrProductNotFound
	}
	variantID, err := uuid.Parse(chi.URLParam(r, "variantId"))
	if err != nil {
		return domain.ErrUnknownVariant
	}
	req, err := httpx.Decode[updateVariantRequest](w, r)
	if err != nil {
		return err
	}

	in := usecase.UpdateVariantInput{
		ProductID: productID, VariantID: variantID,
		Options: req.Options, Position: req.Position,
	}
	// Chỉ dựng Money khi client thật sự gửi giá — bài học P0.2: gọi NewMoney vô
	// điều kiện thì PATCH chỉ đổi trạng thái cũng ăn 422 INVALID_PRICE.
	if req.Price != nil {
		currency := domain.SupportedCurrency
		if req.Currency != nil {
			currency = *req.Currency
		}
		m, err := domain.NewMoney(*req.Price, currency)
		if err != nil {
			return err
		}
		in.Price = &m
	}
	if req.Status != nil {
		s := domain.VariantStatus(*req.Status)
		in.Status = &s
	}

	p, err := h.uc.UpdateVariant.Execute(r.Context(), in)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toProductDTO(p, adminVariants))
}
