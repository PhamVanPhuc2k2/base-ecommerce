package httpapi

import (
	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
)

// Mount gắn toàn bộ route vào router cha.
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
	r.Get("/sitemap/products", httpx.Wrap(h.SitemapProducts))
	r.Get("/attributes", httpx.Wrap(h.ListAttributes))
	r.Get("/categories/{slug}/attributes", httpx.Wrap(h.GetCategoryAttributes))

	r.Post("/auth/register", httpx.Wrap(h.Register))
	r.Post("/auth/login", httpx.Wrap(h.Login))
	r.Post("/auth/refresh", httpx.Wrap(h.Refresh))
	r.Post("/auth/logout", httpx.Wrap(h.Logout))
	r.Post("/auth/password/forgot", httpx.Wrap(h.ForgotPassword))
	r.Post("/auth/password/reset", httpx.Wrap(h.ResetPassword))
	r.Group(func(r chi.Router) {
		r.Use(h.RequireAuth)
		r.Get("/me", httpx.Wrap(h.Me))
		r.Patch("/me", httpx.Wrap(h.UpdateMe))
		r.Get("/me/addresses", httpx.Wrap(h.ListAddresses))
		r.Post("/me/addresses", httpx.Wrap(h.CreateAddress))
		r.Patch("/me/addresses/{id}", httpx.Wrap(h.UpdateAddress))
		r.Delete("/me/addresses/{id}", httpx.Wrap(h.DeleteAddress))
		r.Post("/me/addresses/{id}/default", httpx.Wrap(h.SetDefaultAddress))
		r.Post("/auth/email/verification", httpx.Wrap(h.RequestEmailVerification))
		r.Post("/auth/email/verify", httpx.Wrap(h.VerifyEmail))
	})

	/*
		/admin/*: Bearer token (RequireAuth) RỒI MỚI tới quyền (RequirePermission).
		Thứ tự là cố ý: chưa biết là ai thì trả 401, không phải 403 — 403 cho
		người lạ là nói cho họ biết endpoint tồn tại và cần quyền gì. Đặc tả
		P2.2 mục 2.6.

		Thay cho X-Admin-Key (P0.2 → P2.1): một khóa chung cho mọi thao tác, không
		biết ai đã làm gì, lộ là mất tất cả.
	*/
	r.Route("/admin", func(r chi.Router) {
		r.Use(h.RequireAuth)

		r.Group(func(r chi.Router) {
			r.Use(h.RequirePermission(domain.PermCatalogProductsWrite))
			r.Post("/products", httpx.Wrap(h.CreateProduct))
			r.Patch("/products/{id}", httpx.Wrap(h.UpdateProduct))
			r.Post("/products/{id}/publish", httpx.Wrap(h.PublishProduct))
			r.Post("/products/{id}/variants", httpx.Wrap(h.AddVariant))
			r.Patch("/products/{id}/variants/{variantId}", httpx.Wrap(h.UpdateVariant))
		})

		r.Group(func(r chi.Router) {
			r.Use(h.RequirePermission(domain.PermCatalogTaxonomyWrite))
			r.Post("/categories", httpx.Wrap(h.CreateCategory))
			r.Patch("/categories/{id}", httpx.Wrap(h.UpdateCategory))
			r.Delete("/categories/{id}", httpx.Wrap(h.DeleteCategory))
			r.Put("/categories/{id}/attributes", httpx.Wrap(h.SetCategoryAttributes))

			r.Post("/attributes", httpx.Wrap(h.CreateAttribute))
			r.Patch("/attributes/{id}", httpx.Wrap(h.UpdateAttribute))
			r.Delete("/attributes/{id}", httpx.Wrap(h.DeleteAttribute))

			r.Post("/brands", httpx.Wrap(h.CreateBrand))
			r.Patch("/brands/{id}", httpx.Wrap(h.UpdateBrand))
			r.Delete("/brands/{id}", httpx.Wrap(h.DeleteBrand))
		})

		r.Group(func(r chi.Router) {
			r.Use(h.RequirePermission(domain.PermMediaUpload))
			r.Post("/media", httpx.Wrap(h.CreateMedia))
			r.Post("/media/{id}/complete", httpx.Wrap(h.CompleteMedia))
		})

		r.Group(func(r chi.Router) {
			r.Use(h.RequirePermission(domain.PermInventoryManage))
			r.Get("/locations", httpx.Wrap(h.ListLocations))
			r.Post("/locations", httpx.Wrap(h.CreateLocation))
			r.Patch("/locations/{id}", httpx.Wrap(h.UpdateLocation))
			r.Post("/stock/movements", httpx.Wrap(h.RecordStockMovement))
			r.Get("/stock/movements", httpx.Wrap(h.ListStockMovements))
			r.Get("/variants/{id}/stock", httpx.Wrap(h.VariantStock))
			// P4 gọi use case trực tiếp; các route này để vận hành và kiểm chứng.
			r.Post("/reservations", httpx.Wrap(h.CreateReservation))
			r.Get("/reservations/{id}", httpx.Wrap(h.GetReservation))
			r.Post("/reservations/{id}/release", httpx.Wrap(h.ReleaseReservation))
			r.Post("/reservations/{id}/commit", httpx.Wrap(h.CommitReservation))
		})

		r.Group(func(r chi.Router) {
			r.Use(h.RequirePermission(domain.PermIAMRolesManage))
			r.Get("/permissions", httpx.Wrap(h.ListPermissions))
			r.Get("/roles", httpx.Wrap(h.ListRoles))
			r.Post("/roles", httpx.Wrap(h.CreateRole))
			r.Patch("/roles/{id}", httpx.Wrap(h.UpdateRole))
			r.Delete("/roles/{id}", httpx.Wrap(h.DeleteRole))
			r.Put("/users/{id}/roles", httpx.Wrap(h.SetUserRoles))
		})
	})
}
