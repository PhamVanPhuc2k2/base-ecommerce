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

type AddressRepository struct{ db *postgres.Manager }

func NewAddressRepository(db *postgres.Manager) *AddressRepository { return &AddressRepository{db: db} }

func (r *AddressRepository) List(ctx context.Context, userID uuid.UUID) ([]*domain.Address, error) {
	rows, err := gen.New(r.db.DB(ctx)).AddressesOf(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*domain.Address, len(rows))
	for i, row := range rows {
		out[i] = addressToDomain(row)
	}
	return out, nil
}

// ByIDForUpdate khóa địa chỉ; (nil, nil) khi không có HOẶC là của người khác.
func (r *AddressRepository) ByIDForUpdate(ctx context.Context, userID, id uuid.UUID) (*domain.Address, error) {
	row, err := gen.New(r.db.DB(ctx)).AddressForUpdate(ctx, gen.AddressForUpdateParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return addressToDomain(row), nil
}

func (r *AddressRepository) Count(ctx context.Context, userID uuid.UUID) (int, error) {
	n, err := gen.New(r.db.DB(ctx)).CountAddresses(ctx, userID)
	return int(n), mapErr(err)
}

func (r *AddressRepository) Insert(ctx context.Context, a *domain.Address) error {
	return mapErr(gen.New(r.db.DB(ctx)).InsertAddress(ctx, gen.InsertAddressParams{
		ID: a.ID, UserID: a.UserID, RecipientName: a.RecipientName, Phone: a.Phone, Province: a.Province,
		Ward: a.Ward, Street: a.Street, IsDefault: a.IsDefault, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}))
}

func (r *AddressRepository) Update(ctx context.Context, a *domain.Address) error {
	return mapErr(gen.New(r.db.DB(ctx)).UpdateAddress(ctx, gen.UpdateAddressParams{
		ID: a.ID, RecipientName: a.RecipientName, Phone: a.Phone, Province: a.Province,
		Ward: a.Ward, Street: a.Street, UpdatedAt: a.UpdatedAt,
	}))
}

func (r *AddressRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return mapErr(gen.New(r.db.DB(ctx)).DeleteAddress(ctx, id))
}

// MakeDefault bỏ cờ mặc định cũ RỒI mới đặt cờ mới — ngược thứ tự thì unique
// index addresses_one_default từ chối ngay câu thứ hai.
func (r *AddressRepository) MakeDefault(ctx context.Context, userID, id uuid.UUID) error {
	q := gen.New(r.db.DB(ctx))
	if err := q.ClearDefaultAddress(ctx, userID); err != nil {
		return mapErr(err)
	}
	return mapErr(q.SetDefaultAddress(ctx, id))
}

// NewestID trả uuid.Nil khi sổ trống.
func (r *AddressRepository) NewestID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	id, err := gen.New(r.db.DB(ctx)).NewestAddressID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	return id, mapErr(err)
}

func addressToDomain(r gen.Address) *domain.Address {
	return &domain.Address{ID: r.ID, UserID: r.UserID, RecipientName: r.RecipientName, Phone: r.Phone,
		Province: r.Province, Ward: r.Ward, Street: r.Street, IsDefault: r.IsDefault,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC()}
}
