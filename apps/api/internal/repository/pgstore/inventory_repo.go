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

type LocationRepository struct{ db *postgres.Manager }

func NewLocationRepository(db *postgres.Manager) *LocationRepository {
	return &LocationRepository{db: db}
}

func (r *LocationRepository) List(ctx context.Context) ([]*domain.Location, error) {
	rows, err := gen.New(r.db.DB(ctx)).AllLocations(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*domain.Location, len(rows))
	for i, row := range rows {
		out[i] = locationToDomain(row)
	}
	return out, nil
}

func (r *LocationRepository) ByID(ctx context.Context, id uuid.UUID) (*domain.Location, error) {
	row, err := gen.New(r.db.DB(ctx)).LocationByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return locationToDomain(row), nil
}

func (r *LocationRepository) Insert(ctx context.Context, l *domain.Location) error {
	err := gen.New(r.db.DB(ctx)).InsertLocation(ctx, gen.InsertLocationParams{
		ID: l.ID, Code: l.Code, Name: l.Name, Kind: string(l.Kind), Address: l.Address,
		SellsOnline: l.SellsOnline, Priority: int32(l.Priority), Active: l.Active,
		CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "locations_code_key" {
		return domain.ErrDuplicateLocationCode
	}
	return mapErr(err)
}

func (r *LocationRepository) Update(ctx context.Context, l *domain.Location) error {
	return mapErr(gen.New(r.db.DB(ctx)).UpdateLocation(ctx, gen.UpdateLocationParams{
		ID: l.ID, Name: l.Name, Kind: string(l.Kind), Address: l.Address, SellsOnline: l.SellsOnline,
		Priority: int32(l.Priority), Active: l.Active, UpdatedAt: l.UpdatedAt,
	}))
}

func locationToDomain(r gen.Location) *domain.Location {
	return &domain.Location{ID: r.ID, Code: r.Code, Name: r.Name, Kind: domain.LocationKind(r.Kind),
		Address: r.Address, SellsOnline: r.SellsOnline, Priority: int(r.Priority), Active: r.Active,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC()}
}

type StockRepository struct{ db *postgres.Manager }

func NewStockRepository(db *postgres.Manager) *StockRepository { return &StockRepository{db: db} }

func (r *StockRepository) VariantExists(ctx context.Context, id uuid.UUID) (bool, error) {
	ok, err := gen.New(r.db.DB(ctx)).VariantExists(ctx, id)
	return ok, mapErr(err)
}

// LockLevel: INSERT … ON CONFLICT DO NOTHING rồi FOR UPDATE. PHẢI chạy trong
// transaction — ngoài transaction thì khóa nhả ngay sau câu SELECT.
func (r *StockRepository) LockLevel(ctx context.Context, locationID, variantID uuid.UUID) (*domain.StockLevel, error) {
	q := gen.New(r.db.DB(ctx))
	err := q.EnsureStockLevel(ctx, gen.EnsureStockLevelParams{LocationID: locationID, VariantID: variantID,
		UpdatedAt: time.Now().UTC()})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		switch pgErr.ConstraintName {
		case "stock_levels_variant_id_fkey":
			return nil, domain.ErrVariantNotFound
		case "stock_levels_location_id_fkey":
			return nil, domain.ErrLocationNotFound
		}
	}
	if err != nil {
		return nil, mapErr(err)
	}
	row, err := q.StockLevelForUpdate(ctx, gen.StockLevelForUpdateParams{LocationID: locationID, VariantID: variantID})
	if err != nil {
		return nil, mapErr(err)
	}
	return &domain.StockLevel{LocationID: row.LocationID, VariantID: row.VariantID, OnHand: int(row.OnHand),
		Reserved: int(row.Reserved), UpdatedAt: row.UpdatedAt.UTC()}, nil
}

func (r *StockRepository) SaveLevel(ctx context.Context, s *domain.StockLevel) error {
	return mapErr(gen.New(r.db.DB(ctx)).UpdateStockLevel(ctx, gen.UpdateStockLevelParams{
		LocationID: s.LocationID, VariantID: s.VariantID, OnHand: int32(s.OnHand), Reserved: int32(s.Reserved),
		UpdatedAt: s.UpdatedAt,
	}))
}

func (r *StockRepository) InsertMovement(ctx context.Context, m *domain.StockMovement) error {
	err := gen.New(r.db.DB(ctx)).InsertStockMovement(ctx, gen.InsertStockMovementParams{
		ID: m.ID, LocationID: m.LocationID, VariantID: m.VariantID, Kind: string(m.Kind),
		OnHandDelta: int32(m.OnHandDelta), ReservedDelta: int32(m.ReservedDelta),
		OnHandAfter: int32(m.OnHandAfter), ReservedAfter: int32(m.ReservedAfter),
		Reason: m.Reason, Ref: m.Ref, ActorID: m.ActorID, IdempotencyKey: m.IdempotencyKey, CreatedAt: m.CreatedAt,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "stock_movements_idem_key" {
		return usecase.ErrIdempotencyRace
	}
	return mapErr(err)
}

func (r *StockRepository) MovementByIdempotencyKey(ctx context.Context, key string) (*domain.StockMovement, error) {
	row, err := gen.New(r.db.DB(ctx)).MovementByIdempotencyKey(ctx, &key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return movementToDomain(row), nil
}

func (r *StockRepository) VariantStock(ctx context.Context, variantID uuid.UUID) ([]usecase.LocationStock, error) {
	rows, err := gen.New(r.db.DB(ctx)).VariantStockByLocation(ctx, variantID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]usecase.LocationStock, len(rows))
	for i, row := range rows {
		out[i] = usecase.LocationStock{
			Location: &domain.Location{ID: row.ID, Code: row.Code, Name: row.Name, Kind: domain.LocationKind(row.Kind),
				Address: row.Address, SellsOnline: row.SellsOnline, Priority: int(row.Priority), Active: row.Active},
			OnHand: int(row.OnHand), Reserved: int(row.Reserved),
		}
	}
	return out, nil
}

func (r *StockRepository) Movements(ctx context.Context, f usecase.MovementFilter) ([]*domain.StockMovement, error) {
	rows, err := gen.New(r.db.DB(ctx)).ListStockMovements(ctx, gen.ListStockMovementsParams{
		VariantID: f.VariantID, LocationID: f.LocationID, Before: f.Before, Lim: int32(f.Limit),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]*domain.StockMovement, len(rows))
	for i, row := range rows {
		out[i] = movementToDomain(row)
	}
	return out, nil
}

func movementToDomain(r gen.StockMovement) *domain.StockMovement {
	return &domain.StockMovement{ID: r.ID, LocationID: r.LocationID, VariantID: r.VariantID,
		Kind: domain.MovementKind(r.Kind), OnHandDelta: int(r.OnHandDelta), ReservedDelta: int(r.ReservedDelta),
		OnHandAfter: int(r.OnHandAfter), ReservedAfter: int(r.ReservedAfter), Reason: r.Reason, Ref: r.Ref,
		ActorID: r.ActorID, IdempotencyKey: r.IdempotencyKey, CreatedAt: r.CreatedAt.UTC()}
}
