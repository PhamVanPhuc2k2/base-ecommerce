package main

import (
	"log/slog"

	"base-ecommerce/api/internal/delivery/httpapi"
	"base-ecommerce/api/internal/repository/outbox"
	"base-ecommerce/api/internal/repository/outboxpub"
	"base-ecommerce/api/internal/repository/pgstore"
	"base-ecommerce/api/internal/repository/rediscache"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/authtoken"
	"base-ecommerce/api/pkg/config"
	"base-ecommerce/api/pkg/objectstore"
	"base-ecommerce/api/pkg/password"
	"base-ecommerce/api/pkg/postgres"
	platformredis "base-ecommerce/api/pkg/redis"

	goredis "github.com/redis/go-redis/v9"
)

// newHandler là composition root: nơi DUY NHẤT biết cả repository, usecase và
// delivery. Mọi package khác chỉ thấy đúng phần nó cần.
//
// Đặt ở cmd/api chứ không ở internal/: ráp nối là việc của chương trình chạy,
// không phải của một tầng nào trong Clean Architecture.
func newHandler(cfg *config.Config, db *postgres.Manager, cache *platformredis.Cache,
	rdb *goredis.Client, store *objectstore.Client, log *slog.Logger) (*httpapi.Handler, error) {
	productRepo := pgstore.NewProductRepository(db)
	categoryRepo := pgstore.NewCategoryRepository(db)
	brandRepo := pgstore.NewBrandRepository(db)
	c := rediscache.New(cache)
	// Sự kiện đi vào bảng outbox trong CÙNG transaction với dữ liệu nghiệp vụ,
	// thay cho logpublisher của P0.2 (ghi log, mất khi tiến trình chết). Cả
	// domain lẫn usecase KHÔNG đổi một dòng nào — chỉ đúng dòng lắp ráp này.
	events := outboxpub.New(outbox.NewRepository(db))

	attributeRepo := pgstore.NewAttributeRepository(db)
	mediaRepo := pgstore.NewMediaRepository(db)

	treeUC := usecase.NewGetCategoryTree(categoryRepo, c)
	schemas := usecase.NewAttributeSchemas(attributeRepo, c, treeUC)
	rules := usecase.NewProductRules(schemas, mediaRepo)

	issuer := authtoken.NewIssuer(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL)
	userRepo := pgstore.NewUserRepository(db)
	tokenRepo := pgstore.NewRefreshTokenRepository(db)
	hasher := password.NewHasher()
	limiter := platformredis.NewRateLimiter(rdb, log)
	verification := usecase.NewVerification(db, userRepo, tokenRepo, pgstore.NewOTPRepository(db),
		pgstore.NewEmailRepository(db), events, hasher, limiter, []byte(cfg.Auth.JWTSecret))
	auth, err := usecase.NewAuth(db, userRepo, tokenRepo, hasher, issuer, limiter, verification, cfg.Auth.RefreshTTL)
	if err != nil {
		return nil, err
	}

	roleRepo := pgstore.NewRoleRepository(db)
	stockRepo := pgstore.NewStockRepository(db)

	return httpapi.NewHandler(issuer, httpapi.Usecases{
		CreateProduct:  usecase.NewCreateProduct(db, productRepo, events, c, rules),
		UpdateProduct:  usecase.NewUpdateProduct(db, productRepo, events, c, rules),
		PublishProduct: usecase.NewPublishProduct(db, productRepo, events, c, rules),
		GetProduct:     usecase.NewGetProduct(productRepo, c),
		ListProducts:   usecase.NewListProducts(productRepo, treeUC, c, schemas),
		CategoryTree:   treeUC,
		CreateCategory: usecase.NewCreateCategory(db, categoryRepo, c),
		UpdateCategory: usecase.NewUpdateCategory(db, categoryRepo, c),
		DeleteCategory: usecase.NewDeleteCategory(db, categoryRepo, c),
		AddVariant:     usecase.NewAddVariant(db, productRepo, events, c, rules),
		UpdateVariant:  usecase.NewUpdateVariant(db, productRepo, events, c, rules),
		Media:          usecase.NewMediaUploads(db, mediaRepo, store),
		Sitemap:        usecase.NewSitemapProducts(productRepo),
		ListFacets:     usecase.NewListFacets(productRepo, treeUC, c, schemas),
		ListAttributes: usecase.NewListAttributes(schemas),
		CategoryAttrs:  usecase.NewCategoryAttributes(schemas),
		AttributeAdmin: usecase.NewAttributeAdmin(db, attributeRepo, c),
		ListBrands:     usecase.NewListBrands(brandRepo, c),
		CreateBrand:    usecase.NewCreateBrand(db, brandRepo, c),
		UpdateBrand:    usecase.NewUpdateBrand(db, brandRepo, c),
		DeleteBrand:    usecase.NewDeleteBrand(db, brandRepo, c),
		Auth:           auth,
		Verification:   verification,
		AddressBook:    usecase.NewAddressBook(db, userRepo, pgstore.NewAddressRepository(db)),
		Inventory:      usecase.NewInventory(db, pgstore.NewLocationRepository(db), stockRepo),
		Reservations:   usecase.NewReservations(db, pgstore.NewReservationRepository(db), stockRepo),
		Authorizer:     usecase.NewAuthorizer(roleRepo, c),
		RoleAdmin:      usecase.NewRoleAdmin(db, roleRepo, c),
	}), nil
}
