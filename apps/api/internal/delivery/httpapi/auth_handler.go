package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/authtoken"
	"base-ecommerce/api/pkg/errs"
	"base-ecommerce/api/pkg/httpx"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// TokenVerifier kiểm access token. Cài đặt là pkg/authtoken.Issuer.
type TokenVerifier interface {
	Parse(raw string) (authtoken.Claims, error)
}

type authCtxKey struct{}

// Principal là người đang gọi, gắn vào context bởi RequireAuth.
type Principal struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
}

// PrincipalFrom đọc người gọi. ok=false nghĩa là route không đi qua RequireAuth
// — lỗi lập trình, không phải lỗi của client.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(authCtxKey{}).(Principal)
	return p, ok
}

var errUnauthenticatedToken = errs.New(errs.KindUnauthenticated, "UNAUTHENTICATED",
	"Bạn cần đăng nhập để thực hiện thao tác này")

// RequireAuth chỉ nhận `Authorization: Bearer <access token>`.
//
// Không đọc cookie: storefront là BFF (đặc tả P2.1 mục 2.5), trình duyệt không
// bao giờ gọi thẳng API. Nhận cookie ở đây là mở cửa cho CSRF mà không được
// thêm gì.
//
// Mọi thất bại (thiếu, sai định dạng, hết hạn, sai chữ ký) trả CÙNG một lỗi —
// không cho client biết token hỏng theo kiểu nào.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" {
			httpx.WriteError(w, r, errUnauthenticatedToken)
			return
		}
		c, err := h.verifier.Parse(strings.TrimSpace(raw))
		if err != nil {
			httpx.WriteError(w, r, errUnauthenticatedToken)
			return
		}
		ctx := context.WithValue(r.Context(), authCtxKey{}, Principal{UserID: c.UserID, SessionID: c.SessionID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type userDTO struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	FullName      string    `json:"full_name"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

func toUserDTO(u *domain.User) userDTO {
	return userDTO{ID: u.ID, Email: u.Email, FullName: u.FullName,
		EmailVerified: u.EmailVerifiedAt != nil, CreatedAt: u.CreatedAt}
}

type sessionDTO struct {
	AccessToken      string    `json:"access_token"`
	TokenType        string    `json:"token_type"`
	ExpiresIn        int       `json:"expires_in"` // giây, theo RFC 6749
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	User             userDTO   `json:"user"`
}

func toSessionDTO(s *usecase.Session) sessionDTO {
	return sessionDTO{
		AccessToken: s.AccessToken, TokenType: "Bearer",
		ExpiresIn:    int(math.Round(time.Until(s.AccessExpiresAt).Seconds())),
		RefreshToken: s.RefreshToken, RefreshExpiresAt: s.RefreshExpiresAt,
		User: toUserDTO(s.User),
	}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// authError đặt Retry-After khi bị rate limit rồi trả lỗi cho httpx.Wrap ghi.
// WriteError không xóa header đã đặt, nên header đi kèm response 429.
func authError(w http.ResponseWriter, err error) error {
	var rl *usecase.RateLimitError
	if errors.As(err, &rl) {
		secs := int(math.Ceil(rl.RetryAfter.Seconds()))
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
	}
	return err
}

func clientIP(r *http.Request) string {
	// GetClientIP, KHÔNG r.RemoteAddr hay X-Forwarded-For: router hiện chỉ tin
	// socket (ClientIPFromRemoteAddr). Đọc header trực tiếp thì mỗi request tự
	// bịa một IP mới và rate limit vô nghĩa (thiết kế 03 mục 7).
	return middleware.GetClientIP(r.Context())
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[registerRequest](w, r)
	if err != nil {
		return err
	}
	s, err := h.uc.Auth.Register(r.Context(), usecase.RegisterInput{
		Email: req.Email, Password: req.Password, FullName: req.FullName, ClientIP: clientIP(r),
	})
	if err != nil {
		return authError(w, err)
	}
	// Token không bao giờ được cache ở bất cứ đâu giữa đường.
	w.Header().Set("Cache-Control", "no-store")
	return httpx.JSON(w, http.StatusCreated, toSessionDTO(s))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[loginRequest](w, r)
	if err != nil {
		return err
	}
	s, err := h.uc.Auth.Login(r.Context(), usecase.LoginInput{
		Email: req.Email, Password: req.Password, ClientIP: clientIP(r),
	})
	if err != nil {
		return authError(w, err)
	}
	w.Header().Set("Cache-Control", "no-store")
	return httpx.JSON(w, http.StatusOK, toSessionDTO(s))
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[refreshRequest](w, r)
	if err != nil {
		return err
	}
	s, err := h.uc.Auth.Refresh(r.Context(), req.RefreshToken, clientIP(r))
	if err != nil {
		return authError(w, err)
	}
	w.Header().Set("Cache-Control", "no-store")
	return httpx.JSON(w, http.StatusOK, toSessionDTO(s))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[refreshRequest](w, r)
	if err != nil {
		return err
	}
	if err := h.uc.Auth.Logout(r.Context(), req.RefreshToken); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return errUnauthenticatedToken
	}
	u, err := h.uc.Auth.Me(r.Context(), p.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return errUnauthenticatedToken
		}
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	return httpx.JSON(w, http.StatusOK, toUserDTO(u))
}
