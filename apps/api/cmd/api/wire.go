package main

import (
	"base-ecommerce/api/internal/delivery/httpapi"
	"base-ecommerce/api/internal/repository/outbox"
	"base-ecommerce/api/internal/repository/outboxpub"
	"base-ecommerce/api/internal/repository/pgstore"
	"base-ecommerce/api/internal/repository/rediscache"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/postgres"
	platformredis "base-ecommerce/api/pkg/redis"
)

// newCatalogHandler là composition root của catalog: nơi DUY NHẤT biết cả
// repository, usecase và delivery. Mọi package khác chỉ thấy đúng phần nó cần.
//
// Đặt ở cmd/api chứ không ở internal/: ráp nối là việc của chương trình chạy,
// không phải của một tầng nào trong Clean Architecture.
func newCatalogHandler(db *postgres.Manager, cache *platformredis.Cache, adminKey string) *httpapi.Handler {
	productRepo := pgstore.NewProductRepository(db)
	categoryRepo := pgstore.NewCategoryRepository(db)
	brandRepo := pgstore.NewBrandRepository(db)
	c := rediscache.New(cache)
	// Sự kiện đi vào bảng outbox trong CÙNG transaction với dữ liệu nghiệp vụ,
	// thay cho logpublisher của P0.2 (ghi log, mất khi tiến trình chết). Cả
	// domain lẫn usecase KHÔNG đổi một dòng nào — chỉ đúng dòng lắp ráp này.
	events := outboxpub.New(outbox.NewRepository(db))

	treeUC := usecase.NewGetCategoryTree(categoryRepo, c)

	return httpapi.NewHandler(adminKey, httpapi.Usecases{
		CreateProduct:  usecase.NewCreateProduct(db, productRepo, events, c),
		UpdateProduct:  usecase.NewUpdateProduct(db, productRepo, events, c),
		PublishProduct: usecase.NewPublishProduct(db, productRepo, events, c),
		GetProduct:     usecase.NewGetProduct(productRepo, c),
		ListProducts:   usecase.NewListProducts(productRepo, treeUC, c),
		CategoryTree:   treeUC,
		CreateCategory: usecase.NewCreateCategory(db, categoryRepo, c),
		UpdateCategory: usecase.NewUpdateCategory(db, categoryRepo, c),
		DeleteCategory: usecase.NewDeleteCategory(db, categoryRepo, c),
		ListBrands:     usecase.NewListBrands(brandRepo, c),
		CreateBrand:    usecase.NewCreateBrand(db, brandRepo, c),
		UpdateBrand:    usecase.NewUpdateBrand(db, brandRepo, c),
		DeleteBrand:    usecase.NewDeleteBrand(db, brandRepo, c),
	})
}
