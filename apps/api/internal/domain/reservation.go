package domain

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"base-ecommerce/api/pkg/errs"

	"github.com/google/uuid"
)

// ---- Giữ chỗ tồn kho (đặc tả P3.2) ----

const (
	MovementReserve MovementKind = "reserve"
	MovementRelease MovementKind = "release"
	MovementCommit  MovementKind = "commit"
)

type ReservationStatus string

const (
	ReservationActive    ReservationStatus = "active"
	ReservationCommitted ReservationStatus = "committed"
	ReservationReleased  ReservationStatus = "released"
	ReservationExpired   ReservationStatus = "expired"
)

const (
	maxReservationItems = 50
	maxReservationQty   = 1000
	maxReservationRef   = 100
	MinReservationTTL   = time.Minute
	MaxReservationTTL   = 24 * time.Hour
)

// ReservationItem là một dòng hàng cần giữ — CHƯA phân bổ kho.
type ReservationItem struct {
	VariantID uuid.UUID
	Quantity  int
}

// ReservationLine là phần đã phân bổ: bao nhiêu cái của phiên bản nào, ở kho nào.
type ReservationLine struct {
	VariantID  uuid.UUID
	LocationID uuid.UUID
	Quantity   int
}

type Reservation struct {
	ID        uuid.UUID
	Ref       string
	Status    ReservationStatus
	ExpiresAt *time.Time
	Lines     []ReservationLine
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ValidateReservationRequest kiểm đầu vào trước khi đụng tới tồn.
func ValidateReservationRequest(ref string, items []ReservationItem, ttl *time.Duration) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || utf8.RuneCountInString(ref) > maxReservationRef {
		return "", ErrInvalidReservation
	}
	if len(items) == 0 || len(items) > maxReservationItems {
		return "", ErrInvalidReservation
	}
	seen := make(map[uuid.UUID]bool, len(items))
	for _, it := range items {
		if it.Quantity < 1 || it.Quantity > maxReservationQty || seen[it.VariantID] {
			return "", ErrInvalidReservation
		}
		seen[it.VariantID] = true
	}
	if ttl != nil && (*ttl < MinReservationTTL || *ttl > MaxReservationTTL) {
		return "", ErrInvalidReservation
	}
	return ref, nil
}

// Candidate là tồn ĐÃ KHÓA của một phiên bản ở một kho ứng viên (đang hoạt
// động và bán online), kèm thứ tự ưu tiên của kho.
type Candidate struct {
	Level    *StockLevel
	Priority int
	Code     string
}

// Allocate chia các dòng hàng vào kho (đặc tả P3.2 mục 2.2). Hàm thuần: không
// đổi tồn, chỉ quyết định. Thiếu hàng ở BẤT KỲ dòng nào → không phân bổ gì cả,
// trả lỗi chỉ rõ những dòng thiếu.
func Allocate(items []ReservationItem, candidates map[uuid.UUID][]Candidate) ([]ReservationLine, error) {
	var lines []ReservationLine
	var short []errs.FieldError
	for i, it := range items {
		cs := slices.Clone(candidates[it.VariantID])
		slices.SortFunc(cs, func(a, b Candidate) int {
			return cmp.Or(cmp.Compare(a.Priority, b.Priority), cmp.Compare(a.Code, b.Code))
		})
		// 1. Một kho đủ một mình → lấy cả ở đó: một kiện rẻ hơn gom từ hai kho.
		if j := slices.IndexFunc(cs, func(c Candidate) bool { return c.Level.Available() >= it.Quantity }); j >= 0 {
			lines = append(lines, ReservationLine{VariantID: it.VariantID, LocationID: cs[j].Level.LocationID, Quantity: it.Quantity})
			continue
		}
		// 2. Gom theo ưu tiên cho tới đủ.
		need := it.Quantity
		var part []ReservationLine
		for _, c := range cs {
			take := min(need, c.Level.Available())
			if take <= 0 {
				continue
			}
			part = append(part, ReservationLine{VariantID: it.VariantID, LocationID: c.Level.LocationID, Quantity: take})
			need -= take
			if need == 0 {
				break
			}
		}
		if need > 0 {
			// KHÔNG nói còn bao nhiêu — storefront không lộ số tồn (quyết định P3 số 4).
			short = append(short, errs.FieldError{Field: "items[" + strconv.Itoa(i) + "].quantity",
				Code: "INSUFFICIENT_STOCK", Message: "Sản phẩm không còn đủ hàng"})
			continue
		}
		lines = append(lines, part...)
	}
	if len(short) > 0 {
		return nil, ErrInsufficientStock.WithFields(short...)
	}
	return lines, nil
}

func NewReservation(ref string, lines []ReservationLine, ttl *time.Duration, now time.Time) *Reservation {
	r := &Reservation{ID: uuid.Must(uuid.NewV7()), Ref: ref, Status: ReservationActive, Lines: lines,
		CreatedAt: now, UpdatedAt: now}
	if ttl != nil {
		exp := now.Add(*ttl)
		r.ExpiresAt = &exp
	}
	return r
}

// SameItems: giữ chỗ này có phải cho đúng các dòng hàng này không — để phân
// biệt "gửi lại cùng ref" với "dùng lại ref cho đơn khác".
func (r *Reservation) SameItems(items []ReservationItem) bool {
	got := map[uuid.UUID]int{}
	for _, l := range r.Lines {
		got[l.VariantID] += l.Quantity
	}
	if len(got) != len(items) {
		return false
	}
	for _, it := range items {
		if got[it.VariantID] != it.Quantity {
			return false
		}
	}
	return true
}

// Finish chuyển active → trạng thái cuối. Chỉ chuyển được MỘT lần.
func (r *Reservation) Finish(to ReservationStatus, now time.Time) error {
	if r.Status != ReservationActive {
		return ErrReservationNotActive
	}
	r.Status = to
	r.UpdatedAt = now
	return nil
}

// Reserve / Release / Commit đổi tồn cho một dòng giữ chỗ và trả dòng sổ cái.
// Kiểm lại ở đây dù Allocate đã tính: tồn sai lệch vì lỗi ở đâu đó thì lỗi ở
// đây, không để CHECK của DB là người phát hiện đầu tiên.

func (s *StockLevel) Reserve(q int, ref string) (*StockMovement, error) {
	if q <= 0 || q > s.Available() {
		return nil, ErrInsufficientStock
	}
	return s.move(MovementReserve, 0, q, ref), nil
}

func (s *StockLevel) Release(q int, ref string) (*StockMovement, error) {
	if q <= 0 || q > s.Reserved {
		return nil, ErrInvalidQuantity
	}
	return s.move(MovementRelease, 0, -q, ref), nil
}

// Commit: hàng rời kho — trừ cả on_hand lẫn reserved.
func (s *StockLevel) Commit(q int, ref string) (*StockMovement, error) {
	if q <= 0 || q > s.Reserved || q > s.OnHand {
		return nil, ErrInvalidQuantity
	}
	return s.move(MovementCommit, -q, -q, ref), nil
}

func (s *StockLevel) move(kind MovementKind, dOnHand, dReserved int, ref string) *StockMovement {
	now := time.Now().UTC()
	s.OnHand += dOnHand
	s.Reserved += dReserved
	s.UpdatedAt = now
	return &StockMovement{ID: uuid.Must(uuid.NewV7()), LocationID: s.LocationID, VariantID: s.VariantID, Kind: kind,
		OnHandDelta: dOnHand, ReservedDelta: dReserved, OnHandAfter: s.OnHand, ReservedAfter: s.Reserved,
		Ref: optString(ref), CreatedAt: now}
}
