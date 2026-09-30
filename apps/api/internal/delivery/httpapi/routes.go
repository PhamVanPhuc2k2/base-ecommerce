package httpapi

import (
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// Mount gắn toàn bộ route của catalog vào router cha.
func (h *Handler) Mount(r chi.Router) {
	r.Get("/products", httpx.Wrap(h.ListProducts))
	// Đoạn tĩnh "facets" thắng tham số {slug} trong chi, nên route này phải
	// khai được dù nằm trước hay sau. Cái giá: một sản phẩm có slug đúng bằng
	// "facets" sẽ không mở được bằng /products/facets — chấp nhận, slug sinh
	// từ tên sản phẩm và không ai đặt tên sản phẩm là "Facets".
	r.Get("/products/facets", httpx.Wrap(h.ListFacets))
	r.Get("/products/{slug}", httpx.Wrap(h.GetProduct))
	r.Get("/categories", httpx.Wrap(h.GetCategories))
	r.Get("/brands", httpx.Wrap(h.ListBrands))
	r.Get("/attributes", httpx.Wrap(h.ListAttributes))
	r.Get("/categories/{slug}/attributes", httpx.Wrap(h.GetCategoryAttributes))

	r.Route("/admin", func(r chi.Router) {
		r.Use(RequireAdminKey(h.adminKey))
		r.Post("/products", httpx.Wrap(h.CreateProduct))
		r.Patch("/products/{id}", httpx.Wrap(h.UpdateProduct))
		r.Post("/products/{id}/publish", httpx.Wrap(h.PublishProduct))
		r.Post("/products/{id}/variants", httpx.Wrap(h.AddVariant))
		r.Patch("/products/{id}/variants/{variantId}", httpx.Wrap(h.UpdateVariant))

		r.Post("/categories", httpx.Wrap(h.CreateCategory))
		r.Patch("/categories/{id}", httpx.Wrap(h.UpdateCategory))
		r.Delete("/categories/{id}", httpx.Wrap(h.DeleteCategory))
		r.Put("/categories/{id}/attributes", httpx.Wrap(h.SetCategoryAttributes))

		r.Post("/media", httpx.Wrap(h.CreateMedia))
		r.Post("/media/{id}/complete", httpx.Wrap(h.CompleteMedia))

		r.Post("/attributes", httpx.Wrap(h.CreateAttribute))
		r.Patch("/attributes/{id}", httpx.Wrap(h.UpdateAttribute))
		r.Delete("/attributes/{id}", httpx.Wrap(h.DeleteAttribute))

		r.Post("/brands", httpx.Wrap(h.CreateBrand))
		r.Patch("/brands/{id}", httpx.Wrap(h.UpdateBrand))
		r.Delete("/brands/{id}", httpx.Wrap(h.DeleteBrand))
	})
}
