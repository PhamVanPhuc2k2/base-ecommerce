package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ---- Kho (đặc tả P3.1 mục 2.1) ----

type LocationKind string

const (
	LocationWarehouse LocationKind = "warehouse"
	LocationShowroom  LocationKind = "showroom"
)

func (k LocationKind) Valid() bool { return k == LocationWarehouse || k == LocationShowroom }

var locationCodeRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const (
	maxLocationCodeLen    = 32
	maxLocationAddressLen = 255
)

// Location là một kho hoặc showroom. Không bao giờ bị xóa — chỉ ngừng hoạt
// động — vì sổ cái trỏ tới nó mãi mãi.
type Location struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Kind        LocationKind
	Address     *string
	SellsOnline bool
	// Priority: số nhỏ được phân bổ trước (P3.2); trùng thì theo Code.
	Priority  int
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LocationInput struct {
	Code        string
	Name        string
	Kind        LocationKind
	Address     *string
	SellsOnline bool
	Priority    int
}

func NewLocation(in LocationInput) (*Location, error) {
	code := strings.TrimSpace(in.Code)
	if len(code) > maxLocationCodeLen || !locationCodeRe.MatchString(code) {
		return nil, ErrInvalidLocationCode
	}
	now := time.Now().UTC()
	l := &Location{ID: uuid.Must(uuid.NewV7()), Code: code, Kind: in.Kind, SellsOnline: in.SellsOnline,
		Priority: in.Priority, Active: true, CreatedAt: now, UpdatedAt: now}
	name, addr := in.Name, in.Address
	if err := l.apply(LocationPatch{Name: &name, Address: &addr}); err != nil {
		return nil, err
	}
	return l, nil
}

// LocationPatch: nil = giữ nguyên. Address là con trỏ tới con trỏ để phân
// biệt "không gửi" với "xóa địa chỉ" (gửi null). Code không có trong patch —
// không đổi được.
type LocationPatch struct {
	Name        *string
	Kind        *LocationKind
	Address     **string
	SellsOnline *bool
	Priority    *int
	Active      *bool
}

func (l *Location) Update(p LocationPatch) error {
	if err := l.apply(p); err != nil {
		return err
	}
	l.UpdatedAt = time.Now().UTC()
	return nil
}

func (l *Location) apply(p LocationPatch) error {
	if p.Name != nil {
		name, err := normalizeTaxonomyName(*p.Name, ErrLocationNameInvalid)
		if err != nil {
			return err
		}
		l.Name = name
	}
	if p.Kind != nil {
		l.Kind = *p.Kind
	}
	if !l.Kind.Valid() {
		return ErrInvalidLocationKind
	}
	if p.Address != nil {
		l.Address = nil
		if a := *p.Address; a != nil {
			s := strings.Join(strings.Fields(*a), " ")
			if utf8.RuneCountInString(s) > maxLocationAddressLen {
				return ErrLocationAddressRequired
			}
			if s != "" {
				l.Address = &s
			}
		}
	}
	// Showroom hiện cho khách "còn hàng tại showroom nào" (P3.4) — không có
	// địa chỉ thì khách biết đi đâu.
	if l.Kind == LocationShowroom && l.Address == nil {
		return ErrLocationAddressRequired
	}
	if p.SellsOnline != nil {
		l.SellsOnline = *p.SellsOnline
	}
	if p.Priority != nil {
		l.Priority = *p.Priority
	}
	if p.Active != nil {
		l.Active = *p.Active
	}
	return nil
}

// ---- Tồn kho (đặc tả P3.1 mục 2.2, 2.3) ----

type MovementKind string

const (
	MovementReceipt    MovementKind = "receipt"
	MovementAdjustment MovementKind = "adjustment"
	MovementCount      MovementKind = "count"
	// reserve / release / commit thuộc P3.2 (giữ chỗ).
)

// ManualMovementKinds là các thao tác người quản trị được làm tay.
func (k MovementKind) Manual() bool {
	return k == MovementReceipt || k == MovementAdjustment || k == MovementCount
}

const (
	// maxStockQuantity chặn số vô lý (gõ nhầm thêm ba số 0) và tràn INT4.
	maxStockQuantity = 1_000_000
	maxStockReason   = 500
	maxStockRef      = 100
)

// StockLevel là tồn của một phiên bản tại một kho.
//
// Available KHÔNG phải trường: on_hand − reserved tính ra mỗi lần. Lưu cả ba
// con số thì sớm muộn chúng lệch nhau.
type StockLevel struct {
	LocationID uuid.UUID
	VariantID  uuid.UUID
	OnHand     int
	Reserved   int
	UpdatedAt  time.Time
}

func (s *StockLevel) Available() int { return s.OnHand - s.Reserved }

// StockMovement là một dòng sổ cái — ghi lại MỌI thay đổi của StockLevel.
type StockMovement struct {
	ID             uuid.UUID
	LocationID     uuid.UUID
	VariantID      uuid.UUID
	Kind           MovementKind
	OnHandDelta    int
	ReservedDelta  int
	OnHandAfter    int
	ReservedAfter  int
	Reason         *string
	Ref            *string
	ActorID        *uuid.UUID
	IdempotencyKey *string
	CreatedAt      time.Time
}

// ManualChange là yêu cầu nhập / điều chỉnh / kiểm kê của người quản trị.
// Quantity: receipt = số nhập (> 0); adjustment = delta (≠ 0, có dấu);
// count = số đếm được (tuyệt đối, ≥ 0).
type ManualChange struct {
	Kind     MovementKind
	Quantity int
	Reason   string
	Ref      string
}

// Apply đổi tồn theo yêu cầu và trả dòng sổ cái tương ứng. Lỗi thì StockLevel
// KHÔNG đổi.
//
// Không cho on_hand xuống dưới reserved: hàng đã hứa cho đơn không được "biến
// mất" — muốn giảm thì hủy giữ chỗ trước.
func (s *StockLevel) Apply(c ManualChange, actor *uuid.UUID) (*StockMovement, error) {
	if !c.Kind.Manual() {
		return nil, ErrInvalidMovementKind
	}
	reason := strings.TrimSpace(c.Reason)
	ref := strings.TrimSpace(c.Ref)
	if utf8.RuneCountInString(reason) > maxStockReason || utf8.RuneCountInString(ref) > maxStockRef {
		return nil, ErrStockReasonRequired
	}
	if c.Quantity > maxStockQuantity || c.Quantity < -maxStockQuantity {
		return nil, ErrInvalidQuantity
	}

	var delta int
	switch c.Kind {
	case MovementReceipt:
		if c.Quantity <= 0 {
			return nil, ErrInvalidQuantity
		}
		delta = c.Quantity
	case MovementAdjustment:
		if c.Quantity == 0 {
			return nil, ErrInvalidQuantity
		}
		delta = c.Quantity
	case MovementCount:
		if c.Quantity < 0 {
			return nil, ErrInvalidQuantity
		}
		delta = c.Quantity - s.OnHand
	}
	// Điều chỉnh và kiểm kê đổi số tồn mà không có chứng từ nhập — phải ghi
	// lý do, không thì sổ cái chỉ còn những con số không ai giải thích được.
	if c.Kind != MovementReceipt && reason == "" {
		return nil, ErrStockReasonRequired
	}
	next := s.OnHand + delta
	if next < s.Reserved || next < 0 {
		return nil, ErrInsufficientStock
	}

	now := time.Now().UTC()
	s.OnHand = next
	s.UpdatedAt = now
	return &StockMovement{
		ID: uuid.Must(uuid.NewV7()), LocationID: s.LocationID, VariantID: s.VariantID, Kind: c.Kind,
		OnHandDelta: delta, OnHandAfter: s.OnHand, ReservedAfter: s.Reserved,
		Reason: optString(reason), Ref: optString(ref), ActorID: actor, CreatedAt: now,
	}, nil
}

// SameRequest: dòng sổ cái này có phải kết quả của đúng yêu cầu c không —
// để phân biệt "gửi lại cùng Idempotency-Key" với "dùng lại khóa cho việc khác".
func (m *StockMovement) SameRequest(locationID, variantID uuid.UUID, c ManualChange) bool {
	if m.LocationID != locationID || m.VariantID != variantID || m.Kind != c.Kind {
		return false
	}
	switch c.Kind {
	case MovementCount:
		return m.OnHandAfter == c.Quantity
	default:
		return m.OnHandDelta == c.Quantity
	}
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
