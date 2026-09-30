package pgstore

import (
	"context"
	"errors"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/repository/pgstore/gen"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type UserRepository struct{ db *postgres.Manager }

func NewUserRepository(db *postgres.Manager) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Insert(ctx context.Context, u *domain.User) error {
	err := gen.New(r.db.DB(ctx)).InsertUser(ctx, gen.InsertUserParams{
		ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash, FullName: u.FullName,
		Status: string(u.Status), EmailVerifiedAt: u.EmailVerifiedAt,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
		return domain.ErrEmailTaken
	}
	return mapErr(err)
}

// ByEmail trả (nil, nil) khi không có — "không có" là kết quả bình thường của
// đăng nhập sai email, không phải lỗi; use case tự quyết định trả gì cho client.
func (r *UserRepository) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := gen.New(r.db.DB(ctx)).UserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return userToDomain(row), nil
}

func (r *UserRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row, err := gen.New(r.db.DB(ctx)).UserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return userToDomain(row), nil
}

func userToDomain(r gen.User) *domain.User {
	u := &domain.User{
		ID: r.ID, Email: r.Email, PasswordHash: r.PasswordHash, FullName: r.FullName,
		Status: domain.UserStatus(r.Status), EmailVerifiedAt: r.EmailVerifiedAt,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}
	if u.EmailVerifiedAt != nil {
		t := u.EmailVerifiedAt.UTC()
		u.EmailVerifiedAt = &t
	}
	return u
}

type RefreshTokenRepository struct{ db *postgres.Manager }

func NewRefreshTokenRepository(db *postgres.Manager) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Insert(ctx context.Context, t usecase.RefreshToken) error {
	return mapErr(gen.New(r.db.DB(ctx)).InsertRefreshToken(ctx, gen.InsertRefreshTokenParams{
		ID: t.ID, FamilyID: t.FamilyID, UserID: t.UserID, TokenHash: t.Hash,
		ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt,
	}))
}

// ByHashForUpdate khóa dòng — xem chú thích ở queries/user.sql. (nil, nil) khi
// không có.
func (r *RefreshTokenRepository) ByHashForUpdate(ctx context.Context, hash []byte) (*usecase.RefreshToken, error) {
	row, err := gen.New(r.db.DB(ctx)).RefreshTokenByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return &usecase.RefreshToken{
		ID: row.ID, FamilyID: row.FamilyID, UserID: row.UserID, Hash: row.TokenHash,
		ExpiresAt: row.ExpiresAt, UsedAt: row.UsedAt, RevokedAt: row.RevokedAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (r *RefreshTokenRepository) MarkUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	return mapErr(gen.New(r.db.DB(ctx)).MarkRefreshTokenUsed(ctx, gen.MarkRefreshTokenUsedParams{ID: id, UsedAt: &at}))
}

func (r *RefreshTokenRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error {
	return mapErr(gen.New(r.db.DB(ctx)).RevokeRefreshFamily(ctx, gen.RevokeRefreshFamilyParams{FamilyID: familyID, RevokedAt: &at}))
}
