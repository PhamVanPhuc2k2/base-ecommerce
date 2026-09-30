package usecase

import (
	"context"
	"errors"
	"time"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

type ReservationRepository interface {
	// LockCandidates khóa (FOR UPDATE, thứ tự cố định) tồn ứng viên — kho đang
	// hoạt động và bán online — của các phiên bản.
	LockCandidates(ctx context.Context, variantIDs []uuid.UUID) (map[uuid.UUID][]domain.Candidate, error)
	// LockStock khóa các dòng tồn của một giữ chỗ, cùng thứ tự với trên.
	LockStock(ctx context.Context, reservationID uuid.UUID) (map[[2]uuid.UUID]*domain.StockLevel, error)
	// Insert: trùng ref → ErrIdempotencyRace.
	Insert(ctx context.Context, r *domain.Reservation) error
	// ByID / ByRef trả (nil, nil) khi không có.
	ByID(ctx context.Context, id uuid.UUID, forUpdate bool) (*domain.Reservation, error)
	ByRef(ctx context.Context, ref string) (*domain.Reservation, error)
	UpdateStatus(ctx context.Context, r *domain.Reservation) error
	// DueForUpdate khóa (SKIP LOCKED) tối đa limit giữ chỗ active đã hết hạn.
	DueForUpdate(ctx context.Context, now time.Time, limit int) ([]*domain.Reservation, error)
}

// Reservations giữ / nhả / xuất kho (đặc tả P3.2). P4 gọi thẳng use case này.
type Reservations struct {
	tx           TxManager
	reservations ReservationRepository
	stock        StockRepository
}

func NewReservations(tx TxManager, reservations ReservationRepository, stock StockRepository) *Reservations {
	return &Reservations{tx: tx, reservations: reservations, stock: stock}
}

type ReserveInput struct {
	Ref   string
	Items []domain.ReservationItem
	// TTL nil = không hết hạn (đơn COD).
	TTL *time.Duration
}

// Reserve giữ hàng cho một đơn. Thiếu bất kỳ dòng nào → không giữ gì cả.
// Gọi lại cùng Ref + cùng hàng → trả giữ chỗ cũ, replayed = true.
func (u *Reservations) Reserve(ctx context.Context, in ReserveInput) (*domain.Reservation, bool, error) {
	ref, err := domain.ValidateReservationRequest(in.Ref, in.Items, in.TTL)
	if err != nil {
		return nil, false, err
	}
	replay := func() (*domain.Reservation, bool, error) {
		r, err := u.reservations.ByRef(ctx, ref)
		if err != nil || r == nil {
			return nil, false, err
		}
		if !r.SameItems(in.Items) {
			return nil, false, domain.ErrReservationRefReused
		}
		return r, true, nil
	}
	if r, replayed, err := replay(); r != nil || err != nil {
		return r, replayed, err
	}

	var res *domain.Reservation
	err = u.tx.Run(ctx, func(ctx context.Context) error {
		ids := make([]uuid.UUID, len(in.Items))
		for i, it := range in.Items {
			ids[i] = it.VariantID
		}
		cands, err := u.reservations.LockCandidates(ctx, ids)
		if err != nil {
			return err
		}
		lines, err := domain.Allocate(in.Items, cands)
		if err != nil {
			return err
		}
		res = domain.NewReservation(ref, lines, in.TTL, time.Now().UTC())
		// Insert TRƯỚC khi đổi tồn: trùng ref (request song song) thì thất bại
		// ngay, không tốn công ghi tồn rồi rollback.
		if err := u.reservations.Insert(ctx, res); err != nil {
			return err
		}
		levels := map[[2]uuid.UUID]*domain.StockLevel{}
		for _, cs := range cands {
			for _, c := range cs {
				levels[[2]uuid.UUID{c.Level.LocationID, c.Level.VariantID}] = c.Level
			}
		}
		for _, l := range lines {
			lvl := levels[[2]uuid.UUID{l.LocationID, l.VariantID}]
			mv, err := lvl.Reserve(l.Quantity, ref)
			if err != nil {
				return err
			}
			if err := u.save(ctx, lvl, mv); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, ErrIdempotencyRace) {
		return replay()
	}
	if err != nil {
		return nil, false, err
	}
	return res, false, nil
}

func (u *Reservations) Get(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	r, err := u.reservations.ByID(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, domain.ErrUnknownReservation
	}
	return r, nil
}

// Release nhả giữ chỗ (hủy đơn).
func (u *Reservations) Release(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	return u.finish(ctx, id, domain.ReservationReleased)
}

// Commit xuất kho: hàng rời kho, trừ cả on_hand lẫn reserved.
func (u *Reservations) Commit(ctx context.Context, id uuid.UUID) (*domain.Reservation, error) {
	return u.finish(ctx, id, domain.ReservationCommitted)
}

func (u *Reservations) finish(ctx context.Context, id uuid.UUID, to domain.ReservationStatus) (*domain.Reservation, error) {
	var res *domain.Reservation
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		var err error
		if res, err = u.reservations.ByID(ctx, id, true); err != nil {
			return err
		}
		if res == nil {
			return domain.ErrUnknownReservation
		}
		return u.close(ctx, res, to)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// close chuyển trạng thái và trả tồn. PHẢI chạy trong transaction, sau khi
// đã khóa dòng giữ chỗ.
func (u *Reservations) close(ctx context.Context, res *domain.Reservation, to domain.ReservationStatus) error {
	if err := res.Finish(to, time.Now().UTC()); err != nil {
		return err
	}
	levels, err := u.reservations.LockStock(ctx, res.ID)
	if err != nil {
		return err
	}
	for _, l := range res.Lines {
		lvl := levels[[2]uuid.UUID{l.LocationID, l.VariantID}]
		var mv *domain.StockMovement
		if to == domain.ReservationCommitted {
			mv, err = lvl.Commit(l.Quantity, res.Ref)
		} else {
			mv, err = lvl.Release(l.Quantity, res.Ref)
		}
		if err != nil {
			return err
		}
		if err := u.save(ctx, lvl, mv); err != nil {
			return err
		}
	}
	return u.reservations.UpdateStatus(ctx, res)
}

// ExpireDue nhả tối đa limit giữ chỗ đã hết hạn — worker gọi định kỳ.
func (u *Reservations) ExpireDue(ctx context.Context, limit int) (int, error) {
	n := 0
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		n = 0
		due, err := u.reservations.DueForUpdate(ctx, time.Now().UTC(), limit)
		if err != nil {
			return err
		}
		for _, r := range due {
			if err := u.close(ctx, r, domain.ReservationExpired); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

func (u *Reservations) save(ctx context.Context, lvl *domain.StockLevel, mv *domain.StockMovement) error {
	if err := u.stock.SaveLevel(ctx, lvl); err != nil {
		return err
	}
	return u.stock.InsertMovement(ctx, mv)
}
