package httpapi

import (
	"context"
	"net/http"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type reservationLineDTO struct {
	VariantID  uuid.UUID `json:"variant_id"`
	LocationID uuid.UUID `json:"location_id"`
	Quantity   int       `json:"quantity"`
}

type reservationDTO struct {
	ID        uuid.UUID            `json:"id"`
	Ref       string               `json:"ref"`
	Status    string               `json:"status"`
	ExpiresAt *time.Time           `json:"expires_at"`
	Lines     []reservationLineDTO `json:"lines"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

func toReservationDTO(r *domain.Reservation) reservationDTO {
	lines := make([]reservationLineDTO, len(r.Lines))
	for i, l := range r.Lines {
		lines[i] = reservationLineDTO{VariantID: l.VariantID, LocationID: l.LocationID, Quantity: l.Quantity}
	}
	return reservationDTO{ID: r.ID, Ref: r.Ref, Status: string(r.Status), ExpiresAt: r.ExpiresAt, Lines: lines,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type reserveRequest struct {
	Ref   string `json:"ref"`
	Items []struct {
		VariantID uuid.UUID `json:"variant_id"`
		Quantity  int       `json:"quantity"`
	} `json:"items"`
	TTLSeconds *int `json:"ttl_seconds"`
}

func (h *Handler) CreateReservation(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[reserveRequest](w, r)
	if err != nil {
		return err
	}
	in := usecase.ReserveInput{Ref: req.Ref, Items: make([]domain.ReservationItem, len(req.Items))}
	for i, it := range req.Items {
		in.Items[i] = domain.ReservationItem{VariantID: it.VariantID, Quantity: it.Quantity}
	}
	if req.TTLSeconds != nil {
		ttl := time.Duration(*req.TTLSeconds) * time.Second
		in.TTL = &ttl
	}
	res, replayed, err := h.uc.Reservations.Reserve(r.Context(), in)
	if err != nil {
		return err
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
		w.Header().Set("Idempotent-Replayed", "true")
	}
	return httpx.JSON(w, status, toReservationDTO(res))
}

func reservationID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, domain.ErrUnknownReservation
	}
	return id, nil
}

func (h *Handler) GetReservation(w http.ResponseWriter, r *http.Request) error {
	id, err := reservationID(r)
	if err != nil {
		return err
	}
	res, err := h.uc.Reservations.Get(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toReservationDTO(res))
}

func (h *Handler) ReleaseReservation(w http.ResponseWriter, r *http.Request) error {
	return h.finishReservation(w, r, h.uc.Reservations.Release)
}

func (h *Handler) CommitReservation(w http.ResponseWriter, r *http.Request) error {
	return h.finishReservation(w, r, h.uc.Reservations.Commit)
}

func (h *Handler) finishReservation(w http.ResponseWriter, r *http.Request,
	op func(context.Context, uuid.UUID) (*domain.Reservation, error)) error {
	id, err := reservationID(r)
	if err != nil {
		return err
	}
	res, err := op(r.Context(), id)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toReservationDTO(res))
}
