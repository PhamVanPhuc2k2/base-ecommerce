package pgstore

import (
	"context"
	"errors"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/repository/pgstore/gen"
	"base-ecommerce/api/pkg/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type BrandRepository struct{ db *postgres.Manager }

func NewBrandRepository(db *postgres.Manager) *BrandRepository {
	return &BrandRepository{db: db}
}

func (r *BrandRepository) All(ctx context.Context) (domain.Brands, error) {
	rows, err := gen.New(r.db.DB(ctx)).AllBrands(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make(domain.Brands, 0, len(rows))
	for _, row := range rows {
		out = append(out, brandToDomain(row))
	}
	return out, nil
}

func (r *BrandRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.Brand, error) {
	row, err := gen.New(r.db.DB(ctx)).BrandByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUnknownBrand
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return brandToDomain(row), nil
}

func (r *BrandRepository) Insert(ctx context.Context, b *domain.Brand) error {
	return mapBrandWriteErr(gen.New(r.db.DB(ctx)).InsertBrand(ctx, gen.InsertBrandParams{
		ID: b.ID, Slug: b.Slug, Name: b.Name, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}))
}

func (r *BrandRepository) Update(ctx context.Context, b *domain.Brand) error {
	n, err := gen.New(r.db.DB(ctx)).UpdateBrand(ctx, gen.UpdateBrandParams{
		ID: b.ID, Slug: b.Slug, Name: b.Name, UpdatedAt: b.UpdatedAt,
	})
	if err != nil {
		return mapBrandWriteErr(err)
	}
	if n == 0 {
		return domain.ErrUnknownBrand
	}
	return nil
}

func (r *BrandRepository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := gen.New(r.db.DB(ctx)).DeleteBrand(ctx, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" &&
			pgErr.ConstraintName == "products_brand_id_fkey" {
			return domain.ErrBrandHasProducts
		}
		return mapErr(err)
	}
	if n == 0 {
		return domain.ErrUnknownBrand
	}
	return nil
}

func mapBrandWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if err != nil && errors.As(err, &pgErr) &&
		pgErr.Code == "23505" && pgErr.ConstraintName == "brands_slug_key" {
		return domain.ErrDuplicateBrandSlug
	}
	return mapErr(err)
}

func brandToDomain(row gen.Brand) *domain.Brand {
	return &domain.Brand{
		ID: row.ID, Slug: row.Slug, Name: row.Name,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
}
