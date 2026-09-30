package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type locationDTO struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Address     *string   `json:"address"`
	SellsOnline bool      `json:"sells_online"`
	Priority    int       `json:"priority"`
	Active      bool      `json:"active"`
}

func toLocationDTO(l *domain.Location) locationDTO {
	return locationDTO{ID: l.ID, Code: l.Code, Name: l.Name, Kind: string(l.Kind), Address: l.Address,
		SellsOnline: l.SellsOnline, Priority: l.Priority, Active: l.Active}
}

type createLocationRequest struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Address     *string `json:"address"`
	SellsOnline bool    `json:"sells_online"`
	Priority    int     `json:"priority"`
}

// updateLocationRequest: Address là RawMessage để phân biệt ba trạng thái —
// không gửi (giữ nguyên), null (xóa), chuỗi (đặt).
type updateLocationRequest struct {
	Name        *string         `json:"name"`
	Kind        *string         `json:"kind"`
	Address     json.RawMessage `json:"address"`
	SellsOnline *bool           `json:"sells_online"`
	Priority    *int            `json:"priority"`
	Active      *bool           `json:"active"`
}

func (h *Handler) ListLocations(w http.ResponseWriter, r *http.Request) error {
	list, err := h.uc.Inventory.Locations(r.Context())
	if err != nil {
		return err
	}
	out := make([]locationDTO, len(list))
	for i, l := range list {
		out[i] = toLocationDTO(l)
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) CreateLocation(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[createLocationRequest](w, r)
	if err != nil {
		return err
	}
	l, err := h.uc.Inventory.CreateLocation(r.Context(), domain.LocationInput{Code: req.Code, Name: req.Name,
		Kind: domain.LocationKind(req.Kind), Address: req.Address, SellsOnline: req.SellsOnline, Priority: req.Priority})
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusCreated, toLocationDTO(l))
}

func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownLocation
	}
	req, err := httpx.Decode[updateLocationRequest](w, r)
	if err != nil {
		return err
	}
	p := domain.LocationPatch{Name: req.Name, SellsOnline: req.SellsOnline, Priority: req.Priority, Active: req.Active}
	if req.Kind != nil {
		k := domain.LocationKind(*req.Kind)
		p.Kind = &k
	}
	if req.Address != nil {
		var a *string
		if err := json.Unmarshal(req.Address, &a); err != nil {
			return domain.ErrLocationAddressRequired
		}
		p.Address = &a
	}
	l, err := h.uc.Inventory.UpdateLocation(r.Context(), id, p)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toLocationDTO(l))
}

type stockLevelDTO struct {
	OnHand    int `json:"on_hand"`
	Reserved  int `json:"reserved"`
	Available int `json:"available"`
}

type movementDTO struct {
	ID            uuid.UUID  `json:"id"`
	LocationID    uuid.UUID  `json:"location_id"`
	VariantID     uuid.UUID  `json:"variant_id"`
	Kind          string     `json:"kind"`
	OnHandDelta   int        `json:"on_hand_delta"`
	ReservedDelta int        `json:"reserved_delta"`
	OnHandAfter   int        `json:"on_hand_after"`
	ReservedAfter int        `json:"reserved_after"`
	Reason        *string    `json:"reason"`
	Ref           *string    `json:"ref"`
	ActorID       *uuid.UUID `json:"actor_id"`
	CreatedAt     time.Time  `json:"created_at"`
}

func toMovementDTO(m *domain.StockMovement) movementDTO {
	return movementDTO{ID: m.ID, LocationID: m.LocationID, VariantID: m.VariantID, Kind: string(m.Kind),
		OnHandDelta: m.OnHandDelta, ReservedDelta: m.ReservedDelta, OnHandAfter: m.OnHandAfter,
		ReservedAfter: m.ReservedAfter, Reason: m.Reason, Ref: m.Ref, ActorID: m.ActorID, CreatedAt: m.CreatedAt}
}

type recordMovementRequest struct {
	LocationID uuid.UUID `json:"location_id"`
	VariantID  uuid.UUID `json:"variant_id"`
	Kind       string    `json:"kind"`
	Quantity   int       `json:"quantity"`
	Reason     string    `json:"reason"`
	Ref        string    `json:"ref"`
}

// RecordStockMovement nhập hàng / điều chỉnh / kiểm kê.
//
// Idempotency-Key (header, tùy chọn): gửi lại cùng khóa → 200 + kết quả cũ +
// Idempotent-Replayed: true, tồn KHÔNG đổi thêm (đặc tả P3.1 mục 2.5).
func (h *Handler) RecordStockMovement(w http.ResponseWriter, r *http.Request) error {
	who, err := principal(r)
	if err != nil {
		return err
	}
	req, err := httpx.Decode[recordMovementRequest](w, r)
	if err != nil {
		return err
	}
	res, err := h.uc.Inventory.RecordChange(r.Context(), usecase.StockChangeInput{
		LocationID: req.LocationID, VariantID: req.VariantID,
		Change: domain.ManualChange{Kind: domain.MovementKind(req.Kind), Quantity: req.Quantity,
			Reason: req.Reason, Ref: req.Ref},
		Actor: &who.UserID, IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		return err
	}
	m := res.Movement
	status := http.StatusCreated
	if res.Replayed {
		status = http.StatusOK
		w.Header().Set("Idempotent-Replayed", "true")
	}
	return httpx.JSON(w, status, map[string]any{
		"movement": toMovementDTO(m),
		"level": stockLevelDTO{OnHand: m.OnHandAfter, Reserved: m.ReservedAfter,
			Available: m.OnHandAfter - m.ReservedAfter},
	})
}

type locationStockDTO struct {
	Location locationDTO `json:"location"`
	stockLevelDTO
}

func (h *Handler) VariantStock(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownVariant
	}
	v, err := h.uc.Inventory.VariantStock(r.Context(), id)
	if err != nil {
		return err
	}
	levels := make([]locationStockDTO, len(v.Levels))
	for i, l := range v.Levels {
		levels[i] = locationStockDTO{Location: toLocationDTO(l.Location),
			stockLevelDTO: stockLevelDTO{OnHand: l.OnHand, Reserved: l.Reserved, Available: l.OnHand - l.Reserved}}
	}
	// Tồn không bao giờ cache (thiết kế 03) — kể cả ở trình duyệt/CDN.
	w.Header().Set("Cache-Control", "no-store")
	return httpx.JSON(w, http.StatusOK, map[string]any{
		"variant_id": id,
		"levels":     levels,
		"totals": map[string]int{"on_hand": v.OnHand, "reserved": v.Reserved, "available": v.Available(),
			"online_available": v.OnlineAvailable},
	})
}

func (h *Handler) ListStockMovements(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	parse := func(name string) (*uuid.UUID, error) {
		s := q.Get(name)
		if s == "" {
			return nil, nil
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, domain.ErrInvalidMovementFilter
		}
		return &id, nil
	}
	var f usecase.MovementFilter
	var err error
	if f.VariantID, err = parse("variant_id"); err != nil {
		return err
	}
	if f.LocationID, err = parse("location_id"); err != nil {
		return err
	}
	if f.Before, err = parse("cursor"); err != nil {
		return err
	}
	if s := q.Get("limit"); s != "" {
		if f.Limit, err = strconv.Atoi(s); err != nil {
			return domain.ErrInvalidMovementFilter
		}
	}
	list, next, err := h.uc.Inventory.Movements(r.Context(), f)
	if err != nil {
		return err
	}
	out := make([]movementDTO, len(list))
	for i, m := range list {
		out[i] = toMovementDTO(m)
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out, "meta": map[string]any{"next_cursor": next}})
}
