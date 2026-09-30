package httpapi

import (
	"net/http"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type addressDTO struct {
	ID            uuid.UUID `json:"id"`
	RecipientName string    `json:"recipient_name"`
	Phone         string    `json:"phone"`
	Province      string    `json:"province"`
	Ward          string    `json:"ward"`
	Street        string    `json:"street"`
	IsDefault     bool      `json:"is_default"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toAddressDTO(a *domain.Address) addressDTO {
	return addressDTO{ID: a.ID, RecipientName: a.RecipientName, Phone: a.Phone, Province: a.Province,
		Ward: a.Ward, Street: a.Street, IsDefault: a.IsDefault, UpdatedAt: a.UpdatedAt}
}

// addressRequest dùng chung cho tạo (thiếu trường = rỗng = lỗi của trường đó)
// và sửa (thiếu trường = giữ nguyên).
type addressRequest struct {
	RecipientName *string `json:"recipient_name"`
	Phone         *string `json:"phone"`
	Province      *string `json:"province"`
	Ward          *string `json:"ward"`
	Street        *string `json:"street"`
}

func (r addressRequest) patch() domain.AddressPatch {
	return domain.AddressPatch{RecipientName: r.RecipientName, Phone: r.Phone, Province: r.Province,
		Ward: r.Ward, Street: r.Street}
}

type updateMeRequest struct {
	FullName string `json:"full_name"`
}

// principal lấy người đang đăng nhập; route đã qua RequireAuth nên thiếu là
// lỗi lập trình — vẫn trả 401 chứ không panic.
func principal(r *http.Request) (Principal, error) {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return Principal{}, errUnauthenticatedToken
	}
	return p, nil
}

// addressID: id không phải uuid thì cũng "không tìm thấy" như id lạ.
func addressID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, domain.ErrUnknownAddress
	}
	return id, nil
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	req, err := httpx.Decode[updateMeRequest](w, r)
	if err != nil {
		return err
	}
	u, err := h.uc.Auth.UpdateProfile(r.Context(), p.UserID, req.FullName)
	if err != nil {
		return otpAuthError(w, err)
	}
	w.Header().Set("Cache-Control", "no-store")
	return httpx.JSON(w, http.StatusOK, toUserDTO(u))
}

func (h *Handler) ListAddresses(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	list, err := h.uc.AddressBook.List(r.Context(), p.UserID)
	if err != nil {
		return err
	}
	out := make([]addressDTO, len(list))
	for i, a := range list {
		out[i] = toAddressDTO(a)
	}
	w.Header().Set("Cache-Control", "no-store")
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) CreateAddress(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	req, err := httpx.Decode[addressRequest](w, r)
	if err != nil {
		return err
	}
	a, err := h.uc.AddressBook.Create(r.Context(), p.UserID, req.patch())
	if err != nil {
		return otpAuthError(w, err)
	}
	return httpx.JSON(w, http.StatusCreated, toAddressDTO(a))
}

func (h *Handler) UpdateAddress(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := addressID(r)
	if err != nil {
		return err
	}
	req, err := httpx.Decode[addressRequest](w, r)
	if err != nil {
		return err
	}
	a, err := h.uc.AddressBook.Update(r.Context(), p.UserID, id, req.patch())
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toAddressDTO(a))
}

func (h *Handler) DeleteAddress(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := addressID(r)
	if err != nil {
		return err
	}
	if err := h.uc.AddressBook.Delete(r.Context(), p.UserID, id); err != nil {
		return otpAuthError(w, err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) SetDefaultAddress(w http.ResponseWriter, r *http.Request) error {
	p, err := principal(r)
	if err != nil {
		return err
	}
	id, err := addressID(r)
	if err != nil {
		return err
	}
	a, err := h.uc.AddressBook.SetDefault(r.Context(), p.UserID, id)
	if err != nil {
		return otpAuthError(w, err)
	}
	return httpx.JSON(w, http.StatusOK, toAddressDTO(a))
}
