package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"base-ecommerce/api/internal/domain"
	"base-ecommerce/api/internal/usecase"
	"base-ecommerce/api/pkg/httpx"
)

type verifyEmailRequest struct {
	Code string `json:"code"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

// otpAuthError: tài khoản mất / bị khóa khi đang cầm token hợp lệ thì đối xử
// như chưa đăng nhập — cùng cách với GET /me.
func otpAuthError(w http.ResponseWriter, err error) error {
	if errors.Is(err, domain.ErrInvalidCredentials) {
		return errUnauthenticatedToken
	}
	return authError(w, err)
}

func (h *Handler) RequestEmailVerification(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return errUnauthenticatedToken
	}
	if err := h.uc.Verification.RequestEmailVerification(r.Context(), p.UserID); err != nil {
		return otpAuthError(w, err)
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) error {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		return errUnauthenticatedToken
	}
	req, err := httpx.Decode[verifyEmailRequest](w, r)
	if err != nil {
		return err
	}
	u, err := h.uc.Verification.VerifyEmail(r.Context(), p.UserID, req.Code)
	if err != nil {
		return otpAuthError(w, err)
	}
	w.Header().Set("Cache-Control", "no-store")
	return httpx.JSON(w, http.StatusOK, toUserDTO(u))
}

// forgotJobTimeout là trần cho việc phát mã chạy nền — một transaction nhỏ.
const forgotJobTimeout = 15 * time.Second

// ForgotPassword trả 202 cho MỌI email đúng cú pháp, TRƯỚC khi biết email có
// tài khoản hay không — việc tra và phát mã chạy nền (xem usecase).
//
// Cái giá: tiến trình tắt đúng lúc job đang chạy thì mã không được phát, và
// người dùng không biết. Họ bấm "gửi lại" — rẻ hơn nhiều so với để thời gian
// phản hồi khai ra email nào đã đăng ký.
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[forgotPasswordRequest](w, r)
	if err != nil {
		return err
	}
	job, err := h.uc.Verification.ForgotPassword(r.Context(), req.Email, clientIP(r))
	if err != nil {
		return authError(w, err)
	}
	// WithoutCancel: request xong là context bị hủy, job phải sống tiếp. Giữ
	// lại các giá trị trong context (request_id) để outbox ghi được trace_id.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), forgotJobTimeout)
	go func() {
		defer cancel()
		if err := job(ctx); err != nil {
			slog.ErrorContext(ctx, "phát mã đặt lại mật khẩu thất bại", "err", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
	return nil
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) error {
	req, err := httpx.Decode[resetPasswordRequest](w, r)
	if err != nil {
		return err
	}
	if err := h.uc.Verification.ResetPassword(r.Context(), usecase.ResetPasswordInput{
		Email: req.Email, Code: req.Code, NewPassword: req.NewPassword, ClientIP: clientIP(r),
	}); err != nil {
		return authError(w, err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
