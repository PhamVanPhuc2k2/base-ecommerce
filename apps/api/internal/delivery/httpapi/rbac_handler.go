package httpapi

import (
	"net/http"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RequirePermission chặn request thiếu quyền p. PHẢI đứng SAU RequireAuth.
//
// Tra quyền MỖI request (có cache) — access token không mang quyền, nên gỡ vai
// trò có hiệu lực ngay ở request kế tiếp chứ không chờ token hết hạn.
func (h *Handler) RequirePermission(p domain.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			who, ok := PrincipalFrom(r.Context())
			if !ok {
				// Route khai quên RequireAuth — lỗi lập trình. Từ chối, đừng cho qua.
				httpx.WriteError(w, r, errUnauthenticatedToken)
				return
			}
			if err := h.uc.Authorizer.Require(r.Context(), who.UserID, p); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type permissionDTO struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type roleDTO struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toRoleDTO(r *domain.Role) roleDTO {
	perms := make([]string, len(r.Permissions))
	for i, p := range r.Permissions {
		perms[i] = string(p)
	}
	return roleDTO{ID: r.ID, Code: r.Code, Name: r.Name, IsSystem: r.IsSystem, Permissions: perms, UpdatedAt: r.UpdatedAt}
}

type createRoleRequest struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type updateRoleRequest struct {
	Name        *string  `json:"name"`
	Permissions []string `json:"permissions"`
}

type setUserRolesRequest struct {
	RoleIDs []uuid.UUID `json:"role_ids"`
}

func toPermissions(in []string) []domain.Permission {
	if in == nil {
		return nil
	}
	out := make([]domain.Permission, len(in))
	for i, s := range in {
		out[i] = domain.Permission(s)
	}
	return out
}

func (h *Handler) ListPermissions(w http.ResponseWriter, _ *http.Request) error {
	out := make([]permissionDTO, len(domain.AllPermissions))
	for i, p := range domain.AllPermissions {
		out[i] = permissionDTO{Code: string(p.Code), Description: p.Description}
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) error {
	roles, err := h.uc.RoleAdmin.List(r.Context())
	if err != nil {
		return err
	}
	out := make([]roleDTO, len(roles))
	for i, role := range roles {
		out[i] = toRoleDTO(role)
	}
	return httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[createRoleRequest](w, r)
	if err != nil {
		return err
	}
	perms := toPermissions(req.Permissions)
	if perms == nil {
		perms = []domain.Permission{}
	}
	role, err := h.uc.RoleAdmin.Create(r.Context(), req.Code, req.Name, perms)
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusCreated, toRoleDTO(role))
}

func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownRole
	}
	req, err := httpx.Decode[updateRoleRequest](w, r)
	if err != nil {
		return err
	}
	role, err := h.uc.RoleAdmin.Update(r.Context(), id, req.Name, toPermissions(req.Permissions))
	if err != nil {
		return err
	}
	return httpx.JSON(w, http.StatusOK, toRoleDTO(role))
}

func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownRole
	}
	if err := h.uc.RoleAdmin.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) SetUserRoles(w http.ResponseWriter, r *http.Request) error {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return domain.ErrUnknownUser
	}
	who, _ := PrincipalFrom(r.Context())
	req, err := httpx.Decode[setUserRolesRequest](w, r)
	if err != nil {
		return err
	}
	if req.RoleIDs == nil {
		req.RoleIDs = []uuid.UUID{}
	}
	if err := h.uc.RoleAdmin.SetUserRoles(r.Context(), who.UserID, userID, req.RoleIDs); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
