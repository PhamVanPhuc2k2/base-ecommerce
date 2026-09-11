// Package catalog lắp ráp module. Đây là nơi DUY NHẤT biết cả domain, app và
// adapter — mọi package khác chỉ thấy đúng phần nó cần.
package catalog

import (
	"log/slog"

	"base-ecommerce/api/internal/catalog/adapter/httpapi"
	"base-ecommerce/api/internal/catalog/adapter/outboxpub"
	"base-ecommerce/api/internal/catalog/adapter/pgstore"
	"base-ecommerce/api/internal/catalog/adapter/rediscache"
	"base-ecommerce/api/internal/catalog/app"
	"base-ecommerce/api/internal/outbox"
	"base-ecommerce/api/internal/platform/postgres"
	platformredis "base-ecommerce/api/internal/platform/redis"

	"github.com/go-chi/chi/v5"
)

type Module struct {
	handler  *httpapi.Handler
	adminKey string
}

func New(db *postgres.Manager, cache *platformredis.Cache, log *slog.Logger, adminKey string) *Module {
	productRepo := pgstore.NewProductRepository(db)
	categoryRepo := pgstore.NewCategoryRepository(db)
	c := rediscache.New(cache)
	// Sự kiện đi vào bảng outbox trong CÙNG transaction với dữ liệu nghiệp vụ,
	// thay cho logpublisher của P0.2 (ghi log, mất khi tiến trình chết). Cả
	// domain lẫn app KHÔNG đổi một dòng nào — chỉ đúng dòng lắp ráp này.
	events := outboxpub.New(outbox.NewRepository(db))

	treeUC := app.NewGetCategoryTree(categoryRepo, c)

	return &Module{
		handler: httpapi.NewHandler(
			app.NewCreateProduct(db, productRepo, events, c),
			app.NewUpdateProduct(db, productRepo, events, c),
			app.NewPublishProduct(db, productRepo, events, c),
			app.NewGetProduct(productRepo, c),
			app.NewListProducts(productRepo, treeUC),
			treeUC,
		),
		adminKey: adminKey,
	}
}

func (m *Module) Mount(r chi.Router) { m.handler.Mount(r, m.adminKey) }
