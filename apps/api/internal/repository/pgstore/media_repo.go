package pgstore

import (
	"context"
	"errors"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/repository/pgstore/gen"
	"base-ecommerce/api/pkg/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MediaRepository struct{ db *postgres.Manager }

func NewMediaRepository(db *postgres.Manager) *MediaRepository {
	return &MediaRepository{db: db}
}

func (r *MediaRepository) Insert(ctx context.Context, m *domain.Media) error {
	return mapErr(gen.New(r.db.DB(ctx)).InsertMedia(ctx, gen.InsertMediaParams{
		ID: m.ID, ObjectKey: m.Key, ContentType: m.ContentType, SizeBytes: m.Size,
		Status: string(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}))
}

func (r *MediaRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.Media, error) {
	row, err := gen.New(r.db.DB(ctx)).MediaByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUnknownMedia
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return &domain.Media{
		ID: row.ID, Key: row.ObjectKey, ContentType: row.ContentType, Size: row.SizeBytes,
		Status:    domain.MediaStatus(row.Status),
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

func (r *MediaRepository) Update(ctx context.Context, m *domain.Media) error {
	return mapErr(gen.New(r.db.DB(ctx)).UpdateMedia(ctx, gen.UpdateMediaParams{
		ID: m.ID, ContentType: m.ContentType, SizeBytes: m.Size,
		Status: string(m.Status), UpdatedAt: m.UpdatedAt,
	}))
}

// ReadyKeys trả tập con của keys là media đã ready.
func (r *MediaRepository) ReadyKeys(ctx context.Context, keys []string) (map[string]bool, error) {
	out := make(map[string]bool, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := gen.New(r.db.DB(ctx)).ReadyMediaKeys(ctx, keys)
	if err != nil {
		return nil, mapErr(err)
	}
	for _, k := range rows {
		out[k] = true
	}
	return out, nil
}
