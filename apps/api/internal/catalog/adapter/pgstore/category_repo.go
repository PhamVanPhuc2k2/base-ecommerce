package pgstore

import (
	"context"

	"base-ecommerce/api/internal/catalog/adapter/pgstore/gen"
	"base-ecommerce/api/internal/catalog/domain"
	"base-ecommerce/api/internal/platform/postgres"
)

type CategoryRepository struct{ db *postgres.Manager }

func NewCategoryRepository(db *postgres.Manager) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) All(ctx context.Context) ([]*domain.Category, error) {
	// db.DB(ctx) trả transaction nếu đang ở trong transaction, ngược lại trả
	// pool. KHÔNG giữ con trỏ pool trực tiếp trong struct — check-arch.sh chặn
	// điều đó (xem scripts/check-arch.sh, mục 3).
	rows, err := gen.New(r.db.DB(ctx)).AllCategories(ctx)
	if err != nil {
		return nil, mapErr(err)
	}

	out := make([]*domain.Category, 0, len(rows))
	for _, row := range rows {
		out = append(out, &domain.Category{
			ID:        row.ID,
			ParentID:  row.ParentID,
			Slug:      row.Slug,
			Name:      row.Name,
			Position:  int(row.Position),
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out, nil
}
