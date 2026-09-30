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

type ReservationRepository struct{ db *postgres.Manager }

func NewReservationRepository(db *postgres.Manager) *ReservationRepository {
	return &ReservationRepository{db: db}
}

// LockCandidates khóa tồn ứng viên của các phiên bản — xem LockCandidateStock.
func (r *ReservationRepository) LockCandidates(ctx context.Context, variantIDs []uuid.UUID) (map[uuid.UUID][]domain.Candidate, error) {
	rows, err := gen.New(r.db.DB(ctx)).LockCandidateStock(ctx, variantIDs)
	if err != nil {
		return nil, mapErr(err)
	}
	out := map[uuid.UUID][]domain.Candidate{}
	for _, row := range rows {
		out[row.VariantID] = append(out[row.VariantID], domain.Candidate{
			Level: &domain.StockLevel{LocationID: row.LocationID, VariantID: row.VariantID, OnHand: int(row.OnHand),
				Reserved: int(row.Reserved), UpdatedAt: row.UpdatedAt.UTC()},
			Priority: int(row.Priority), Code: row.Code,
		})
	}
	return out, nil
}

// LockStock khóa các dòng tồn của một giữ chỗ; khóa của map là (kho, phiên bản).
func (r *ReservationRepository) LockStock(ctx context.Context, reservationID uuid.UUID) (map[[2]uuid.UUID]*domain.StockLevel, error) {
	rows, err := gen.New(r.db.DB(ctx)).LockReservationStock(ctx, reservationID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make(map[[2]uuid.UUID]*domain.StockLevel, len(rows))
	for _, row := range rows {
		out[[2]uuid.UUID{row.LocationID, row.VariantID}] = &domain.StockLevel{LocationID: row.LocationID,
			VariantID: row.VariantID, OnHand: int(row.OnHand), Reserved: int(row.Reserved), UpdatedAt: row.UpdatedAt.UTC()}
	}
	return out, nil
}

func (r *ReservationRepository) Insert(ctx context.Context, res *domain.Reservation) error {
	q := gen.New(r.db.DB(ctx))
	err := q.InsertReservation(ctx, gen.InsertReservationParams{ID: res.ID, Ref: res.Ref, Status: string(res.Status),
		ExpiresAt: res.ExpiresAt, CreatedAt: res.CreatedAt, UpdatedAt: res.UpdatedAt})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "reservations_ref_key" {
		return usecase.ErrIdempotencyRace
	}
	if err != nil {
		return mapErr(err)
	}
	for _, l := range res.Lines {
		if err := q.InsertReservationLine(ctx, gen.InsertReservationLineParams{ReservationID: res.ID,
			VariantID: l.VariantID, LocationID: l.LocationID, Quantity: int32(l.Quantity)}); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (r *ReservationRepository) ByID(ctx context.Context, id uuid.UUID, forUpdate bool) (*domain.Reservation, error) {
	q := gen.New(r.db.DB(ctx))
	get := q.ReservationByID
	if forUpdate {
		get = q.ReservationByIDForUpdate
	}
	row, err := get(ctx, id)
	return r.withLines(ctx, row, err)
}

func (r *ReservationRepository) ByRef(ctx context.Context, ref string) (*domain.Reservation, error) {
	row, err := gen.New(r.db.DB(ctx)).ReservationByRef(ctx, ref)
	return r.withLines(ctx, row, err)
}

func (r *ReservationRepository) withLines(ctx context.Context, row gen.Reservation, err error) (*domain.Reservation, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	out, err := r.attachLines(ctx, []gen.Reservation{row})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

func (r *ReservationRepository) attachLines(ctx context.Context, rows []gen.Reservation) ([]*domain.Reservation, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(rows))
	byID := make(map[uuid.UUID]*domain.Reservation, len(rows))
	out := make([]*domain.Reservation, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
		out[i] = &domain.Reservation{ID: row.ID, Ref: row.Ref, Status: domain.ReservationStatus(row.Status),
			ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
		byID[row.ID] = out[i]
	}
	lines, err := gen.New(r.db.DB(ctx)).ReservationLinesOf(ctx, ids)
	if err != nil {
		return nil, mapErr(err)
	}
	for _, l := range lines {
		res := byID[l.ReservationID]
		res.Lines = append(res.Lines, domain.ReservationLine{VariantID: l.VariantID, LocationID: l.LocationID,
			Quantity: int(l.Quantity)})
	}
	return out, nil
}

func (r *ReservationRepository) UpdateStatus(ctx context.Context, res *domain.Reservation) error {
	return mapErr(gen.New(r.db.DB(ctx)).UpdateReservationStatus(ctx, gen.UpdateReservationStatusParams{
		ID: res.ID, Status: string(res.Status), UpdatedAt: res.UpdatedAt}))
}

// DueForUpdate khóa (SKIP LOCKED) tối đa limit giữ chỗ đã hết hạn.
func (r *ReservationRepository) DueForUpdate(ctx context.Context, now time.Time, limit int) ([]*domain.Reservation, error) {
	rows, err := gen.New(r.db.DB(ctx)).DueReservations(ctx, gen.DueReservationsParams{Now: &now, Lim: int32(limit)})
	if err != nil {
		return nil, mapErr(err)
	}
	return r.attachLines(ctx, rows)
}
