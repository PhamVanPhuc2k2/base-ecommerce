package httpapi

import (
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// Mount gắn toàn bộ route của catalog vào router cha.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/products", httpx.Wrap(h.ListProducts))
	r.Get("/products/{slug}", httpx.Wrap(h.GetProduct))
	r.Get("/categories", httpx.Wrap(h.GetCategories))

	r.Route("/admin", func(r chi.Router) {
		r.Use(RequireAdminKey(h.adminKey))
		r.Post("/products", httpx.Wrap(h.CreateProduct))
		r.Patch("/products/{id}", httpx.Wrap(h.UpdateProduct))
		r.Post("/products/{id}/publish", httpx.Wrap(h.PublishProduct))
	})
}
