package httpapi

import (
	"net/http"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type mediaDTO struct {
	ID          uuid.UUID `json:"id"`
	Key         string    `json:"key"` // giá trị đặt vào Product.images
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func toMediaDTO(m *domain.Media) mediaDTO {
	return mediaDTO{ID: m.ID, Key: m.Key, ContentType: m.ContentType, Size: m.Size,
		Status: string(m.Status), CreatedAt: m.CreatedAt}
}

type uploadDTO struct {
	URL    string            `json:"url"`
	Fields map[string]string `json:"fields"`
}

type createMediaRequest struct {
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// CreateMedia cấp URL upload. File KHÔNG đi qua API: client POST thẳng lên
// storage, API không phải nhận hay giữ 10 MB trong bộ nhớ cho mỗi ảnh.
func (h *Handler) CreateMedia(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[createMediaRequest](w, r)
	if err != nil {
		return err
	}
	m, up, err := h.uc.Media.Create(r.Context(), req.ContentType, req.Size)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusCreated, map[string]any{
		"media":  toMediaDTO(m),
		"upload": uploadDTO{URL: up.URL, Fields: up.Fields},
	})
}

func (h *Handler) CompleteMedia(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownMedia
	}
	m, err := h.uc.Media.Complete(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toMediaDTO(m))
}
