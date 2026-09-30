package usecase

import (
	"context"
	"errors"
	"strings"

	"base-ecommerce/api/internal/domain"

	"github.com/google/uuid"
)

// ---- Ports (P3.1) ----

type LocationRepository interface {
	List(ctx context.Context) ([]*domain.Location, error)
	// ByID trả (nil, nil) khi không có.
	ByID(ctx context.Context, id uuid.UUID) (*domain.Location, error)
	// Insert: trùng code → domain.ErrDuplicateLocationCode.
	Insert(ctx context.Context, l *domain.Location) error
	Update(ctx context.Context, l *domain.Location) error
}

// LocationStock là tồn của một phiên bản tại một kho, kèm thông tin kho.
type LocationStock struct {
	Location *domain.Location
	OnHand   int
	Reserved int
}

type MovementFilter struct {
	VariantID  *uuid.UUID
	LocationID *uuid.UUID
	// Before: con trỏ — chỉ lấy dòng có id nhỏ hơn (cũ hơn).
	Before *uuid.UUID
	Limit  int
}

// ErrIdempotencyRace: hai request cùng Idempotency-Key cùng đi tới bước ghi
// sổ cái; cái tới sau vấp UNIQUE. Không phải lỗi của khách — use case đọc lại
// kết quả của cái tới trước và trả nó.
var ErrIdempotencyRace = errors.New("idempotency key vừa được dùng bởi request song song")

type StockRepository interface {
	VariantExists(ctx context.Context, id uuid.UUID) (bool, error)
	// LockLevel bảo đảm có dòng tồn rồi khóa nó (FOR UPDATE). Phiên bản
	// không tồn tại → domain.ErrVariantNotFound.
	LockLevel(ctx context.Context, locationID, variantID uuid.UUID) (*domain.StockLevel, error)
	SaveLevel(ctx context.Context, s *domain.StockLevel) error
	// InsertMovement: trùng idempotency_key → ErrIdempotencyRace.
	InsertMovement(ctx context.Context, m *domain.StockMovement) error
	// MovementByIdempotencyKey trả (nil, nil) khi chưa có.
	MovementByIdempotencyKey(ctx context.Context, key string) (*domain.StockMovement, error)
	VariantStock(ctx context.Context, variantID uuid.UUID) ([]LocationStock, error)
	Movements(ctx context.Context, f MovementFilter) ([]*domain.StockMovement, error)
}

// ---- Use case ----

const maxIdempotencyKeyLen = 100

// Inventory là quản trị kho và tồn kho (đặc tả P3.1).
type Inventory struct {
	tx        TxManager
	locations LocationRepository
	stock     StockRepository
}

func NewInventory(tx TxManager, locations LocationRepository, stock StockRepository) *Inventory {
	return &Inventory{tx: tx, locations: locations, stock: stock}
}

func (i *Inventory) Locations(ctx context.Context) ([]*domain.Location, error) {
	return i.locations.List(ctx)
}

func (i *Inventory) CreateLocation(ctx context.Context, in domain.LocationInput) (*domain.Location, error) {
	l, err := domain.NewLocation(in)
	if err != nil {
		return nil, err
	}
	if err := i.locations.Insert(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

func (i *Inventory) UpdateLocation(ctx context.Context, id uuid.UUID, p domain.LocationPatch) (*domain.Location, error) {
	l, err := i.locations.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, domain.ErrUnknownLocation
	}
	if err := l.Update(p); err != nil {
		return nil, err
	}
	if err := i.locations.Update(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

type StockChangeInput struct {
	LocationID     uuid.UUID
	VariantID      uuid.UUID
	Change         domain.ManualChange
	Actor          *uuid.UUID
	IdempotencyKey string
}

// StockChangeResult: Replayed = true khi đây là kết quả CŨ của một request
// trước đó cùng Idempotency-Key — tồn không đổi lần nữa.
type StockChangeResult struct {
	Movement *domain.StockMovement
	Replayed bool
}

// RecordChange nhập hàng / điều chỉnh / kiểm kê — đúng một lần cho mỗi
// Idempotency-Key (đặc tả P3.1 mục 2.4, 2.5).
func (i *Inventory) RecordChange(ctx context.Context, in StockChangeInput) (*StockChangeResult, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	if len(key) > maxIdempotencyKeyLen {
		return nil, domain.ErrInvalidIdempotencyKey
	}
	replay := func() (*StockChangeResult, error) {
		if key == "" {
			return nil, nil
		}
		m, err := i.stock.MovementByIdempotencyKey(ctx, key)
		if err != nil || m == nil {
			return nil, err
		}
		if !m.SameRequest(in.LocationID, in.VariantID, in.Change) {
			return nil, domain.ErrIdempotencyKeyReused
		}
		return &StockChangeResult{Movement: m, Replayed: true}, nil
	}
	// Đường nhanh: khóa đã dùng xong từ trước — không mở transaction, không khóa.
	if r, err := replay(); r != nil || err != nil {
		return r, err
	}

	var mv *domain.StockMovement
	err := i.tx.Run(ctx, func(ctx context.Context) error {
		loc, err := i.locations.ByID(ctx, in.LocationID)
		if err != nil {
			return err
		}
		if loc == nil {
			return domain.ErrLocationNotFound
		}
		// Chỉ chặn NHẬP vào kho ngừng hoạt động. Điều chỉnh/kiểm kê vẫn được —
		// đóng cửa một showroom thì phải đưa được tồn của nó về 0.
		if in.Change.Kind == domain.MovementReceipt && !loc.Active {
			return domain.ErrLocationInactive
		}
		lvl, err := i.stock.LockLevel(ctx, in.LocationID, in.VariantID)
		if err != nil {
			return err
		}
		if mv, err = lvl.Apply(in.Change, in.Actor); err != nil {
			return err
		}
		if key != "" {
			mv.IdempotencyKey = &key
		}
		if err := i.stock.SaveLevel(ctx, lvl); err != nil {
			return err
		}
		return i.stock.InsertMovement(ctx, mv)
	})
	if errors.Is(err, ErrIdempotencyRace) {
		// Request song song cùng khóa đã commit trước — trả kết quả của nó.
		if r, rerr := replay(); r != nil || rerr != nil {
			return r, rerr
		}
	}
	if err != nil {
		return nil, err
	}
	return &StockChangeResult{Movement: mv}, nil
}

// VariantStockView là tồn của một phiên bản trên mọi kho, kèm các tổng.
type VariantStockView struct {
	Levels   []LocationStock
	OnHand   int
	Reserved int
	// OnlineAvailable: phần bán được qua web — chỉ kho đang hoạt động VÀ có
	// cờ bán online (quyết định P3 số 1).
	OnlineAvailable int
}

func (v *VariantStockView) Available() int { return v.OnHand - v.Reserved }

func (i *Inventory) VariantStock(ctx context.Context, variantID uuid.UUID) (*VariantStockView, error) {
	ok, err := i.stock.VariantExists(ctx, variantID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrUnknownVariant
	}
	levels, err := i.stock.VariantStock(ctx, variantID)
	if err != nil {
		return nil, err
	}
	v := &VariantStockView{Levels: levels}
	for _, l := range levels {
		v.OnHand += l.OnHand
		v.Reserved += l.Reserved
		if l.Location.Active && l.Location.SellsOnline {
			v.OnlineAvailable += l.OnHand - l.Reserved
		}
	}
	return v, nil
}

const (
	defaultMovementLimit = 50
	maxMovementLimit     = 200
)

// Movements trả một trang sổ cái (mới nhất trước) và con trỏ trang sau — nil
// khi chắc chắn hết. Lấy dư một dòng để biết còn hay không, thay vì trả con
// trỏ cho một trang cuối rỗng.
func (i *Inventory) Movements(ctx context.Context, f MovementFilter) ([]*domain.StockMovement, *uuid.UUID, error) {
	switch {
	case f.Limit <= 0:
		f.Limit = defaultMovementLimit
	case f.Limit > maxMovementLimit:
		f.Limit = maxMovementLimit
	}
	want := f.Limit
	f.Limit++
	list, err := i.stock.Movements(ctx, f)
	if err != nil || len(list) <= want {
		return list, nil, err
	}
	list = list[:want]
	return list, &list[want-1].ID, nil
}
