package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/httpx"
)

type sitemapEntryDTO struct {
	Slug      string    `json:"slug"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *Handler) SitemapProducts(w http.ResponseWriter, r *http.Request) error {
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return domain.ErrInvalidPagination
		}
		page = v
	}
	res, err := h.uc.Sitemap.Execute(r.Context(), page)
	if err != nil {
		return err
	}
	items := make([]sitemapEntryDTO, 0, len(res.Items))
	for _, e := range res.Items {
		items = append(items, sitemapEntryDTO{Slug: e.Slug, UpdatedAt: e.UpdatedAt})
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]int{
			"page": res.Page, "page_size": usecase.SitemapPageSize,
			"total": res.Total, "total_pages": res.TotalPages,
		},
	})
}
